package main

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

// pageInfo is window.moki of a page. The kit scripts set it; other pages are screens.
type pageInfo struct {
	Kind     string
	Size     string
	Slides   int
	Duration float64
	Fps      int
	W, H     int
}

// renderer makes one export file in one Chrome tab.
type renderer struct {
	t    *tab
	ctx  context.Context
	req  exportReq
	url  string
	info pageInfo
	prog func(float64)
}

// render starts Chrome, opens the page and makes the file.
func (s *Server) render(ctx context.Context, req exportReq, prog func(float64)) ([]byte, error) {
	var out []byte
	err := s.shots.Batch(ctx, func(b *Batch) error {
		t, err := b.newTab()
		if err != nil {
			return err
		}
		defer t.cancel()
		r := &renderer{t: t, ctx: t.ctx, req: req, url: s.pageURL(req.Slug, req.Ver, req.Page), prog: prog}
		if err := r.open(); err != nil {
			return err
		}
		out, err = r.make()
		return err
	})
	return out, err
}

func (r *renderer) make() ([]byte, error) {
	kind := r.info.Kind
	switch r.req.Format {
	case "png":
		return r.png()
	case "pdf":
		if kind == "deck" {
			return r.print(true, 0)
		}
		return r.pagePDF()
	case "zip":
		if kind != "deck" {
			return nil, fmt.Errorf("zip is for decks: the page does not load /kit/deck.js")
		}
		return r.slides()
	case "mp4":
		if kind != "video" {
			return nil, fmt.Errorf("mp4 is for videos: the page does not load /kit/video.js")
		}
		return r.mp4()
	case "svg":
		return r.svg()
	}
	return nil, fmt.Errorf("unknown format %q", r.req.Format)
}

// open loads the page at 1280x800 and reads window.moki. A deck or a video opens
// again at its own size, so one CSS pixel of the slide is one pixel of the file.
func (r *renderer) open() error {
	if err := r.load(r.url, 1280, 800); err != nil {
		return err
	}
	info, err := chromedp.Run(r.ctx, chromedp.Evaluate[pageInfo](infoJS))
	if err != nil {
		return err
	}
	info.W, info.H = 1280, 800
	if w, h, ok := parseSize(info.Size); ok {
		info.W, info.H = w, h
	}
	r.info = info
	switch info.Kind {
	case "video":
		t := 0.0
		if r.req.Format == "png" {
			t = r.req.Time
		}
		return r.load(addQuery(r.url, "t", strconv.FormatFloat(t, 'f', 3, 64)), info.W, info.H)
	case "deck":
		return r.load(r.url, info.W, info.H)
	}
	return nil
}

func (r *renderer) load(u string, w, h int) error {
	sh := Shot{URL: u, W: w, H: h, Scale: r.scale(), Dark: r.req.Dark}
	ctx, cancel := context.WithTimeout(r.ctx, 30*time.Second)
	defer cancel()
	if err := r.t.open(ctx, sh); err != nil {
		return err
	}
	_, err := chromedp.Run(r.ctx, chromedp.Evaluate[bool](`document.documentElement.classList.add('export'), true`))
	return err
}

// scale is 2 for pictures, so they stay sharp on a retina screen.
func (r *renderer) scale() float64 {
	if r.req.Format == "png" || r.req.Format == "zip" {
		return 2
	}
	return 1
}

func (r *renderer) png() ([]byte, error) {
	switch r.info.Kind {
	case "deck":
		return r.slide(max(r.req.Slide, 1))
	case "video":
		return chromedp.Run(r.ctx, chromedp.CaptureScreenshot())
	}
	return r.t.full(r.ctx, Shot{W: 1280, H: 800, Scale: 2})
}

// slide shows slide n (from 1) and takes it.
func (r *renderer) slide(n int) ([]byte, error) {
	_, err := chromedp.Run(r.ctx, chromedp.Evaluate[bool](fmt.Sprintf(`moki.show(%d), true`, n)))
	if err != nil {
		return nil, err
	}
	if err := chromedp.Do(r.ctx, chromedp.Sleep(80*time.Millisecond)); err != nil {
		return nil, err
	}
	return chromedp.Run(r.ctx, chromedp.CaptureScreenshot())
}

