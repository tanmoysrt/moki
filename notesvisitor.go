package main

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
	"unicode/utf8"
)

// People with a public link can leave notes when its notes switch is on. They give a
// name, and their browser keeps a secret: moki keeps a hash of it with each note, so only
// that browser can delete the note. They cannot approve or reopen.

const (
	maxVisitorNotes = 500 // on one project, so a link cannot fill the disk
	maxName         = 60
)

func (s *Server) apiVisitorNote(w http.ResponseWriter, r *http.Request) {
	slug, ok := s.visitorSlug(w, r)
	if !ok {
		return
	}
	var in Note
	if !s.readNoteBody(w, r, slug, &in) || !newNote(w, &in) {
		return
	}
	in.By = strings.Join(strings.Fields(in.By), " ")
	if in.By == "" || utf8.RuneCountInString(in.By) > maxName || len(in.Key) < 16 {
		http.Error(w, "a note from the link needs a name of up to 60 letters", http.StatusBadRequest)
		return
	}
	in.Key = keyHash(in.Key)
	if s.visitorNotes(slug) >= maxVisitorNotes {
		http.Error(w, "this project has too many notes from the link", http.StatusTooManyRequests)
		return
	}
	s.changeNotes(w, r, slug, in.By+" noted on "+in.Version, func(p *ProjectNotes) { p.Notes = append(p.Notes, in) })
}

// apiVisitorDelete deletes a note of the visitor. The secret comes in X-Moki-Key.
func (s *Server) apiVisitorDelete(w http.ResponseWriter, r *http.Request) {
	slug, ok := s.visitorSlug(w, r)
	if !ok {
		return
	}
	id, key := r.PathValue("note"), keyHash(r.Header.Get("X-Moki-Key"))
	for _, n := range s.notes.Read(slug).Notes {
		if n.ID == id && (n.Key == "" || n.Key != key) {
			http.Error(w, "only who wrote a note can delete it", http.StatusForbidden)
			return
		}
	}
	s.changeNotes(w, r, slug, "a visitor deleted a note", func(p *ProjectNotes) { p.Notes = without(p.Notes, id) })
}

// visitorSlug is the project of the link, when the link is on and open to notes.
func (s *Server) visitorSlug(w http.ResponseWriter, r *http.Request) (string, bool) {
	if !sameOrigin(w, r) {
		return "", false
	}
	slug := s.shares.Slug(r.PathValue("id"))
	if slug == "" || !s.shares.Notes(slug) {
		http.Error(w, "this link takes no notes", http.StatusForbidden)
		return "", false
	}
	return slug, true
}

func (s *Server) visitorNotes(slug string) int {
	n := 0
	for _, x := range s.notes.Read(slug).Notes {
		if x.By != "" {
			n++
		}
	}
	return n
}

func keyHash(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}
