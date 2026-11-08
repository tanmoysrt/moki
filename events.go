package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Event goes to every client on /api/events.
// "change": a version is new, changed or gone. "thumb": its thumbnail is ready.
// "agent": an agent used its token for the first time. "share": a project went public or private.
type Event struct {
	Type    string `json:"-"`
	Slug    string `json:"slug"`
	Version string `json:"version"`
}

// Hub sends events to the browser clients and watches the folders for changes.
type Hub struct {
	mu       sync.Mutex
	clients  map[chan Event]struct{}
	stamps   map[string]time.Time
	onChange func(slug, ver string)
}

func NewHub(onChange func(slug, ver string)) *Hub {
	return &Hub{clients: map[chan Event]struct{}{}, stamps: map[string]time.Time{}, onChange: onChange}
}

// Changed records a version that moki wrote itself and sends the event at once,
// so the watcher does not send it a second time.
func (h *Hub) Changed(dir, slug, ver string) {
	h.mu.Lock()
	h.stamps[slug+"/"+ver] = stamp(filepath.Join(dir, slug, ver))
	h.mu.Unlock()
	h.change(slug, ver)
}

func (h *Hub) change(slug, ver string) {
	h.Publish(Event{Type: "change", Slug: slug, Version: ver})
	if h.onChange != nil {
		h.onChange(slug, ver)
	}
}

func (h *Hub) Publish(ev Event) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for c := range h.clients {
		select {
		case c <- ev:
		default: // a slow client misses an event, it does not block the others
		}
	}
}

// Watch checks the slug and version folders every second. This also finds files
// that an agent writes directly to disk.
func (h *Hub) Watch(dir string) {
	h.mu.Lock()
	h.stamps = scan(dir)
	h.mu.Unlock()
	for range time.Tick(time.Second) {
		now := scan(dir)
		h.mu.Lock()
		old := h.stamps
		h.stamps = now
		h.mu.Unlock()
		for key, t := range now {
			if !old[key].Equal(t) {
				h.diskChange(key, false)
			}
		}
		for key := range old {
			if _, ok := now[key]; !ok {
				h.diskChange(key, true)
			}
		}
	}
}

func (h *Hub) diskChange(key string, gone bool) {
	slug, ver := splitKey(key)
	activity.Disk(slug, ver, gone)
	h.change(slug, ver)
}

func scan(dir string) map[string]time.Time {
	out := map[string]time.Time{}
	slugs, _ := os.ReadDir(dir)
	for _, s := range slugs {
		if !s.IsDir() || checkSlug(s.Name()) != nil {
			continue
		}
		vers, _ := os.ReadDir(filepath.Join(dir, s.Name()))
		for _, v := range vers {
			if v.IsDir() && versionRe.MatchString(v.Name()) {
				out[s.Name()+"/"+v.Name()] = stamp(filepath.Join(dir, s.Name(), v.Name()))
			}
		}
	}
	return out
}

func splitKey(key string) (string, string) {
	slug, ver := filepath.Split(key)
	return slug[:len(slug)-1], ver
}

// ServeHTTP streams the events as server-sent events.
func (h *Hub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming is not supported", 500)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	c := make(chan Event, 16)
	h.mu.Lock()
	h.clients[c] = struct{}{}
	h.mu.Unlock()
	defer func() {
		h.mu.Lock()
		delete(h.clients, c)
		h.mu.Unlock()
	}()
	fmt.Fprint(w, "retry: 2000\n\n")
	flusher.Flush()
	ping := time.NewTicker(25 * time.Second)
	defer ping.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case ev := <-c:
			b, _ := json.Marshal(ev)
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", ev.Type, b)
		case <-ping.C:
			fmt.Fprint(w, ": ping\n\n")
		}
		flusher.Flush()
	}
}
