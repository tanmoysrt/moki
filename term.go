package main

// Terminal output: the banner when moki starts, and one activity line for each
// tool call, change on disk or problem while it runs.

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

var logo = []string{
	"┌┬┐┌─┐┬┌─┬",
	"││││ │├┴┐│",
	"┴ ┴└─┘┴ ┴┴",
}

// printBanner shows where moki runs and what it has.
func printBanner(cfg Config, s *Server) {
	printLogo()
	row("Open", cyan(cfg.URL))
	row("Data", tildePath(cfg.Dir)+dim(" · "+count(len(s.store.Projects()), "project")))
	tokens := s.tokens.List()
	row("Agents", agentsLine(tokens))
	row("Chrome", chromeLine())
	if cfg.Username != "" {
		row("Auth", "basic"+dim(" · "+cfg.Username))
	} else if !isLoopback(cfg.Host) {
		row("Auth", yellow("none")+dim(" · anyone on the network can look. Set MOKI_USERNAME and MOKI_PASSWORD"))
	}
	fmt.Println()
	if len(tokens) == 0 {
		fmt.Println("  Connect an agent: click " + bold("Connect agent") + " in moki, or run " + bold("moki token register"))
		fmt.Println("  and paste what it prints into the agent.")
		fmt.Println()
	}
}

// printRunning is the short banner for a moki that already runs on the port.
func printRunning(cfg Config) {
	printLogo()
	row("Open", cyan(cfg.URL))
	fmt.Println()
	fmt.Println("  " + dim("moki already runs on this port."))
	fmt.Println()
}

func printLogo() {
	fmt.Println()
	for i, line := range logo {
		tail := ""
		if i == 1 {
			tail = "   " + dim(version)
		}
		fmt.Println("  " + bold(line) + tail)
	}
	fmt.Println()
}

func row(name, value string) {
	fmt.Println("  " + dim(pad(name, 8)) + value)
}

func agentsLine(tokens []Token) string {
	if len(tokens) == 0 {
		return yellow("none yet")
	}
	var names []string
	seen := map[string]bool{}
	for _, t := range tokens {
		if !seen[t.Name] {
			seen[t.Name] = true
			names = append(names, magenta(t.Name))
		}
	}
	return strings.Join(names, dim(", "))
}

func chromeLine() string {
	if path := findChrome(); path != "" {
		return green("found") + dim(" · "+tildePath(path))
	}
	return yellow("not found") + dim(" · no screenshots. Install Chrome or set CHROME to its path")
}

// findChrome looks where chromedp looks, so the banner says what screenshots will use.
func findChrome() string {
	names := []string{"google-chrome", "google-chrome-stable", "chromium", "chromium-browser", "chrome", "/snap/bin/chromium"}
	switch runtime.GOOS {
	case "darwin":
		names = []string{
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
		}
	case "windows":
		names = []string{"chrome", `C:\Program Files\Google\Chrome\Application\chrome.exe`, "msedge"}
	}
	if p := os.Getenv("CHROME"); p != "" {
		names = []string{p}
	}
	for _, name := range names {
		if path, err := exec.LookPath(name); err == nil {
			return path
		}
	}
	return ""
}

// ---------- activity ----------

// whoWidth fits the longest token name, such as plucky-thistle. toolWidth fits the
// longest tool name, download_project.
const (
	whoWidth  = 16
	toolWidth = 18
)

// Activity prints one line per event. The lock keeps lines from mixing.
type Activity struct {
	mu sync.Mutex
}

var activity Activity

// Call logs a finished tool call by an agent.
func (a *Activity) Call(agent, tool, note string, took time.Duration, err error) {
	name := pad(tool, toolWidth)
	switch {
	case err != nil:
		name, note = red(name), red(short(err.Error()))
	case tool == "upload" || tool == "write_memory":
		name = green(name)
	case tool == "screenshot":
		name = cyan(name)
	}
	a.line(magenta(pad(agent, whoWidth)), name, note+dim(duration(took)))
}

// Disk logs a version that changed or went away without moki, for example by an agent
// that writes the folder itself.
func (a *Activity) Disk(slug, ver string, gone bool) {
	what := "changed"
	if gone {
		what = "removed"
	}
	a.line(dim(pad("disk", whoWidth)), yellow(pad(what, toolWidth)), slug+" "+ver)
}

// Browser logs a change made in the moki page.
func (a *Activity) Browser(text string) {
	a.line(dim(pad("browser", whoWidth)), green(pad("token", toolWidth)), text)
}

// Warn logs a problem that moki can live with.
func (a *Activity) Warn(format string, args ...any) {
	a.line(dim(pad("moki", whoWidth)), yellow(pad("warning", toolWidth)), fmt.Sprintf(format, args...))
}

func (a *Activity) line(who, what, text string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	fmt.Printf("  %s  %s%s%s\n", dim(time.Now().Format("15:04:05")), who, what, text)
}

type noteKey struct{}

// note sets the text that the activity line of this tool call shows.
func note(ctx context.Context, format string, args ...any) {
	if p, ok := ctx.Value(noteKey{}).(*string); ok {
		*p = fmt.Sprintf(format, args...)
	}
}

// ---------- text ----------

var colorOn = useColor()

func useColor() bool {
	if os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
		return false
	}
	fi, err := os.Stdout.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}

func paint(code, s string) string {
	if !colorOn || s == "" {
		return s
	}
	return "\x1b[" + code + "m" + s + "\x1b[0m"
}

func bold(s string) string    { return paint("1", s) }
func dim(s string) string     { return paint("2", s) }
func red(s string) string     { return paint("31", s) }
func green(s string) string   { return paint("32", s) }
func yellow(s string) string  { return paint("33", s) }
func magenta(s string) string { return paint("35", s) }
func cyan(s string) string    { return paint("36", s) }

// pad fills s with spaces to width, so colored columns line up.
func pad(s string, width int) string {
	n := utf8.RuneCountInString(s)
	if n >= width {
		return s + " "
	}
	return s + strings.Repeat(" ", width-n)
}

// short is the first line of s, cut at 100 characters.
func short(s string) string {
	s, _, _ = strings.Cut(s, "\n")
	if r := []rune(s); len(r) > 100 {
		return string(r[:99]) + "…"
	}
	return s
}

// count writes n with the noun, plural when n is not 1: "1 file", "3 files".
func count(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

// byteSize writes a byte count as bytes, KB or MB.
func byteSize(n int) string {
	switch {
	case n < 1<<10:
		return count(n, "byte")
	case n < 1<<20:
		return fmt.Sprintf("%d KB", n>>10)
	}
	return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
}

// duration is the time a slow call took, or "" for a quick one.
func duration(d time.Duration) string {
	switch {
	case d < 100*time.Millisecond:
		return ""
	case d < time.Second:
		return fmt.Sprintf("  %dms", d.Milliseconds())
	}
	return fmt.Sprintf("  %.1fs", d.Seconds())
}

// ago writes a past time the way the UI does: "just now", "5 min ago", "3 h ago", "Nov 8".
func ago(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%d min ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%d h ago", int(d.Hours()))
	}
	return t.Local().Format("Jan 2")
}

func tildePath(p string) string {
	home, err := os.UserHomeDir()
	if err != nil || !strings.HasPrefix(p, home+string(filepath.Separator)) {
		return p
	}
	return "~" + p[len(home):]
}
