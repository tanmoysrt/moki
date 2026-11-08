package main

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const shotDesc = `Take screenshots of a project: every combination of versions, pages, sizes and themes, up to 48 shots.
Sizes: desktop 1280x800, laptop 1440x900, tablet 834x1194, phone 390x844, phone-landscape 844x390, or "WxH".
full: true takes the whole page, also when an inner element scrolls. Do not send a large height for that.
More than one shot comes back as one labeled sheet. Each shot also has the URL of its full-size PNG.`

type shotIn struct {
	Slug     string   `json:"slug" jsonschema:"project slug"`
	Versions any      `json:"versions,omitempty" jsonschema:"\"v3\", [\"v2\", \"v3\"] or \"all\". Default: the newest version"`
	Pages    any      `json:"pages,omitempty" jsonschema:"[\"index.html\", \"b.html\"] or \"all\" for every .html file. Default: index.html"`
	Sizes    []string `json:"sizes,omitempty" jsonschema:"size names or WxH. Default: desktop and phone"`
	Themes   string   `json:"themes,omitempty" jsonschema:"light, dark or both. Default: light"`
	Full     bool     `json:"full,omitempty" jsonschema:"take the whole page height"`
	Selector string   `json:"selector,omitempty" jsonschema:"a CSS selector, to take one element"`
	Sheet    *bool    `json:"sheet,omitempty" jsonschema:"put all shots on one labeled image. Default true when there is more than one shot"`
}

const maxShots = 48

var sizeNames = map[string][2]int{
	"desktop": {1280, 800}, "laptop": {1440, 900}, "tablet": {834, 1194},
	"phone": {390, 844}, "phone-landscape": {844, 390},
}

var sizeRe = regexp.MustCompile(`^(\d{2,4})x(\d{2,5})$`)

func (s *Server) screenshot(ctx context.Context, in shotIn) ([]mcp.Content, error) {
	shots, err := s.planShots(in)
	if err != nil {
		return nil, err
	}
	sheet := len(shots) > 1
	if in.Sheet != nil {
		sheet = *in.Sheet
	}
	run, err := s.run(ctx, shots, sheet)
	if err != nil {
		return nil, err
	}
	note(ctx, "%s · %s%s", in.Slug, count(len(shots), "shot"), warnNote(run.Warns))
	text := "Shots:\n- " + strings.Join(run.List, "\n- ") + "\n\n" + warnList(run.Warns)
	return withImages(text, run.Images), nil
}

// planShots makes every combination of versions, pages, sizes and themes.
func (s *Server) planShots(in shotIn) ([]Shot, error) {
	if err := checkSlug(in.Slug); err != nil {
		return nil, err
	}
	p, vers, err := s.store.Project(in.Slug)
	if err != nil {
		return nil, err
	}
	names, err := pickVersions(in.Versions, p.Version, vers)
	if err != nil {
		return nil, err
	}
	sizes, err := parseSizes(in.Sizes)
	if err != nil {
		return nil, err
	}
	themes, err := parseThemes(in.Themes)
	if err != nil {
		return nil, err
	}
	var shots []Shot
	for _, ver := range names {
		pages, err := s.pickPages(in.Slug, ver, in.Pages)
		if err != nil {
			return nil, err
		}
		for _, page := range pages {
			for _, sz := range sizes {
				for _, dark := range themes {
					shots = append(shots, Shot{
						URL: s.pageURL(in.Slug, ver, page), W: sz.w, H: sz.h, Scale: 1, Dark: dark,
						Full: in.Full, Selector: in.Selector, Label: shotLabel(ver, page, sz.name, themeName(dark)),
					})
				}
			}
		}
	}
	if len(shots) > maxShots {
		return nil, fmt.Errorf("this call asks for %d shots, above the limit of %d. Ask for fewer versions, pages, sizes or themes", len(shots), maxShots)
	}
	return shots, nil
}

func pickVersions(v any, newest string, vers []Version) ([]string, error) {
	var all []string
	for _, x := range vers {
		all = append(all, x.Name)
	}
	list, err := stringList(v, "versions")
	if err != nil {
		return nil, err
	}
	switch {
	case len(list) == 0:
		return []string{newest}, nil
	case len(list) == 1 && list[0] == "all":
		return all, nil
	}
	for _, name := range list {
		if !hasFile(all, name) {
			return nil, fmt.Errorf("version %q does not exist. The versions are %s", name, strings.Join(all, ", "))
		}
	}
	return list, nil
}

