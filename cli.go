package main

// The command line: each command with its flags, its help and its run function.

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// setting is one value of Config. A command takes it as a flag, reads it from the
// env, or both. A setting without arg is a switch, such as --bg.
type setting struct {
	flag, arg, env, def, help string
}

var settings = []setting{
	{"host", "HOST", "MOKI_HOST", "127.0.0.1", "host to listen on"},
	{"port", "PORT", "MOKI_PORT", "9999", "port to listen on"},
	{"dir", "DIR", "MOKI_HOME", "~/.moki", "data folder"},
	{"username", "USERNAME", "MOKI_USERNAME", "", "basic auth username"},
	{"password", "PASSWORD", "MOKI_PASSWORD", "", "basic auth password"},
	{"url", "URL", "MOKI_URL", "", "URL that agents use to reach moki"},
	{"bg", "", "", "", "run in the background"},
}

type command struct {
	name  string
	lines [][2]string // usage, and what it does
	about string
	flags []string // settings it takes as flags, by flag name
	env   []string // settings it reads only from the env
	run   func(cfg Config, args []string) error
}

var commands []*command

func init() {
	commands = []*command{
		{
			name:  "start",
			lines: [][2]string{{"moki [flags]", "start moki"}},
			about: "Starts the server and prints where to open it. Each flag but --bg also works as an env var.",
			flags: []string{"host", "port", "dir", "username", "password", "url", "bg"},
			run:   start,
		},
		{
			name: "token",
			lines: [][2]string{
				{"moki token register", "make a token and print the guide with it"},
				{"moki token list", "list the tokens"},
				{"moki token revoke <name>", "delete a token"},
			},
			about: "register prints the text to paste into an agent: what moki is, and how to connect with the token.\n" +
				"Each token gets a name, such as brave-otter. The name starts the token and shows in the log.\n" +
				"You can also make tokens in the browser: Connect agent.",
			env: []string{"url", "dir"},
			run: tokenCommand,
		},
		{
			name:  "llm-guide",
			lines: [][2]string{{"moki llm-guide", "print the guide for agents"}},
			env:   []string{"url"},
			run:   func(cfg Config, _ []string) error { fmt.Print(guideText(cfg.URL, nil)); return nil },
		},
		{
			name:  "version",
			lines: [][2]string{{"moki version", "print the version"}},
			run:   func(Config, []string) error { fmt.Println("moki", version); return nil },
		},
		{
			name:  "help",
			lines: [][2]string{{"moki help [command]", "show the help of a command"}},
			run:   helpCommand,
		},
	}
}

func main() {
	c, args := commands[0], os.Args[1:]
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		if c = findCommand(args[0]); c == nil {
			fail(unknownCommand(args[0]))
		}
		args = args[1:]
	}
	cfg, pos, err := c.parse(args)
	if errors.Is(err, flag.ErrHelp) {
		c.printHelp()
		return
	}
	if err == nil {
		err = c.run(cfg, pos)
	}
	if err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, red(bold("moki:"))+" "+err.Error())
	os.Exit(1)
}

func findCommand(name string) *command {
	for _, c := range commands {
		if c.name == name {
			return c
		}
	}
	return nil
}

func unknownCommand(name string) error {
	for _, c := range commands {
		if distance(name, c.name) <= 2 {
			return fmt.Errorf("there is no command %q. Did you mean %q?", name, "moki "+c.name)
		}
	}
	return fmt.Errorf(`there is no command %q. Run "moki help" to see the commands`, name)
}

// parse reads the env, then the flags, which may stand anywhere among the arguments.
func (c *command) parse(args []string) (Config, []string, error) {
	values := map[string]*string{}
	fs := flag.NewFlagSet(c.name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	for _, s := range settings {
		v := env(s.env, s.def)
		values[s.flag] = &v
		if c.takes(s.flag) {
			fs.Var(flagValue{values[s.flag], s.arg == ""}, s.flag, s.help)
		}
	}
	var pos []string
	for {
		if err := fs.Parse(args); err != nil {
			return Config{}, nil, c.flagError(err)
		}
		if fs.NArg() == 0 {
			break
		}
		pos = append(pos, fs.Arg(0))
		args = fs.Args()[1:]
	}
	cfg, err := newConfig(values)
	return cfg, pos, err
}

// flagValue keeps a flag in its string, so that every setting reads the same way.
// A switch is "true" when it is set.
type flagValue struct {
	p      *string
	isBool bool
}

func (f flagValue) String() string {
	if f.p == nil {
		return ""
	}
	return *f.p
}

func (f flagValue) Set(v string) error { *f.p = v; return nil }
func (f flagValue) IsBoolFlag() bool   { return f.isBool }

func start(cfg Config, _ []string) error {
	if cfg.Background {
		return startBackground(cfg)
	}
	return serve(cfg)
}

func (c *command) takes(flag string) bool {
	for _, f := range c.flags {
		if f == flag {
			return true
		}
	}
	return false
}

func (c *command) flagError(err error) error {
	if errors.Is(err, flag.ErrHelp) {
		return err
	}
	msg, unknown := strings.CutPrefix(err.Error(), "flag provided but not defined: -")
	if unknown {
		msg = "unknown flag --" + msg
	}
	return fmt.Errorf(`%s. Run "moki help %s" to see what it takes`, msg, c.name)
}

func newConfig(v map[string]*string) (Config, error) {
	port, err := strconv.Atoi(*v["port"])
	if err != nil || port < 1 || port > 65535 {
		return Config{}, fmt.Errorf("port %q is not a port number", *v["port"])
	}
	cfg := Config{Host: *v["host"], Port: port, Dir: expandHome(*v["dir"]), Username: *v["username"], Password: *v["password"], URL: *v["url"], Background: *v["bg"] == "true"}
	if cfg.URL == "" {
		cfg.URL = "http://" + joinHostPort(publicHost(cfg.Host), port)
	}
	cfg.URL = strings.TrimRight(cfg.URL, "/")
	return cfg, nil
}

func expandHome(p string) string {
	if rest, ok := strings.CutPrefix(p, "~"); ok && (rest == "" || rest[0] == '/') {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, rest)
	}
	return p
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// distance is the edit distance of two short words.
func distance(a, b string) int {
	prev := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		cur := []int{i}
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			cur = append(cur, min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost))
		}
		prev = cur
	}
	return prev[len(b)]
}
