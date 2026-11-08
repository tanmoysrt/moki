package main

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"html/template"
	"image"
	"image/draw"
	"image/jpeg"
	"image/png"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// A sheet puts many shots on one labeled image. moki has no image library for text,
// so it writes the shots to .cache/shots, opens /_sheet that shows them in a grid,
// and takes one shot of that page in the same Chrome batch.

const (
	sheetWidth = 1600 // the widest sheet page
	cellWidth  = 900  // a wider shot is scaled down to this width, so a phone stays 1:1
	partHeight = 1100 // a tall shot is cut into columns of this height on the sheet
	partGap    = 8
	maxSide    = 1600 // images for the agent are cut so no side is longer
)

var shotIDRe = regexp.MustCompile(`^[A-Z2-7]{26}\.png$`)

type sheetItem struct {
	Label, Src, Err string
	W, H            int // shown size in px
	Parts           []part
}

// part is one column of a tall shot: the offset of the image and the column height.
type part struct{ Top, H int }

var sheets sync.Map // sheet id -> []sheetItem

// saveShot writes a PNG to .cache/shots and returns its id.
func (s *Server) saveShot(b []byte) (string, error) {
	dir := filepath.Join(s.cfg.Dir, ".cache", "shots")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	cleanOld(dir, 24*time.Hour)
	id := rand.Text() + ".png"
	return id, os.WriteFile(filepath.Join(dir, id), b, 0o644)
}

// sheet takes one shot of /_sheet with the results and returns it in parts for the agent.
func (s *Server) sheet(b *Batch, results []Result, ids []string) ([][]byte, error) {
	var items []sheetItem
	row := 0
	for i, r := range results {
		it := sheetItem{Label: r.Label, W: 380}
		if r.Err != nil {
			it.Err = r.Err.Error()
		} else {
			it.Src = "/_shot/" + ids[i]
			it.W, it.H = sheetSize(r)
			for y := 0; y < it.H; y += partHeight {
				it.Parts = append(it.Parts, part{-y, min(partHeight, it.H-y)})
			}
		}
		row += max(1, len(it.Parts))*(it.W+partGap) + 16
		items = append(items, it)
	}
	id := rand.Text()
	sheets.Store(id, items)
	defer sheets.Delete(id)
	w := min(sheetWidth, row+32) // one row needs no more width
	res := b.Take([]Shot{{URL: s.shots.origin + "/_sheet?id=" + id, W: w, H: 900, Full: true, Label: "sheet"}})
	if res[0].Err != nil {
		return nil, res[0].Err
	}
	return splitImage(res[0].PNG, maxSide)
}

// sheetSize scales a shot to at most cellWidth, and further when its columns
// do not fit the sheet side by side.
func sheetSize(r Result) (int, int) {
	cfg, err := png.DecodeConfig(bytes.NewReader(r.PNG))
	if err != nil {
		return 0, 0
	}
	scale := max(r.Scale, 1)
	w0, h0 := float64(cfg.Width)/scale, float64(cfg.Height)/scale
	f := min(1, cellWidth/w0)
	for range 5 {
		n := math.Ceil(h0 * f / partHeight)
		need := n*w0*f + (n-1)*partGap
		if need <= sheetWidth-32 {
			break
		}
		f *= (sheetWidth - 32) / need
	}
	return int(w0 * f), int(h0 * f)
}

