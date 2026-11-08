package main

import (
	"bytes"
	"crypto/subtle"
	"embed"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io/fs"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

//go:embed assets
var assets embed.FS

// Server holds every part of moki and routes the HTTP requests.
type Server struct {
	cfg    Config
	store  *Store
	hub    *Hub
	tokens *Tokens
	shares *Shares
	shots  *Shooter
	thumbs *Thumbs
	mux    *http.ServeMux
}

func NewServer(cfg Config) *Server {
	s := &Server{
		cfg:    cfg,
		store:  &Store{dir: cfg.Dir},
		tokens: &Tokens{path: filepath.Join(cfg.Dir, "tokens")},
		shares: &Shares{path: filepath.Join(cfg.Dir, "shares")},
		mux:    http.NewServeMux(),
	}
	s.shots = NewShooter(cfg)
	s.thumbs = NewThumbs(s)
	s.hub = NewHub(s.thumbs.Queue)
	s.tokens.onFirst = func() { s.hub.Publish(Event{Type: "agent"}) }
	s.routes()
	go s.hub.Watch(cfg.Dir)
	go s.thumbs.Run()
	return s
}

func (s *Server) routes() {
	m := s.mux
	m.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) { serveAsset(w, r, "assets/ui/index.html") })
	m.HandleFunc("GET /ui/{file...}", func(w http.ResponseWriter, r *http.Request) { serveAsset(w, r, "assets/ui/"+r.PathValue("file")) })
	m.HandleFunc("GET /kit/{file...}", func(w http.ResponseWriter, r *http.Request) { serveAsset(w, r, "assets/kit/"+r.PathValue("file")) })
	m.HandleFunc("GET /p/{slug}/{ver}/{file...}", s.serveVersion)
	m.HandleFunc("GET /api/projects", s.apiProjects)
	m.HandleFunc("GET /api/projects/{slug}", s.apiProject)
	m.Handle("GET /api/events", s.hub)
	m.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) { fmt.Fprintln(w, "moki", version) })
	m.HandleFunc("GET /thumb/{slug}/{file}", s.serveThumb)
	m.HandleFunc("GET /llm-guide.txt", func(w http.ResponseWriter, r *http.Request) { writeText(w, r, guideText(s.cfg.URL, nil)) })
	m.HandleFunc("GET /design-skill.txt", func(w http.ResponseWriter, r *http.Request) { writeText(w, r, skillText()) })
	m.HandleFunc("GET /_sheet", s.serveSheet)
	m.HandleFunc("GET /_shot/{id}", s.serveShot)
	m.Handle("/mcp", s.tokens.Require(newMCPHandler(s)))
	s.tokenRoutes()
	s.shareRoutes()
}

// ServeHTTP adds no-cache to every response and checks basic auth.
// /mcp has its own tokens. /health and the guides stay open, so that an agent can
// check that moki runs and read the guide before it has the password. So do the
// public links, and the moki page files they load.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-cache")
	if s.cfg.Username != "" && !isOpen(r.URL.Path) && !s.authorized(r) {
		w.Header().Set("WWW-Authenticate", `Basic realm="moki"`)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	s.mux.ServeHTTP(w, r)
}

var openPaths = map[string]bool{"/mcp": true, "/health": true, "/llm-guide.txt": true, "/design-skill.txt": true}

func (s *Server) authorized(r *http.Request) bool {
	if c, err := r.Cookie("moki_shot"); err == nil && subtle.ConstantTimeCompare([]byte(c.Value), []byte(s.shots.secret)) == 1 {
		return true
	}
	username, password, ok := r.BasicAuth()
	return ok && subtle.ConstantTimeCompare([]byte(username), []byte(s.cfg.Username)) == 1 &&
		subtle.ConstantTimeCompare([]byte(password), []byte(s.cfg.Password)) == 1
}

// serveVersion serves files of one version. Only paths of this shape are served,
// so tokens, memory and .cache stay private. os.Root keeps symlinks inside the folder.
func (s *Server) serveVersion(w http.ResponseWriter, r *http.Request) {
	s.serveFiles(w, r, r.PathValue("slug"), r.PathValue("ver"), r.PathValue("file"))
}

