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

// Tokens is the tokens file: one "<RFC 3339 time> <token> <name> [<first use>]" per line.
// The CLI and the connect dialog write it. moki reads it on each MCP request,
// so a revoke works at once.
type Tokens struct {
	path    string
	mu      sync.Mutex
	onFirst func() // runs when an agent uses a token for the first time
}

// Token is one agent's key. Its value starts with its name: mk_brave-otter_K7Q2...
type Token struct {
	Time  time.Time
	Value string
	Name  string
	Used  time.Time // the first MCP request with it, zero before that
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
	return t.write(keep)
}

// MarkUsed records the first use of a token. Later uses change nothing.
func (t *Tokens) MarkUsed(tok Token) {
	if !tok.Used.IsZero() {
		return
	}
	t.mu.Lock()
	var lines []string
	first := false // another request may have marked it already
	for _, x := range t.List() {
		if x.Value == tok.Value && x.Used.IsZero() {
			x.Used, first = time.Now().UTC(), true
		}
		lines = append(lines, x.line())
	}
	if first && t.write(lines) != nil {
		first = false
	}
	t.mu.Unlock()
	if first && t.onFirst != nil {
		t.onFirst()
	}
}

// LastAgent names the token that an agent used first most recently, or "" for none.
func (t *Tokens) LastAgent() string {
	last := Token{}
	for _, x := range t.List() {
		if x.Used.After(last.Used) {
			last = x
		}
	}
	return last.Name
}

// write replaces the file through a rename, so a request that reads it at the
// same time sees the old tokens or the new ones, never half of them.
func (t *Tokens) write(lines []string) error {
	tmp := t.path + ".tmp"
	if err := os.WriteFile(tmp, []byte(strings.Join(lines, "")), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, t.path)
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
		if len(f) > 3 {
			tok.Used, _ = time.Parse(time.RFC3339, f[3])
		}
		list = append(list, tok)
	}
	return list
}

func (t Token) line() string {
	used := ""
	if !t.Used.IsZero() {
		used = " " + t.Used.Format(time.RFC3339)
	}
	return fmt.Sprintf("%s %s %s%s\n", t.Time.Format(time.RFC3339), t.Value, t.Name, used)
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
		value, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		tok, known := t.Find(strings.TrimSpace(value))
		if !ok || !known {
			activity.Warn("an MCP request from %s has a missing or unknown token", r.RemoteAddr)
			w.Header().Set("WWW-Authenticate", `Bearer realm="moki"`)
			http.Error(w, `missing or unknown token. Make one in moki with "Connect agent", or run "moki token register"`, http.StatusUnauthorized)
			return
		}
		t.MarkUsed(tok)
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