func (s *Server) serveSheet(w http.ResponseWriter, r *http.Request) {
	items, ok := sheets.Load(r.URL.Query().Get("id"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	sheetTmpl.Execute(w, items)
}

var sheetTmpl = template.Must(template.New("sheet").Parse(`<!doctype html>
<html>
<head>
<meta charset="utf-8">
<style>
  html, body { margin: 0; background: #e4e4e7; }
  body { padding: 16px; box-sizing: border-box; font: 600 14px/20px system-ui, sans-serif; color: #18181b; }
  .g { display: flex; flex-wrap: wrap; gap: 20px 16px; align-items: flex-start; }
  .s { display: flex; flex-direction: column; gap: 6px; }
  .parts { display: flex; gap: 8px; align-items: flex-start; }
  .p { overflow: hidden; background: #fff; box-shadow: 0 0 0 1px rgba(0, 0, 0, 0.15); }
  .p img { display: block; }
  .err { width: 380px; padding: 12px; background: #fee2e2; color: #991b1b; font-weight: 400; }
</style>
</head>
<body>
<div class="g">
  {{range .}}
  <div class="s">
    <div>{{.Label}}</div>
    {{if .Err}}
    <div class="err">{{.Err}}</div>
    {{else}}
    <div class="parts">
      {{$it := .}}
      {{range .Parts}}
      <div class="p" style="width:{{$it.W}}px;height:{{.H}}px">
        <img src="{{$it.Src}}" style="width:{{$it.W}}px;height:{{$it.H}}px;margin-top:{{.Top}}px">
      </div>
      {{end}}
    </div>
    {{end}}
  </div>
  {{end}}
</div>
</body>
</html>`))

// splitImage cuts a PNG into JPEG parts of the same height, at most maxH px each.
// A vision model shrinks larger images until text cannot be read.
func splitImage(b []byte, maxH int) ([][]byte, error) {
	img, err := png.Decode(bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	bounds := img.Bounds()
	n := (bounds.Dy() + maxH - 1) / maxH
	step := (bounds.Dy() + n - 1) / n
	var parts [][]byte
	for y := bounds.Min.Y; y < bounds.Max.Y; y += step {
		r := image.Rect(bounds.Min.X, y, bounds.Max.X, min(y+step, bounds.Max.Y))
		part := image.NewRGBA(image.Rect(0, 0, r.Dx(), r.Dy()))
		draw.Draw(part, part.Bounds(), img, r.Min, draw.Src)
		var buf bytes.Buffer
		if err := jpeg.Encode(&buf, part, &jpeg.Options{Quality: 85}); err != nil {
			return nil, err
		}
		parts = append(parts, buf.Bytes())
	}
	return parts, nil
}

func cleanOld(dir string, age time.Duration) {
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if info, err := e.Info(); err == nil && time.Since(info.ModTime()) > age {
			os.Remove(filepath.Join(dir, e.Name()))
		}
	}
}

// shotLabel names a shot on the sheet, for example "v3 · index.html · phone · dark".
func shotLabel(parts ...string) string {
	var keep []string
	for _, p := range parts {
		if p != "" {
			keep = append(keep, p)
		}
	}
	return strings.Join(keep, " · ")
}

// warnText lists what went wrong on the page of one shot.
// A console line about a missing file is left out, because the file is listed.
func warnText(r Result) []string {
	var out []string
	for _, m := range r.Missing {
		out = append(out, "missing file "+m)
	}
	for _, c := range r.Console {
		if !mentionsAny(c, r.Missing) {
			out = append(out, "console: "+firstLine(c))
		}
	}
	if r.SideScroll && r.W <= 400 {
		out = append(out, fmt.Sprintf("the page scrolls sideways at %d px", r.W))
	}
	return out
}

// groupWarnings puts the shots that have the same warning on one line.
func groupWarnings(results []Result) []string {
	var order []string
	labels := map[string][]string{}
	for _, r := range results {
		for _, w := range warnText(r) {
			if labels[w] == nil {
				order = append(order, w)
			}
			labels[w] = append(labels[w], r.Label)
		}
	}
	var out []string
	for _, w := range order {
		l := labels[w]
		if len(l) == len(results) && len(results) > 1 {
			out = append(out, "all shots: "+w)
		} else {
			out = append(out, strings.Join(l, ", ")+": "+w)
		}
	}
	return out
}

func mentionsAny(s string, missing []string) bool {
	for _, m := range missing {
		path, _, _ := strings.Cut(m, " ")
		if strings.Contains(s, path) {
			return true
		}
	}
	return false
}

func firstLine(s string) string {
	line, _, _ := strings.Cut(s, "\n")
	if len(line) > 300 {
		line = line[:300] + "..."
	}
	return line
}
