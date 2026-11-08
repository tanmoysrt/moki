package main

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

// tab is one Chrome tab. It counts open requests and keeps the errors of the page.
type tab struct {
	ctx      context.Context
	cancel   context.CancelFunc
	origin   string
	mu       sync.Mutex
	inflight map[network.RequestID]string
	last     time.Time
	console  []string
	missing  []string
}

func (b *Batch) newTab() (*tab, error) {
	ctx, cancel := chromedp.NewContext(b.ctx)
	t := &tab{ctx: ctx, cancel: cancel, origin: b.s.origin, inflight: map[network.RequestID]string{}}
	host, _ := url.Parse(b.s.origin)
	cookie := network.SetCookieParams{Name: "moki_shot", Value: b.s.secret, Domain: host.Hostname(), Path: "/"}
	if _, err := chromedp.Call(ctx, network.SetCookie, cookie); err != nil {
		cancel()
		return nil, err
	}
	msgs := chromedp.Console(ctx)
	go func() {
		for m, err := range msgs {
			if err != nil {
				return
			}
			if (m.Type == chromedp.ConsoleError || m.IsException()) && m.Source != "network" {
				t.mu.Lock()
				t.console = append(t.console, m.String())
				t.mu.Unlock()
			}
		}
	}()
	return t, chromedp.Listen(ctx, t.onEvent)
}

func (t *tab) onEvent(ev any) {
	t.mu.Lock()
	defer t.mu.Unlock()
	switch e := ev.(type) {
	case *network.EventRequestWillBeSent:
		t.inflight[e.RequestID] = e.Request.URL
	case *network.EventLoadingFinished:
		delete(t.inflight, e.RequestID)
	case *network.EventLoadingFailed:
		if !e.Canceled {
			t.addMissing(t.inflight[e.RequestID], e.ErrorText)
		}
		delete(t.inflight, e.RequestID)
	case *network.EventResponseReceived:
		if e.Response.Status >= 400 {
			t.addMissing(e.Response.URL, fmt.Sprint(e.Response.Status))
		}
	default:
		return
	}
	t.last = time.Now()
}

// addMissing records a local file that did not load. Files from other hosts are fine.
func (t *tab) addMissing(u, why string) {
	path, ok := strings.CutPrefix(u, t.origin+"/")
	path, _, _ = strings.Cut(path, "?")
	if !ok || path == "favicon.ico" {
		return
	}
	for _, m := range t.missing {
		if strings.HasPrefix(m, "/"+path+" ") {
			return
		}
	}
	t.missing = append(t.missing, fmt.Sprintf("/%s (%s)", path, why))
}

func (t *tab) take(sh Shot) Result {
	t.mu.Lock()
	t.console, t.missing, t.inflight = nil, nil, map[network.RequestID]string{}
	t.mu.Unlock()
	ctx, cancel := context.WithTimeout(t.ctx, 30*time.Second)
	defer cancel()
	res := Result{Shot: sh}
	res.SideScroll, res.PNG, res.Err = t.shoot(ctx, sh)
	if errors.Is(res.Err, context.DeadlineExceeded) {
		res.Err = fmt.Errorf("the page took longer than 30 seconds")
	}
	t.mu.Lock()
	res.Console, res.Missing = t.console, t.missing
	t.mu.Unlock()
	return res
}

func (t *tab) shoot(ctx context.Context, sh Shot) (bool, []byte, error) {
	if err := t.open(ctx, sh); err != nil {
		return false, nil, err
	}
	side, _ := chromedp.Run(ctx, chromedp.Evaluate[bool](`document.documentElement.scrollWidth > innerWidth + 1`))
	var png []byte
	var err error
	switch {
	case sh.Selector != "":
		sctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		png, err = chromedp.Run(sctx, chromedp.Screenshot(chromedp.CSS(sh.Selector)))
		if err != nil && ctx.Err() == nil {
			err = fmt.Errorf("no visible element matches the selector %q", sh.Selector)
		}
	case sh.Full:
		png, err = t.full(ctx, sh)
	default:
		png, err = chromedp.Run(ctx, chromedp.CaptureScreenshot())
	}
	return side, png, err
}

