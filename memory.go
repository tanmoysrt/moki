package main

import (
	"os"
	"path/filepath"
	"strings"
	"time"
)

// maxMemory is the size at which write_memory asks the agent to make the memory
// shorter. get_project returns all of it, so a long memory fills the agent's context.
const maxMemory = 8 << 10

// maxHits caps a memory search, so a common word does not return every file.
const maxHits = 50

type MemoryInfo struct {
	Slug  string    `json:"slug"`
	Time  time.Time `json:"time"`
	First string    `json:"first_line"`
}

// MemoryHit is a line that matches a search, with its "## v3" heading and the
// lines next to it.
type MemoryHit struct {
	Slug    string `json:"slug"`
	Section string `json:"section,omitempty"`
	Line    int    `json:"line"`
	Text    string `json:"text"`
}

func (s *Store) ReadMemory(slug string) string {
	b, _ := os.ReadFile(s.memoryPath(slug))
	return string(b)
}

func (s *Store) WriteMemory(slug, content string, add bool) error {
	if err := os.MkdirAll(filepath.Join(s.dir, "memory"), 0o755); err != nil {
		return err
	}
	if add {
		if old := s.ReadMemory(slug); old != "" && !strings.HasSuffix(old, "\n") {
			content = "\n" + content
		}
		f, err := os.OpenFile(s.memoryPath(slug), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = f.WriteString(content)
		return err
	}
	return os.WriteFile(s.memoryPath(slug), []byte(content), 0o644)
}

func (s *Store) ListMemory() []MemoryInfo {
	list := []MemoryInfo{}
	for _, e := range s.memoryFiles() {
		slug := strings.TrimSuffix(e.Name(), ".md")
		info, _ := e.Info()
		first, _, _ := strings.Cut(strings.TrimSpace(s.ReadMemory(slug)), "\n")
		list = append(list, MemoryInfo{Slug: slug, Time: info.ModTime(), First: first})
	}
	return list
}

// SearchMemory finds the lines that have each word of query, in any case.
// more is true when there are more hits than maxHits.
func (s *Store) SearchMemory(query string) (hits []MemoryHit, more bool) {
	words := strings.Fields(strings.ToLower(query))
	hits = []MemoryHit{}
	for _, e := range s.memoryFiles() {
		slug := strings.TrimSuffix(e.Name(), ".md")
		for _, h := range searchLines(slug, s.ReadMemory(slug), words) {
			if len(hits) == maxHits {
				return hits, true
			}
			hits = append(hits, h)
		}
	}
	return hits, false
}

func (s *Store) memoryFiles() []os.DirEntry {
	entries, _ := os.ReadDir(filepath.Join(s.dir, "memory"))
	var out []os.DirEntry
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
			out = append(out, e)
		}
	}
	return out
}

func (s *Store) memoryPath(slug string) string {
	return filepath.Join(s.dir, "memory", slug+".md")
}

func searchLines(slug, text string, words []string) []MemoryHit {
	lines := strings.Split(text, "\n")
	var hits []MemoryHit
	section := ""
	for i, line := range lines {
		if strings.HasPrefix(line, "#") {
			section = strings.TrimSpace(strings.TrimLeft(line, "#"))
		}
		if hasWords(strings.ToLower(line), words) {
			lo, hi := max(i-1, 0), min(i+2, len(lines))
			hits = append(hits, MemoryHit{slug, section, i + 1, strings.Join(lines[lo:hi], "\n")})
		}
	}
	return hits
}

func hasWords(line string, words []string) bool {
	for _, w := range words {
		if !strings.Contains(line, w) {
			return false
		}
	}
	return len(words) > 0
}
