package main

import (
	"crypto/rand"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Notes are what the user says about a version in the viewer: pins on spots of a page,
// each with a line of text, and the approved version. People with a public link can add
// notes too, when its notes switch is on. The agent reads the open notes in get_project and
// answers them with answer_notes: done in a version, a reply, or both. They live in
// notes/<slug>.json, apart from the memory, which is the agent's own file.

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
	Done     string    `json:"done,omitempty"`  // the version that fixed it, "" while it is open
	Reply    string    `json:"reply,omitempty"` // the agent's answer
	By       string    `json:"by,omitempty"`    // the name of a visitor of the public link
	Key      string    `json:"key,omitempty"`   // a hash of the visitor's secret: only they can delete it
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

// public is the notes without the keys of the visitors, for the browser.
func (p ProjectNotes) public() *ProjectNotes {
	notes := make([]Note, len(p.Notes))
	for i, n := range p.Notes {
		n.Key = ""
		notes[i] = n
	}
	p.Notes = notes
	return &p
}

func (s *Server) noteRoutes() {
	s.mux.HandleFunc("POST /api/projects/{slug}/notes", s.apiAddNote)
	s.mux.HandleFunc("DELETE /api/projects/{slug}/notes/{id}", s.apiDeleteNote)
	s.mux.HandleFunc("POST /api/projects/{slug}/notes/{id}/reopen", s.apiReopenNote)
	s.mux.HandleFunc("POST /api/projects/{slug}/approve", s.apiApprove)
}

func (s *Server) apiAddNote(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	var in Note
	if !s.readNoteBody(w, r, slug, &in) || !newNote(w, &in) {
		return
	}
	in.By, in.Key = "", ""
	s.changeNotes(w, r, slug, "noted on "+in.Version, func(p *ProjectNotes) { p.Notes = append(p.Notes, in) })
}

// newNote checks the text and the page of a note from the browser, and gives it an id.
func newNote(w http.ResponseWriter, in *Note) bool {
	in.Text = strings.TrimSpace(in.Text)
	if in.Text == "" || len(in.Text) > maxNote || in.Page == "" || in.Version == "" {
		http.Error(w, "a note needs a version, a page and text of up to 2000 bytes", http.StatusBadRequest)
		return false
	}
	in.ID, in.Time = strings.ToLower(rand.Text()[:8]), time.Now()
	in.Done, in.Reply = "", ""
	return true
}

func (s *Server) apiDeleteNote(w http.ResponseWriter, r *http.Request) {
	if !sameOrigin(w, r) {
		return
	}
	id := r.PathValue("id")
	s.changeNotes(w, r, r.PathValue("slug"), "deleted a note", func(p *ProjectNotes) { p.Notes = without(p.Notes, id) })
}

// apiReopenNote sends a done note back to the agent.
func (s *Server) apiReopenNote(w http.ResponseWriter, r *http.Request) {
	if !sameOrigin(w, r) {
		return
	}
	id := r.PathValue("id")
	s.changeNotes(w, r, r.PathValue("slug"), "reopened a note", func(p *ProjectNotes) {
		for i := range p.Notes {
			if p.Notes[i].ID == id {
				p.Notes[i].Done = ""
			}
		}
	})
}

func without(notes []Note, id string) []Note {
	kept := []Note{}
	for _, n := range notes {
		if n.ID != id {
			kept = append(kept, n)
		}
	}
	return kept
}

// apiApprove sets the approved version. An empty version takes the approval back.
func (s *Server) apiApprove(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	var in Note
	if !s.readNoteBody(w, r, slug, &in) {
		return
	}
	what := "approved " + in.Version
	if in.Version == "" {
		what = "took back the approval"
	}
	s.changeNotes(w, r, slug, what, func(p *ProjectNotes) { p.Approved = in.Version })
}

// readNoteBody checks the origin and the version, as the share switch does.
func (s *Server) readNoteBody(w http.ResponseWriter, r *http.Request, slug string, in *Note) bool {
	if !sameOrigin(w, r) {
		return false
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(in); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return false
	}
	if in.Version != "" && (checkVersion(in.Version) != nil || !isDir(s.store.VersionDir(slug, in.Version))) {
		http.Error(w, "no such version", http.StatusNotFound)
		return false
	}
	return true
}

func (s *Server) changeNotes(w http.ResponseWriter, r *http.Request, slug, what string, change func(*ProjectNotes)) {
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
	writeJSON(w, r, notes.public())
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
