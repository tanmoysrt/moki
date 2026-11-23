package main

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Shares keeps the public links in the shares file, one "<id> <slug>" line each, with
// " notes" at the end when people with the link can leave notes. A link opens one project
// with no password. An agent turns it on or off with the share tool, and the user with the
// switch in the viewer. Off removes the line, so sharing again makes a new link, with notes off.
type Shares struct {
	mu   sync.Mutex
	path string
}

var shareIDRe = regexp.MustCompile(`^[a-z2-7]{16}$`)

type shareLine struct {
	id    string
	notes bool
}

// All maps each public slug to its id.
func (s *Shares) All() map[string]string {
	out := map[string]string{}
	for slug, l := range s.lines() {
		out[slug] = l.id
	}
	return out
}

// Notes says if people with the link of a project can leave notes.
func (s *Shares) Notes(slug string) bool {
	return s.lines()[slug].notes
}

func (s *Shares) lines() map[string]shareLine {
	b, _ := os.ReadFile(s.path)
	out := map[string]shareLine{}
	for _, line := range strings.Split(string(b), "\n") {
		if f := strings.Fields(line); len(f) >= 2 {
			out[f[1]] = shareLine{f[0], len(f) > 2 && f[2] == "notes"}
		}
	}
	return out
}

// Slug finds the project of a link, "" when the link is off.
func (s *Shares) Slug(id string) string {
	if !shareIDRe.MatchString(id) {
		return ""
	}
	for slug, x := range s.All() {
		if x == id {
			return slug
		}
	}
	return ""
}

// Set turns the link of a project on or off and returns its id, "" when off.
// A project that is public already keeps its link.
func (s *Shares) Set(slug string, public bool) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	all := s.lines()
	if public == (all[slug].id != "") {
		return all[slug].id, nil
	}
	delete(all, slug)
	if public {
		all[slug] = shareLine{id: strings.ToLower(rand.Text()[:16])}
	}
	return all[slug].id, s.write(all)
}

// SetNotes lets people with the link leave notes, or stops it. The link must be on.
func (s *Shares) SetNotes(slug string, on bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	all := s.lines()
	l, ok := all[slug]
	if !ok {
		return fmt.Errorf("%s has no public link", slug)
	}
	l.notes = on
	all[slug] = l
	return s.write(all)
}

func (s *Shares) write(all map[string]shareLine) error {
	slugs := make([]string, 0, len(all))
	for slug := range all {
		slugs = append(slugs, slug)
	}
	sort.Strings(slugs)
	var b strings.Builder
	for _, slug := range slugs {
		fmt.Fprintf(&b, "%s %s", all[slug].id, slug)
		if all[slug].notes {
			b.WriteString(" notes")
		}
		b.WriteString("\n")
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, []byte(b.String()), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

// sharePath is the link of a project without the host, "" when it is private.
func sharePath(id string) string {
	if id == "" {
		return ""
	}
	return "/s/" + id + "/"
}

// ---------- tool ----------

type shareIn struct {
	Slug   string `json:"slug" jsonschema:"project slug"`
	Public bool   `json:"public" jsonschema:"true makes a link that opens the project with no password. false turns the link off"`
}

const shareDesc = "Make a project public or private. Only do it when the user asks. " +
	"Public returns a link that anyone can open without the moki password: every version. Only the user can let people with the link leave notes. " +
	"Private turns the link off. Sharing again makes a new link."

func (s *Server) share(ctx context.Context, in shareIn) ([]mcp.Content, error) {
	id, err := s.setShare(in.Slug, in.Public)
	if err != nil {
		return nil, err
	}
	text := in.Slug + " is private. Its link is off."
	if in.Public {
		text = in.Slug + " is public. Anyone with this link can see it: " + s.cfg.URL + sharePath(id)
		note(ctx, "%s · public", in.Slug)
	} else {
		note(ctx, "%s · private", in.Slug)
	}
	return []mcp.Content{&mcp.TextContent{Text: text}}, nil
}

// setShare turns the link of a project on or off and tells the open pages.
func (s *Server) setShare(slug string, public bool) (string, error) {
	if err := checkSlug(slug); err != nil {
		return "", err
	}
	if _, _, err := s.store.Project(slug); err != nil {
		return "", err
	}
	id, err := s.shares.Set(slug, public)
	if err != nil {
		return "", err
	}
	s.hub.Publish(Event{Type: "share", Slug: slug})
	return id, nil
}

// ---------- routes ----------

// A link serves the moki page, the project as JSON, and the files of its versions.
// The page sees the /s/<id>/ path and shows only the viewer.
func (s *Server) shareRoutes() {
	m := s.mux
	m.HandleFunc("GET /s/{id}", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, r.URL.Path+"/", http.StatusMovedPermanently)
	})
	m.HandleFunc("GET /s/{id}/{$}", func(w http.ResponseWriter, r *http.Request) { serveAsset(w, r, "assets/ui/index.html") })
	m.HandleFunc("GET /s/{id}/api", s.apiShare)
	m.HandleFunc("POST /s/{id}/notes", s.apiVisitorNote)
	m.HandleFunc("DELETE /s/{id}/notes/{note}", s.apiVisitorDelete)
	m.HandleFunc("POST /api/projects/{slug}/share", s.apiSetShare)
	m.HandleFunc("GET /s/{id}/{ver}/{file...}", func(w http.ResponseWriter, r *http.Request) {
		slug := s.shares.Slug(r.PathValue("id"))
		if slug == "" {
			http.NotFound(w, r)
			return
		}
		s.serveFiles(w, r, slug, r.PathValue("ver"), r.PathValue("file"))
	})
}

func (s *Server) apiShare(w http.ResponseWriter, r *http.Request) {
	slug := s.shares.Slug(r.PathValue("id"))
	if slug == "" {
		http.Error(w, "this link is off", http.StatusNotFound)
		return
	}
	var notes *ProjectNotes
	if s.shares.Notes(slug) {
		p := s.notes.Read(slug)
		notes = &p
	}
	s.writeProject(w, r, slug, "", notes)
}

// apiSetShare is the switch in the viewer. The body is {"public": true} or false, and
// "notes" for the second switch, which lets people with the link leave notes.
// It must come from a moki page, so a page on another site cannot share a project.
func (s *Server) apiSetShare(w http.ResponseWriter, r *http.Request) {
	if !sameOrigin(w, r) {
		return
	}
	var in struct {
		Public bool  `json:"public"`
		Notes  *bool `json:"notes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	slug := r.PathValue("slug")
	id, err := s.setShare(slug, in.Public)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	state := "private"
	if in.Public {
		state = "public"
	}
	if in.Notes != nil && id != "" {
		if err := s.shares.SetNotes(slug, *in.Notes); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		state = map[bool]string{true: "open to notes from the link", false: "closed to notes from the link"}[*in.Notes]
		s.hub.Publish(Event{Type: "share", Slug: slug})
	}
	activity.Browser("share", "made "+slug+" "+state)
	writeJSON(w, r, struct {
		Share      string `json:"share"`
		ShareNotes bool   `json:"shareNotes"`
	}{sharePath(id), s.shares.Notes(slug)})
}

// isOpen says if a path needs no password: the links, the files of the moki page,
// and /api/upload, which checks the agent token like /mcp.
func isOpen(p string) bool {
	return openPaths[p] || strings.HasPrefix(p, "/s/") || strings.HasPrefix(p, "/api/upload/") || strings.HasPrefix(p, "/ui/") || strings.HasPrefix(p, "/kit/")
}