// pagePDF prints a page as one long PDF page that looks like the screen, not like print.
func (r *renderer) pagePDF() ([]byte, error) {
	m, err := chromedp.Run(r.ctx, chromedp.Evaluate[struct{ Doc, Inner float64 }](fullJS))
	if err != nil {
		return nil, err
	}
	h := min(max(m.Doc, 800)+m.Inner, maxFull)
	theme := map[bool]string{true: "dark", false: "light"}[r.req.Dark]
	_, err = chromedp.Call(r.ctx, emulation.SetEmulatedMedia, emulation.SetEmulatedMediaParams{Media: "screen",
		Features: []*emulation.MediaFeature{{Name: "prefers-color-scheme", Value: theme}}})
	if err != nil {
		return nil, err
	}
	return r.print(false, h)
}

// print makes a PDF. A deck brings its page size in CSS; a screen is 1280 wide and h high.
func (r *renderer) print(css bool, h float64) ([]byte, error) {
	zero := 0.0
	p := page.PrintToPDFParams{PrintBackground: ptr(true), PreferCSSPageSize: ptr(css),
		MarginTop: &zero, MarginBottom: &zero, MarginLeft: &zero, MarginRight: &zero}
	if !css {
		p.PaperWidth, p.PaperHeight = 1280.0/96, h/96
	}
	res, err := chromedp.Call(r.ctx, page.PrintToPDF, p)
	return res.Data, err
}

// slides takes each slide as a PNG, in a zip: slide-01.png, slide-02.png and so on.
func (r *renderer) slides() ([]byte, error) {
	var buf bytes.Buffer
	z := zip.NewWriter(&buf)
	for n := 1; n <= r.info.Slides; n++ {
		png, err := r.slide(n)
		if err != nil {
			return nil, err
		}
		f, err := z.Create(fmt.Sprintf("slide-%02d.png", n))
		if err != nil {
			return nil, err
		}
		if _, err := f.Write(png); err != nil {
			return nil, err
		}
		r.prog(float64(n) / float64(r.info.Slides))
	}
	if err := z.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// svg returns the biggest svg on the page as a file. It keeps the color that
// currentColor means on the page, and the hand-drawn font of sketch.js.
func (r *renderer) svg() ([]byte, error) {
	s, err := chromedp.Run(r.ctx, chromedp.Evaluate[string](svgJS))
	if err != nil {
		return nil, err
	}
	if s == "" {
		return nil, fmt.Errorf("the page has no svg")
	}
	return []byte(s), nil
}

func parseSize(s string) (int, int, bool) {
	a, b, ok := strings.Cut(s, "x")
	w, err1 := strconv.Atoi(a)
	h, err2 := strconv.Atoi(b)
	ok = ok && err1 == nil && err2 == nil && w >= 100 && h >= 100 && w <= 4000 && h <= 4000
	return w, h, ok
}

func addQuery(u, k, v string) string {
	sep := map[bool]string{true: "&", false: "?"}[strings.Contains(u, "?")]
	return u + sep + k + "=" + v
}

const infoJS = `(() => {
  const m = window.moki || {}
  return {Kind: m.kind || 'screen', Size: m.size || '', Slides: m.slides || 0, Duration: m.duration || 0, Fps: m.fps || 0}
})()`

const svgJS = `(() => {
  const all = [...document.querySelectorAll('svg')].map((s) => [s, s.getBoundingClientRect()])
  all.sort((a, b) => b[1].width * b[1].height - a[1].width * a[1].height)
  if (!all.length || all[0][1].width < 50) return ''
  const [svg, box] = all[0]
  const c = svg.cloneNode(true)
  c.setAttribute('xmlns', 'http://www.w3.org/2000/svg')
  c.setAttribute('width', Math.round(box.width))
  c.setAttribute('height', Math.round(box.height))
  c.style.color = getComputedStyle(svg).color
  if (c.innerHTML.includes('Caveat')) {
    c.insertAdjacentHTML('afterbegin', '<style>@import url(https://fonts.googleapis.com/css2?family=Caveat:wght@600&amp;display=swap);</style>')
  }
  return '<?xml version="1.0" encoding="UTF-8"?>\n' + new XMLSerializer().serializeToString(c)
})()`
