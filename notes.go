package main

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Notes are what the user says about a version in the viewer: pins on spots of a page,
// each with a line of text, and the approved version. Only the viewer writes them, and
// the agent reads them in get_project. They live in notes/<slug>.json, apart from the
// memory, which is the agent's own file.

type Note struct {
	ID       string    `json:"id"`
	Version  string    `json:"version"`
	Page     string    `json:"page"`               // index.html, or page.html?v=a
	Target   string    `json:"target,omitempty"`   // what the user clicked: button "Start trial"
	Selector string    `json:"selector,omitempty"` // a CSS path to that element
	X        float64   `json:"x"`                  // the spot inside the element, from 0 to 1
	Y        float64   `json:"y"`
	Text     string    `json:"text"`
	Time     time.Time `json:"time"`
}

type ProjectNotes struct {
	Approved string `json:"approved,omitempty"`
	Notes    []Note `json:"notes"`
}

type NoteStore struct {
	dir string
	mu  sync.Mutex
}

const maxNote = 2000

func (s *Server) noteRoutes() {
	s.mux.HandleFunc("POST /api/projects/{slug}/notes", s.apiAddNote)
	s.mux.HandleFunc("DELETE /api/projects/{slug}/notes/{id}", s.apiDeleteNote)
	s.mux.HandleFunc("POST /api/projects/{slug}/approve", s.apiApprove)
}

func (s *Server) apiAddNote(w http.ResponseWriter, r *http.Request) {
	var in Note
	if !s.readNoteBody(w, r, &in) {
		return
	}
	in.Text = strings.TrimSpace(in.Text)
	if in.Text == "" || len(in.Text) > maxNote || in.Page == "" {
		http.Error(w, "a note needs a page and text of up to 2000 bytes", http.StatusBadRequest)
		return
	}
	in.ID, in.Time = strings.ToLower(rand.Text()[:8]), time.Now()
	s.changeNotes(w, r, "noted on "+in.Version, func(p *ProjectNotes) { p.Notes = append(p.Notes, in) })
}

func (s *Server) apiDeleteNote(w http.ResponseWriter, r *http.Request) {
	if !sameOrigin(w, r) {
		return
	}
	id := r.PathValue("id")
	s.changeNotes(w, r, "deleted a note", func(p *ProjectNotes) {
		kept := []Note{}
		for _, n := range p.Notes {
			if n.ID != id {
				kept = append(kept, n)
			}
		}
		p.Notes = kept
	})
}

// apiApprove sets the approved version. An empty version takes the approval back.
func (s *Server) apiApprove(w http.ResponseWriter, r *http.Request) {
	var in Note
	if !s.readNoteBody(w, r, &in) {
		return
	}
	what := "approved " + in.Version
	if in.Version == "" {
		what = "took back the approval"
	}
	s.changeNotes(w, r, what, func(p *ProjectNotes) { p.Approved = in.Version })
}

// readNoteBody checks the origin and the version, as the share switch does.
func (s *Server) readNoteBody(w http.ResponseWriter, r *http.Request, in *Note) bool {
	if !sameOrigin(w, r) {
		return false
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(in); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return false
	}
	if in.Version != "" && !isDir(s.store.VersionDir(r.PathValue("slug"), in.Version)) {
		http.Error(w, "no such version", http.StatusNotFound)
		return false
	}
	return true
}

func (s *Server) changeNotes(w http.ResponseWriter, r *http.Request, what string, change func(*ProjectNotes)) {
	slug := r.PathValue("slug")
	if checkSlug(slug) != nil || !isDir(filepath.Join(s.cfg.Dir, slug)) {
		http.NotFound(w, r)
		return
	}
	notes, err := s.notes.Update(slug, change)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	activity.Browser("notes", what+" of "+slug)
	s.hub.Publish(Event{Type: "notes", Slug: slug})
	writeJSON(w, r, notes)
}

// ---------- store ----------

func (n *NoteStore) Read(slug string) ProjectNotes {
	p := ProjectNotes{Notes: []Note{}}
	b, err := os.ReadFile(n.path(slug))
	if err == nil {
		json.Unmarshal(b, &p)
	}
	return p
}

func (n *NoteStore) Update(slug string, change func(*ProjectNotes)) (ProjectNotes, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	p := n.Read(slug)
	change(&p)
	if err := os.MkdirAll(n.dir, 0o755); err != nil {
		return p, err
	}
	b, _ := json.MarshalIndent(p, "", "  ")
	tmp := n.path(slug) + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return p, err
	}
	return p, os.Rename(tmp, n.path(slug))
}

func (n *NoteStore) path(slug string) string {
	return filepath.Join(n.dir, slug+".json")
}

// ---------- for the agent ----------

// AgentNotes is the text that get_project shows: the approved version, then the notes
// of each version, numbered as the viewer numbers them.
func (n *NoteStore) AgentNotes(slug string) string {
	p := n.Read(slug)
	var b strings.Builder
	if p.Approved != "" {
		fmt.Fprintf(&b, "Approved: %s\n", p.Approved)
	}
	count := map[string]int{}
	for _, x := range p.Notes {
		count[x.Version]++
		fmt.Fprintf(&b, "%s #%d · %s", x.Version, count[x.Version], x.Page)
		if x.Target != "" {
			fmt.Fprintf(&b, " · %s", x.Target)
		}
		if x.Selector != "" {
			fmt.Fprintf(&b, " (%s)", x.Selector)
		}
		fmt.Fprintf(&b, "\n  %s\n", strings.ReplaceAll(x.Text, "\n", "\n  "))
	}
	return b.String()
}
