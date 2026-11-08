package main

import (
	"context"
	"crypto/rand"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/chromedp/chromedp"
)

// Shot is one screenshot to take.
type Shot struct {
	URL      string
	W, H     int
	Scale    float64
	Dark     bool
	Full     bool
	Selector string
	Label    string
}

// Result is one screenshot and what went wrong on its page.
type Result struct {
	Shot
	PNG        []byte
	Console    []string
	Missing    []string
	SideScroll bool
	Err        error
}

// Shooter runs one Chrome batch at a time. Chrome starts for a batch and stops after it.
type Shooter struct {
	mu     sync.Mutex
	origin string
	secret string // a cookie that lets Chrome past basic auth on moki only
}

const batchTimeout = 60 * time.Second

func NewShooter(cfg Config) *Shooter {
	return &Shooter{origin: cfg.localURL(), secret: rand.Text()}
}

// Capture starts Chrome, takes every shot, and stops Chrome.
func (s *Shooter) Capture(ctx context.Context, shots []Shot) ([]Result, error) {
	var out []Result
	err := s.Batch(ctx, func(b *Batch) error { out = b.Take(shots); return nil })
	return out, err
}

// Batch starts Chrome, runs fn with it, and stops Chrome.
func (s *Shooter) Batch(ctx context.Context, fn func(*Batch) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx, cancel := context.WithTimeout(ctx, batchTimeout)
	defer cancel()
	opts := append(chromedp.DefaultExecAllocatorOptions[:], chromedp.Flag("hide-scrollbars", true), chromedp.Flag("mute-audio", true))
	if p := os.Getenv("CHROME"); p != "" {
		opts = append(opts, chromedp.ExecPath(p))
	}
	if inContainer() {
		opts = append(opts, chromedp.NoSandbox) // containers seldom allow the Chrome sandbox
	}
	actx, acancel := chromedp.NewExecAllocator(ctx, opts...)
	defer acancel()
	bctx, bcancel := chromedp.NewContext(actx)
	defer bcancel()
	if err := chromedp.Do(bctx); err != nil {
		return fmt.Errorf("could not start Chrome: %v. Install Chrome or Chromium, or set CHROME to its path", err)
	}
	return fn(&Batch{s: s, ctx: bctx})
}

// Batch is a running Chrome. Take can run many times in one batch.
type Batch struct {
	s   *Shooter
	ctx context.Context
}

// Take takes the shots in up to 4 tabs in parallel and keeps their order.
func (b *Batch) Take(shots []Shot) []Result {
	out := make([]Result, len(shots))
	jobs := make(chan int)
	var wg sync.WaitGroup
	for range min(4, len(shots)) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			t, err := b.newTab()
			for i := range jobs {
				if err != nil {
					out[i] = Result{Shot: shots[i], Err: err}
					continue
				}
				out[i] = t.take(shots[i])
			}
			if t != nil {
				t.cancel()
			}
		}()
	}
	for i := range shots {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
	return out
}

func inContainer() bool {
	for _, f := range []string{"/.dockerenv", "/run/.containerenv"} {
		if _, err := os.Stat(f); err == nil {
			return true
		}
	}
	return false
}
