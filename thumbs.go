package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Thumbs makes the light and dark thumbnail of the newest version of each project.
// A save adds the version to a pending set. One worker takes the whole set in one Chrome batch.
type Thumbs struct {
	s       *Server
	mu      sync.Mutex
	pending map[string]bool
	wake    chan struct{}
	noted   bool
}

func NewThumbs(s *Server) *Thumbs {
	return &Thumbs{s: s, pending: map[string]bool{}, wake: make(chan struct{}, 1)}
}

func (t *Thumbs) Queue(slug, ver string) {
	t.mu.Lock()
	t.pending[slug+"/"+ver] = true
	t.mu.Unlock()
	select {
	case t.wake <- struct{}{}:
	default:
	}
}

func (t *Thumbs) Path(slug, ver string, dark bool) string {
	name := ver + ".png"
	if dark {
		name = ver + ".dark.png"
	}
	return filepath.Join(t.s.cfg.Dir, ".cache", slug, name)
}

// URL is the thumbnail URL with its time, or "" when the cache has none or an old one.
func (t *Thumbs) URL(slug, ver string, dark bool) string {
	fi, err := os.Stat(t.Path(slug, ver, dark))
	if err != nil || fi.ModTime().Before(stamp(t.s.store.VersionDir(slug, ver))) {
		return ""
	}
	q := fmt.Sprintf("?t=%d", fi.ModTime().Unix())
	if dark {
		q += "&dark=1"
	}
	return "/thumb/" + slug + "/" + ver + ".png" + q
}

// Run makes the missing thumbnails at start, then waits for saves.
func (t *Thumbs) Run() {
	for _, p := range t.s.store.Projects() {
		if t.URL(p.Slug, p.Version, false) == "" || t.URL(p.Slug, p.Version, true) == "" {
			t.Queue(p.Slug, p.Version)
		}
	}
	for range t.wake {
		time.Sleep(time.Second) // an agent that writes files one by one sends many changes
		t.mu.Lock()
		keys := t.pending
		t.pending = map[string]bool{}
		t.mu.Unlock()
		t.make(keys)
	}
}

func (t *Thumbs) make(keys map[string]bool) {
	var shots []Shot
	var names [][2]string
	for key := range keys {
		slug, ver := splitKey(key)
		p, _, err := t.s.store.Project(slug)
		if err != nil || p.Version != ver {
			continue // only the newest version shows on the home
		}
		for _, dark := range []bool{false, true} {
			shots = append(shots, Shot{URL: t.s.pageURL(slug, ver, "index.html"), W: 1280, H: 800, Scale: 1, Dark: dark})
			names = append(names, [2]string{slug, ver})
		}
	}
	if len(shots) == 0 {
		return
	}
	results, err := t.s.shots.Capture(context.Background(), shots)
	if err != nil {
		if !t.noted {
			activity.Warn("no thumbnails: %v. The home shows live pages instead.", err)
			t.noted = true
		}
		return
	}
	for i, r := range results {
		if r.Err != nil {
			continue
		}
		slug, ver := names[i][0], names[i][1]
		path := t.Path(slug, ver, r.Dark)
		os.MkdirAll(filepath.Dir(path), 0o755)
		if os.WriteFile(path, r.PNG, 0o644) == nil && r.Dark {
			t.s.hub.Publish(Event{Type: "thumb", Slug: slug, Version: ver})
		}
	}
}

// pageURL is the address of a page that Chrome opens.
func (s *Server) pageURL(slug, ver, page string) string {
	return s.shots.origin + "/p/" + slug + "/" + ver + "/" + page
}
