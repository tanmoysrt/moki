package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Options are the ideas of a round: for each part, such as a screen, a video or a deck
// cover, a list of options. Each option is a page of the version. The agent sends them
// as the options field of upload, or as options.json in the version. The viewer shows a
// grid of the options, opens one at full size, and keeps the ones the user picks. moki
// refuses an upload whose options do not fit this shape.

type Options struct {
	Parts []Part `json:"parts" jsonschema:"one part for each question of the round, such as a screen, a video or a deck cover"`
}

type Part struct {
	Title   string   `json:"title" jsonschema:"the name of the part, unique in the version, for example Plans page"`
	Size    string   `json:"size,omitempty" jsonschema:"WxH of the pages in the grid. 1280x560 crops a screen to its top. A deck or a video gives its own size. Default 1280x800"`
	Options []Option `json:"options" jsonschema:"the options of the part: 5 or more in the first round"`
}

type Option struct {
	ID     string  `json:"id" jsonschema:"a short id, unique in the part: A, B, C"`
	Name   string  `json:"name" jsonschema:"a short name, for example Comparison table"`
	Note   string  `json:"note,omitempty" jsonschema:"one line on what is different"`
	Page   string  `json:"page" jsonschema:"the page of the option in this version, with a query when it needs one: web.html?v=b"`
	Rec    bool    `json:"rec,omitempty" jsonschema:"true for the option you recommend"`
	States []State `json:"states,omitempty" jsonschema:"more pages of the option, such as empty, error or as a visitor"`
}

type State struct {
	Name string `json:"name" jsonschema:"a short name, for example Empty"`
	Page string `json:"page" jsonschema:"a page in this version, for example web.html?v=b&s=empty"`
}

const optionsFile = "options.json"

// readOptions reads the options of a version folder. It is nil when there are none.
func readOptions(dir string) *Options {
	data, err := os.ReadFile(filepath.Join(dir, optionsFile))
	if err != nil {
		return nil
	}
	o, err := parseOptions(data)
	if err != nil {
		return nil
	}
	return o
}

// parseOptions reads options.json. A field that moki does not know is an error.
func parseOptions(data []byte) (*Options, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var o Options
	if err := dec.Decode(&o); err != nil {
		return nil, fmt.Errorf("%s: %s", optionsFile, strings.TrimPrefix(err.Error(), "json: "))
	}
	if dec.More() {
		return nil, fmt.Errorf("%s: more than one JSON value", optionsFile)
	}
	return &o, nil
}

// problems lists what is wrong with the options of the version in dir. Each line says
// where: options.json parts[1].options[2]: id "B" is used twice in "Intro video".
func (o *Options) problems(dir string) []string {
	var out []string
	add := func(at, format string, a ...any) {
		out = append(out, fmt.Sprintf("%s %s: %s", optionsFile, at, fmt.Sprintf(format, a...)))
	}
	if len(o.Parts) == 0 {
		add("parts", "there are no parts. Give one part for each question of the round")
	}
	titles := map[string]bool{}
	for i, p := range o.Parts {
		at := fmt.Sprintf("parts[%d]", i)
		title := strings.TrimSpace(p.Title)
		switch {
		case title == "":
			add(at, "no title")
		case len([]rune(title)) > 80:
			add(at, "the title is longer than 80 characters")
		case titles[title]:
			add(at, "the title %q is used twice", title)
		}
		titles[title] = true
		if p.Size != "" && !sizeRe.MatchString(p.Size) {
			add(at, "size %q is not WxH, for example 1280x560", p.Size)
		}
		if len(p.Options) == 0 {
			add(at, "no options")
		}
		if len(p.Options) > 40 {
			add(at, "%d options, above the limit of 40", len(p.Options))
		}
		ids := map[string]bool{}
		for j, x := range p.Options {
			at := fmt.Sprintf("%s.options[%d]", at, j)
			id := strings.TrimSpace(x.ID)
			switch {
			case id == "":
				add(at, "no id. Use A, B, C")
			case len([]rune(id)) > 8:
				add(at, "the id %q is longer than 8 characters. Use A, B, C", id)
			case ids[id]:
				add(at, "the id %q is used twice in %q", id, title)
			}
			ids[id] = true
			if n := len([]rune(strings.TrimSpace(x.Name))); n == 0 || n > 80 {
				add(at, "a name of 1 to 80 characters is needed")
			}
			if len([]rune(x.Note)) > 300 {
				add(at, "the note is longer than 300 characters. One line is enough")
			}
			if msg := pageProblem(dir, x.Page); msg != "" {
				add(at, "%s", msg)
			}
			for k, st := range x.States {
				at := fmt.Sprintf("%s.states[%d]", at, k)
				if n := len([]rune(strings.TrimSpace(st.Name))); n == 0 || n > 40 {
					add(at, "a name of 1 to 40 characters is needed")
				}
				if msg := pageProblem(dir, st.Page); msg != "" {
					add(at, "%s", msg)
				}
			}
		}
	}
	return out
}

// pageProblem says why a page of an option cannot open, or "".
func pageProblem(dir, page string) string {
	if strings.TrimSpace(page) == "" {
		return "no page. Use a page of the version, for example web.html?v=b"
	}
	file := pageFile(page)
	if !strings.HasSuffix(file, ".html") {
		return fmt.Sprintf("page %q is not an .html file", page)
	}
	rel, err := relPath(file)
	fi, statErr := os.Stat(filepath.Join(dir, filepath.FromSlash(rel)))
	if err != nil || statErr != nil || !fi.Mode().IsRegular() {
		return fmt.Sprintf("page %q is not in the version", page)
	}
	return ""
}

// pageFile is the file of a page without its query and hash: web.html?v=b is web.html.
func pageFile(page string) string {
	return strings.SplitN(strings.SplitN(page, "#", 2)[0], "?", 2)[0]
}

// warnings are small things that do not stop the upload.
func (o *Options) warnings(firstRound bool) []string {
	var out []string
	for _, p := range o.Parts {
		if firstRound && len(p.Options) < 5 {
			out = append(out, fmt.Sprintf("%s: %q has %s. Give 5 or more in the first round, different in idea",
				optionsFile, p.Title, count(len(p.Options), "option")))
		}
		for _, x := range p.Options {
			if strings.TrimSpace(x.Note) == "" {
				out = append(out, fmt.Sprintf("%s: %s %s has no note. Say in one line what is different", optionsFile, p.Title, x.ID))
			}
		}
	}
	return out
}

// find is the option of a page, and the state when the page is a state: "Plans page B",
// or "Plans page B, Empty". It is "" for a page that is not an option.
func (o *Options) find(page string) string {
	for _, p := range o.Parts {
		for _, x := range p.Options {
			if x.Page == page {
				return p.Title + " " + x.ID
			}
			for _, st := range x.States {
				if st.Page == page {
					return p.Title + " " + x.ID + ", " + st.Name
				}
			}
		}
	}
	return ""
}

// pages are the pages of all options, for screenshot pages: "options".
func (o *Options) pages() []string {
	var out []string
	for _, p := range o.Parts {
		for _, x := range p.Options {
			out = append(out, x.Page)
		}
	}
	return out
}
