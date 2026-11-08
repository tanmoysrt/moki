package main

import (
	"crypto/rand"
	"crypto/subtle"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Tokens is the tokens file: one "<RFC 3339 time> <token> <name>" per line.
// The CLI and the connect dialog write it. moki reads it on each MCP request,
// so a revoke works at once.
type Tokens struct {
	path string
	mu   sync.Mutex
}

// Token is one agent's key. Its value starts with its name: mk_brave-otter_K7Q2...
type Token struct {
	Time  time.Time
	Value string
	Name  string
}

// Make adds a token with a new name and returns it.
func (t *Tokens) Make() (Token, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if err := os.MkdirAll(filepath.Dir(t.path), 0o755); err != nil {
		return Token{}, err
	}
	taken := map[string]bool{}
	for _, x := range t.List() {
		taken[x.Name] = true
	}
	name := newName(taken)
	tok := Token{Time: time.Now().UTC(), Value: "mk_" + name + "_" + rand.Text()[:20], Name: name}
	f, err := os.OpenFile(t.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return Token{}, err
	}
	defer f.Close()
	_, err = fmt.Fprint(f, tok.line())
	return tok, err
}

// Revoke deletes the tokens with this name.
func (t *Tokens) Revoke(name string) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	var keep []string
	found := false
	for _, tok := range t.List() {
		if tok.Name == name {
			found = true
			continue
		}
		keep = append(keep, tok.line())
	}
	if !found {
		return fmt.Errorf("no token has the name %q", name)
	}
	return os.WriteFile(t.path, []byte(strings.Join(keep, "")), 0o600)
}

func (t *Tokens) List() []Token {
	b, _ := os.ReadFile(t.path)
	var list []Token
	for _, line := range strings.Split(string(b), "\n") {
		f := strings.Fields(line)
		if len(f) < 2 {
			continue
		}
		tm, _ := time.Parse(time.RFC3339, f[0])
		tok := Token{Time: tm, Value: f[1]}
		if len(f) > 2 {
			tok.Name = f[2]
		}
		list = append(list, tok)
	}
	return list
}

func (t Token) line() string {
	return fmt.Sprintf("%s %s %s\n", t.Time.Format(time.RFC3339), t.Value, t.Name)
}

// Find returns the token with this value. It compares every token in constant time.
func (t *Tokens) Find(value string) (Token, bool) {
	var found Token
	ok := false
	for _, x := range t.List() {
		if subtle.ConstantTimeCompare([]byte(x.Value), []byte(value)) == 1 {
			found, ok = x, true
		}
	}
	return found, ok
}

// Name names the agent behind an Authorization header, or "agent" when it is unknown.
func (t *Tokens) Name(header string) string {
	value, _ := strings.CutPrefix(header, "Bearer ")
	if tok, ok := t.Find(strings.TrimSpace(value)); ok {
		return tok.Name
	}
	return "agent"
}

// Require lets a request through only with a valid bearer token.
func (t *Tokens) Require(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tok, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		if _, known := t.Find(strings.TrimSpace(tok)); !ok || !known {
			activity.Warn("an MCP request from %s has a missing or unknown token", r.RemoteAddr)
			w.Header().Set("WWW-Authenticate", `Bearer realm="moki"`)
			http.Error(w, `missing or unknown token. Make one in moki with "Connect agent", or run "moki token register"`, http.StatusUnauthorized)
			return
		}
		h.ServeHTTP(w, r)
	})
}

// ---------- moki token ----------

// tokenCommand runs "moki token register", "moki token list" and "moki token revoke <name>".
func tokenCommand(cfg Config, args []string) error {
	t := &Tokens{path: filepath.Join(cfg.Dir, "tokens")}
	sub := ""
	if len(args) > 0 {
		sub = args[0]
	}
	switch sub {
	case "":
		findCommand("token").printHelp()
		return nil
	case "register":
		return t.printRegister(cfg.URL)
	case "list":
		t.printList()
		return nil
	case "revoke":
		if len(args) < 2 {
			return fmt.Errorf("give the name of the token, for example: moki token revoke brave-otter")
		}
		return t.printRevoke(args[1])
	}
	return fmt.Errorf(`there is no command "moki token %s". Run "moki help token"`, sub)
}

// printRegister prints the guide with a new token in it. The notes go to stderr, so
// "moki token register | pbcopy" copies only the guide.
func (t *Tokens) printRegister(url string) error {
	tok, err := t.Make()
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "\nMade the token %s. Paste the text below into your agent.\n\n", magenta(tok.Name))
	fmt.Print(guideText(url, &tok))
	fmt.Fprintln(os.Stderr, "\n"+dim("Revoke it with: moki token revoke "+tok.Name))
	return nil
}

func (t *Tokens) printList() {
	list := t.List()
	if len(list) == 0 {
		fmt.Println("No tokens yet. Make one with: moki token register")
		return
	}
	width := 0
	for _, tok := range list {
		width = max(width, len(tok.Name))
	}
	for _, tok := range list {
		fmt.Println(magenta(pad(tok.Name, width+2)) + dim("made "+ago(tok.Time)))
	}
}

func (t *Tokens) printRevoke(name string) error {
	if err := t.Revoke(name); err != nil {
		return fmt.Errorf(`%v. See the names with: moki token list`, err)
	}
	fmt.Println("Revoked " + magenta(name) + ". Agents with it can no longer reach moki.")
	return nil
}
