package main

import (
	"cmp"
	"context"
	"crypto/rand"
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// exportReq is one export: a page of a version, in one format.
type exportReq struct {
	Slug, Ver, Page string
	Format          string // png, pdf, zip, mp4, svg or files
	Dark            bool
	Slide           int     // deck png: the slide, from 1
	Time            float64 // video png: the time in seconds
}

var exportTypes = map[string]string{
	"png": "image/png", "pdf": "application/pdf", "svg": "image/svg+xml", "mp4": "video/mp4", "files": "application/gzip",
	"zip": "application/zip",
}

const exportTimeout = 5 * time.Minute

func (req exportReq) check() error {
	if err := checkSlug(req.Slug); err != nil {
		return err
	}
	if err := checkVersion(req.Ver); err != nil {
		return err
	}
	if _, ok := exportTypes[req.Format]; !ok {
		return fmt.Errorf("format is png, pdf, zip, mp4, svg or files")
	}
	p, _, _ := strings.Cut(req.Page, "?")
	if strings.Contains(p, "..") || strings.HasPrefix(p, "/") || strings.Contains(p, "://") {
		return fmt.Errorf("page is a path inside the version, for example index.html")
	}
	return nil
}

// name is the file name, for example billing-v3.pdf or pitch-v2-deck-s4.png.
func (req exportReq) name() string {
	ext := map[string]string{"files": "tar.gz"}[req.Format]
	if ext == "" {
		ext = req.Format
	}
	n := req.Slug + "-" + req.Ver
	if p, _, _ := strings.Cut(req.Page, "?"); p != "" && p != "index.html" {
		n += "-" + strings.TrimSuffix(path.Base(p), ".html")
	}
	if req.Format == "png" && req.Slide > 0 {
		n += "-s" + strconv.Itoa(req.Slide)
	}
	return n + "." + ext
}

// make makes the file. Files is the folder of the version; everything else needs Chrome.
func (s *Server) makeExport(ctx context.Context, req exportReq, prog func(float64)) ([]byte, error) {
	if req.Format == "files" {
		return tarGz(s.cfg.Dir, path.Join(req.Slug, req.Ver))
	}
	ctx, cancel := context.WithTimeout(ctx, exportTimeout)
	defer cancel()
	return s.render(ctx, req, prog)
}

// exportJob runs in the background, so the viewer can show progress and stop it.
type exportJob struct {
	mu     sync.Mutex
	req    exportReq
	done   float64
	data   []byte
	err    error
	ready  bool
	cancel context.CancelFunc
	start  time.Time
}

type exportJobs struct {
	mu   sync.Mutex
	jobs map[string]*exportJob
}

func (s *Server) exportRoutes() {
	s.jobs = &exportJobs{jobs: map[string]*exportJob{}}
	s.mux.HandleFunc("POST /api/export/{slug}/{ver}", s.apiStartExport)
	s.mux.HandleFunc("GET /api/export/{id}", s.apiExportStatus)
	s.mux.HandleFunc("GET /api/export/{id}/file", s.apiExportFile)
	s.mux.HandleFunc("DELETE /api/export/{id}", s.apiStopExport)
}

func (s *Server) apiStartExport(w http.ResponseWriter, r *http.Request) {
	if !sameOrigin(w, r) {
		return
	}
	q := r.URL.Query()
	slide, _ := strconv.Atoi(q.Get("slide"))
	at, _ := strconv.ParseFloat(q.Get("time"), 64)
	req := exportReq{Slug: r.PathValue("slug"), Ver: r.PathValue("ver"), Page: q.Get("page"), Format: q.Get("format"),
		Dark: q.Get("theme") == "dark", Slide: slide, Time: at}
	if req.Page == "" {
		req.Page = "index.html"
	}
	if err := req.check(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, r, map[string]string{"id": s.jobs.start(s, req)})
}

func (s *Server) apiExportStatus(w http.ResponseWriter, r *http.Request) {
	j := s.jobs.get(r.PathValue("id"))
	if j == nil {
		http.NotFound(w, r)
		return
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	out := map[string]any{"done": j.done, "ready": j.ready}
	if j.err != nil {
		out["error"] = j.err.Error()
	}
	writeJSON(w, r, out)
}

func (s *Server) apiExportFile(w http.ResponseWriter, r *http.Request) {
	j := s.jobs.get(r.PathValue("id"))
	if j == nil {
		http.NotFound(w, r)
		return
	}
	j.mu.Lock()
	data, ready := j.data, j.ready && j.err == nil
	j.mu.Unlock()
	if !ready {
		http.Error(w, "the file is not ready", http.StatusConflict)
		return
	}
	w.Header().Set("Content-Type", exportTypes[j.req.Format])
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", j.req.name()))
	w.Write(data)
}

func (s *Server) apiStopExport(w http.ResponseWriter, r *http.Request) {
	if !sameOrigin(w, r) {
		return
	}
	if j := s.jobs.get(r.PathValue("id")); j != nil {
		j.cancel()
	}
	w.WriteHeader(http.StatusNoContent)
}

// start runs the export in the background and drops jobs older than 15 minutes.
func (js *exportJobs) start(s *Server, req exportReq) string {
	ctx, cancel := context.WithCancel(context.Background())
	j := &exportJob{req: req, cancel: cancel, start: time.Now()}
	id := strings.ToLower(rand.Text()[:12])
	js.mu.Lock()
	for k, old := range js.jobs {
		if time.Since(old.start) > 15*time.Minute {
			old.cancel()
			delete(js.jobs, k)
		}
	}
	js.jobs[id] = j
	js.mu.Unlock()
	go func() {
		defer cancel()
		data, err := s.makeExport(ctx, req, func(p float64) { j.mu.Lock(); j.done = p; j.mu.Unlock() })
		if ctx.Err() == context.Canceled {
			err = fmt.Errorf("stopped")
		}
		j.mu.Lock()
		j.data, j.err, j.ready, j.done = data, err, true, 1
		j.mu.Unlock()
		if err != nil {
			activity.Warn("export %s: %v", req.name(), err)
		} else {
			activity.Browser("export", fmt.Sprintf("%s · %s", req.name(), byteSize(len(data))))
		}
	}()
	return id
}

func (js *exportJobs) get(id string) *exportJob {
	js.mu.Lock()
	defer js.mu.Unlock()
	return js.jobs[id]
}

type exportIn struct {
	Slug    string  `json:"slug" jsonschema:"project slug"`
	Version string  `json:"version,omitempty" jsonschema:"for example v3. Default: the newest"`
	Page    string  `json:"page,omitempty" jsonschema:"page in the version, with a query if needed. Default index.html"`
	Format  string  `json:"format" jsonschema:"png, pdf, zip (decks: a PNG of each slide), mp4 (videos), svg (drawings) or files (.tar.gz of the version)"`
	Theme   string  `json:"theme,omitempty" jsonschema:"light or dark. Default light"`
	Slide   int     `json:"slide,omitempty" jsonschema:"deck png: the slide, from 1"`
	Time    float64 `json:"time,omitempty" jsonschema:"video png: the time in seconds"`
	Dir     string  `json:"dir,omitempty" jsonschema:"absolute folder on the moki machine to write the file into, instead of returning it"`
}

const exportDesc = `Export a page as a file people can use: png or pdf of any page; pdf, a zip with a png of each slide,
or a png of one slide for a deck (/kit/deck.js); mp4 or a png of one frame for a video (/kit/video.js); svg for a drawing; files for the
version as .tar.gz. The file comes back as a resource of up to 20 MB, or goes into dir on the moki machine.
The user has the same files under the Export button (E) in the viewer.`

func (s *Server) export(ctx context.Context, in exportIn) ([]mcp.Content, error) {
	if err := checkSlug(in.Slug); err != nil {
		return nil, err
	}
	p, _, err := s.store.Project(in.Slug)
	if err != nil {
		return nil, err
	}
	req := exportReq{Slug: in.Slug, Ver: cmp.Or(in.Version, p.Version), Page: cmp.Or(in.Page, "index.html"),
		Format: in.Format, Dark: in.Theme == "dark", Slide: in.Slide, Time: in.Time}
	if err := req.check(); err != nil {
		return nil, err
	}
	data, err := s.makeExport(ctx, req, func(float64) {})
	if err != nil {
		return nil, err
	}
	name := req.name()
	if in.Dir != "" {
		dir, err := absPath(in.Dir)
		if err != nil {
			return nil, err
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
		file := filepath.Join(dir, name)
		if err := os.WriteFile(file, data, 0o644); err != nil {
			return nil, err
		}
		note(ctx, "%s · %s to %s", name, byteSize(len(data)), tildePath(dir))
		return textContent("Wrote %s (%s).", file, byteSize(len(data))), nil
	}
	if len(data) > maxDownload {
		return nil, fmt.Errorf("%s is %d MB, above the 20 MB limit. Use dir to write it on the moki machine", name, len(data)>>20)
	}
	note(ctx, "%s · %s", name, byteSize(len(data)))
	return []mcp.Content{
		&mcp.TextContent{Text: fmt.Sprintf("%s: %s.", name, byteSize(len(data)))},
		&mcp.EmbeddedResource{Resource: &mcp.ResourceContents{URI: "moki://export/" + name, MIMEType: exportTypes[req.Format], Blob: data}},
	}, nil
}
