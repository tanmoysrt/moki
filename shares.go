package main

import (
	"context"
	"crypto/rand"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Shares keeps the public links in the shares file, one "<id> <slug>" line each.
// A link opens one project with no password. Only an agent turns it on or off, with
// the share tool. Off removes the line, so sharing again makes a new link.
type Shares struct {
	mu   sync.Mutex
	path string
}

var shareIDRe = regexp.MustCompile(`^[a-z2-7]{16}$`)

// All maps each public slug to its id.
func (s *Shares) All() map[string]string {
	b, _ := os.ReadFile(s.path)
	out := map[string]string{}
	for _, line := range strings.Split(string(b), "\n") {
		if f := strings.Fields(line); len(f) == 2 {
			out[f[1]] = f[0]
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
	all := s.All()
	if public == (all[slug] != "") {
		return all[slug], nil
	}
	delete(all, slug)
	if public {
		all[slug] = strings.ToLower(rand.Text()[:16])
	}
	return all[slug], s.write(all)
}

func (s *Shares) write(all map[string]string) error {
	slugs := make([]string, 0, len(all))
	for slug := range all {
		slugs = append(slugs, slug)
	}
	sort.Strings(slugs)
	var b strings.Builder
	for _, slug := range slugs {
		fmt.Fprintf(&b, "%s %s\n", all[slug], slug)
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
	"Public returns a link that anyone can open without the moki password: every version, read only. " +
	"Private turns the link off. Sharing again makes a new link."

func (s *Server) share(ctx context.Context, in shareIn) ([]mcp.Content, error) {
	if err := checkSlug(in.Slug); err != nil {
		return nil, err
	}
	if _, _, err := s.store.Project(in.Slug); err != nil {
		return nil, err
	}
	id, err := s.shares.Set(in.Slug, in.Public)
	if err != nil {
		return nil, err
	}
	s.hub.Publish(Event{Type: "share", Slug: in.Slug})
	text := in.Slug + " is private. Its link is off."
	if in.Public {
		text = in.Slug + " is public. Anyone with this link can see it: " + s.cfg.URL + sharePath(id)
		note(ctx, "%s · public", in.Slug)
	} else {
		note(ctx, "%s · private", in.Slug)
	}
	return []mcp.Content{&mcp.TextContent{Text: text}}, nil
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
	s.writeProject(w, r, slug, "")
}

// isOpen says if a path needs no password: the links, and the files of the moki page.
func isOpen(p string) bool {
	return openPaths[p] || strings.HasPrefix(p, "/s/") || strings.HasPrefix(p, "/ui/") || strings.HasPrefix(p, "/kit/")
}