func (s *Server) pickPages(slug, ver string, v any) ([]string, error) {
	files := s.store.Files(slug, ver)
	var html []string
	for _, f := range files {
		if strings.HasSuffix(f, ".html") {
			html = append(html, f)
		}
	}
	list, err := stringList(v, "pages")
	if err != nil {
		return nil, err
	}
	switch {
	case len(list) == 0:
		list = []string{"index.html"}
	case len(list) == 1 && list[0] == "all":
		return html, nil
	}
	for _, page := range list {
		if !hasFile(files, page) {
			return nil, fmt.Errorf("page %q is not in %s. Its pages are %s", page, ver, strings.Join(html, ", "))
		}
	}
	return list, nil
}

// stringList reads a string or a list of strings from a JSON value.
func stringList(v any, name string) ([]string, error) {
	switch x := v.(type) {
	case nil:
		return nil, nil
	case string:
		if x == "" {
			return nil, nil
		}
		return []string{x}, nil
	case []any:
		var out []string
		for _, e := range x {
			str, ok := e.(string)
			if !ok {
				return nil, fmt.Errorf("%s must be a string or a list of strings", name)
			}
			out = append(out, str)
		}
		return out, nil
	}
	return nil, fmt.Errorf("%s must be a string or a list of strings", name)
}

type size struct {
	name string
	w, h int
}

func parseSizes(names []string) ([]size, error) {
	if len(names) == 0 {
		names = []string{"desktop", "phone"}
	}
	var out []size
	for _, n := range names {
		if wh, ok := sizeNames[n]; ok {
			out = append(out, size{n, wh[0], wh[1]})
			continue
		}
		m := sizeRe.FindStringSubmatch(n)
		if m == nil {
			return nil, fmt.Errorf("size %q is not valid. Use desktop, laptop, tablet, phone, phone-landscape or WxH such as 1280x800", n)
		}
		w, _ := strconv.Atoi(m[1])
		h, _ := strconv.Atoi(m[2])
		if w < 200 || w > 3840 || h < 200 || h > maxFull {
			return nil, fmt.Errorf("size %q is out of range. The width is 200 to 3840, the height 200 to %d", n, maxFull)
		}
		out = append(out, size{n, w, h})
	}
	return out, nil
}

func parseThemes(t string) ([]bool, error) {
	switch t {
	case "", "light":
		return []bool{false}, nil
	case "dark":
		return []bool{true}, nil
	case "both":
		return []bool{false, true}, nil
	}
	return nil, fmt.Errorf(`themes %q is not valid. Use "light", "dark" or "both"`, t)
}

func themeName(dark bool) string {
	if dark {
		return "dark"
	}
	return "light"
}

// shotRun is what a batch of shots gives the agent.
type shotRun struct {
	Images [][]byte // JPEG
	List   []string // label and full-size URL of each shot
	Warns  []string
}

// run takes the shots in one Chrome batch, saves them, and makes a sheet or single images.
func (s *Server) run(ctx context.Context, shots []Shot, sheet bool) (shotRun, error) {
	var out shotRun
	err := s.shots.Batch(ctx, func(b *Batch) error {
		results := b.Take(shots)
		ids := make([]string, len(results))
		for i, r := range results {
			if r.Err != nil {
				out.List = append(out.List, r.Label+": "+r.Err.Error())
				continue
			}
			id, err := s.saveShot(r.PNG)
			if err != nil {
				return err
			}
			ids[i] = id
			out.List = append(out.List, r.Label+": "+s.cfg.URL+"/_shot/"+id)
		}
		out.Warns = groupWarnings(results)
		if sheet {
			parts, err := s.sheet(b, results, ids)
			out.Images = parts
			return err
		}
		for _, r := range results {
			if r.Err == nil {
				parts, err := splitImage(r.PNG, maxSide)
				if err != nil {
					return err
				}
				out.Images = append(out.Images, parts...)
			}
		}
		return nil
	})
	return out, err
}

func withImages(text string, images [][]byte) []mcp.Content {
	out := []mcp.Content{&mcp.TextContent{Text: text}}
	for _, img := range images {
		out = append(out, &mcp.ImageContent{Data: img, MIMEType: "image/jpeg"})
	}
	return out
}