// open sets the viewport and the theme, opens the page and waits until it is still.
func (t *tab) open(ctx context.Context, sh Shot) error {
	theme := map[bool]string{true: "dark", false: "light"}[sh.Dark]
	u := sh.URL + map[bool]string{true: "&", false: "?"}[strings.Contains(sh.URL, "?")] + "theme=" + theme
	err := chromedp.Do(ctx,
		t.viewport(sh, sh.H),
		chromedp.Func(func(ctx context.Context, tg *chromedp.Target) error {
			_, err := chromedp.Call(ctx, emulation.SetEmulatedMedia, emulation.SetEmulatedMediaParams{
				Features: []*emulation.MediaFeature{{Name: "prefers-color-scheme", Value: theme}}})
			return err
		}),
		chromedp.Navigate(u),
	)
	if err != nil {
		return err
	}
	// The viewer sets the dark class on the page, so the shot does the same.
	chromedp.Run(ctx, chromedp.Evaluate[bool](fmt.Sprintf(`document.documentElement.classList.toggle('dark', %v)`, sh.Dark)))
	chromedp.Run(ctx, chromedp.Evaluate[bool](`document.fonts.ready.then(() => true)`, chromedp.EvalAwaitPromise))
	t.waitQuiet(ctx)
	return chromedp.Do(ctx, chromedp.Sleep(300*time.Millisecond))
}

func (t *tab) viewport(sh Shot, h int) chromedp.Action[chromedp.Void] {
	opts := []chromedp.EmulateViewportOption{chromedp.EmulateScale(max(sh.Scale, 1))}
	if sh.W < 768 {
		opts = append(opts, chromedp.EmulateMobile, chromedp.EmulateTouch)
	}
	return chromedp.EmulateViewport(int64(sh.W), int64(h), opts...)
}

// waitQuiet waits until no request is open for 500 ms, at most 5 seconds.
func (t *tab) waitQuiet(ctx context.Context) {
	end := time.Now().Add(5 * time.Second)
	for time.Now().Before(end) && ctx.Err() == nil {
		t.mu.Lock()
		quiet := len(t.inflight) == 0 && time.Since(t.last) > 500*time.Millisecond
		t.mu.Unlock()
		if quiet {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
}

const maxFull = 12000

// full takes the whole page. An app shell keeps the document one screen high and
// scrolls an inner element, so moki makes the viewport taller by its hidden part.
func (t *tab) full(ctx context.Context, sh Shot) ([]byte, error) {
	m, err := chromedp.Run(ctx, chromedp.Evaluate[struct{ Doc, Inner float64 }](fullJS))
	if err != nil {
		return nil, err
	}
	if m.Inner > 0 {
		h := min(sh.H+int(m.Inner), maxFull)
		if err := chromedp.Do(ctx, t.viewport(sh, h), chromedp.Sleep(300*time.Millisecond)); err != nil {
			return nil, err
		}
		return chromedp.Run(ctx, chromedp.CaptureScreenshot())
	}
	h := min(max(m.Doc, float64(sh.H)), maxFull)
	r, err := chromedp.Call(ctx, page.CaptureScreenshot, page.CaptureScreenshotParams{
		CaptureBeyondViewport: ptr(true), FromSurface: ptr(true),
		Clip: &page.Viewport{Width: float64(sh.W), Height: h, Scale: 1},
	})
	return r.Data, err
}

const fullJS = `(() => {
  const doc = (document.scrollingElement || document.documentElement).scrollHeight
  if (doc > innerHeight + 1) return {Doc: doc, Inner: 0}
  let inner = 0
  for (const el of document.querySelectorAll('*')) {
    const oy = getComputedStyle(el).overflowY
    if (oy === 'auto' || oy === 'scroll') inner = Math.max(inner, el.scrollHeight - el.clientHeight)
  }
  return {Doc: doc, Inner: inner}
})()`

func ptr[T any](v T) *T { return &v }