func (s *Server) serveFiles(w http.ResponseWriter, r *http.Request, slug, ver, file string) {
	if checkSlug(slug) != nil || checkVersion(ver) != nil {
		http.NotFound(w, r)
		return
	}
	root, err := os.OpenRoot(s.store.VersionDir(slug, ver))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer root.Close()
	if file == "" || strings.HasSuffix(file, "/") {
		file += "index.html"
	}
	f, err := root.Open(file)
	if err != nil {
		notFound(w, file)
		return
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil || fi.IsDir() {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("ETag", fmt.Sprintf(`"%x-%x"`, fi.Size(), fi.ModTime().UnixNano()))
	http.ServeContent(w, r, fi.Name(), fi.ModTime(), f)
}

// Share is the public link of a project, "" when it is private.
type projectJSON struct {
	Project
	Thumb     string `json:"thumb"`
	ThumbDark string `json:"thumb_dark"`
	Share     string `json:"share,omitempty"`
}

func (s *Server) apiProjects(w http.ResponseWriter, r *http.Request) {
	list := []projectJSON{}
	shares := s.shares.All()
	for _, p := range s.store.Projects() {
		thumb, dark := s.thumbs.URL(p.Slug, p.Version, false), s.thumbs.URL(p.Slug, p.Version, true)
		list = append(list, projectJSON{p, thumb, dark, sharePath(shares[p.Slug])})
	}
	writeJSON(w, r, list)
}

func (s *Server) apiProject(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	s.writeProject(w, r, slug, sharePath(s.shares.All()[slug]))
}

func (s *Server) writeProject(w http.ResponseWriter, r *http.Request, slug, share string) {
	p, vers, err := s.store.Project(slug)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	writeJSON(w, r, struct {
		Project
		Share string    `json:"share,omitempty"`
		List  []Version `json:"list"`
	}{p, share, vers})
}

func (s *Server) serveThumb(w http.ResponseWriter, r *http.Request) {
	slug, ver := r.PathValue("slug"), strings.TrimSuffix(r.PathValue("file"), ".png")
	if checkSlug(slug) != nil || checkVersion(ver) != nil {
		http.NotFound(w, r)
		return
	}
	serveFile(w, r, s.thumbs.Path(slug, ver, r.URL.Query().Get("dark") == "1"))
}

func (s *Server) serveShot(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !shotIDRe.MatchString(id) {
		http.NotFound(w, r)
		return
	}
	serveFile(w, r, filepath.Join(s.cfg.Dir, ".cache", "shots", id))
}

// notFound keeps the type of the file, so the browser reports a 404 and not a wrong type.
func notFound(w http.ResponseWriter, file string) {
	if t := mime.TypeByExtension(path.Ext(file)); t != "" {
		w.Header().Set("Content-Type", t)
	}
	w.WriteHeader(http.StatusNotFound)
}

func serveFile(w http.ResponseWriter, r *http.Request, name string) {
	f, err := os.Open(name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	fi, _ := f.Stat()
	w.Header().Set("ETag", fmt.Sprintf(`"%x-%x"`, fi.Size(), fi.ModTime().UnixNano()))
	http.ServeContent(w, r, fi.Name(), fi.ModTime(), f)
}

func serveAsset(w http.ResponseWriter, r *http.Request, name string) {
	b, err := fs.ReadFile(assets, path.Clean(name))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("ETag", etag(b))
	http.ServeContent(w, r, path.Base(name), time.Time{}, bytes.NewReader(b))
}

func writeJSON(w http.ResponseWriter, r *http.Request, v any) {
	b, _ := json.Marshal(v)
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("ETag", etag(b))
	http.ServeContent(w, r, "", time.Time{}, bytes.NewReader(b))
}

func writeText(w http.ResponseWriter, r *http.Request, text string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("ETag", etag([]byte(text)))
	http.ServeContent(w, r, "", time.Time{}, strings.NewReader(text))
}

func etag(b []byte) string {
	h := fnv.New64a()
	h.Write(b)
	return fmt.Sprintf(`"%x"`, h.Sum64())
}
