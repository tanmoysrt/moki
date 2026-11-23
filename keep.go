package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"
)

// A version with options shows them as a grid in the viewer. The user keeps the ones
// worth another round, and the viewer sends it here. The agent reads the kept options
// in get_project (optionsText), and builds the next version from them.

const maxKept = 100 // on one version

func (s *Server) apiKeep(w http.ResponseWriter, r *http.Request) {
	if !sameOrigin(w, r) {
		return
	}
	slug := r.PathValue("slug")
	var in struct {
		Version string `json:"version"`
		KeptOption
		On bool `json:"on"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(&in); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	k := in.KeptOption
	k.Part, k.Option, k.Name = cut(strings.TrimSpace(k.Part), 80), cut(strings.TrimSpace(k.Option), 20), cut(strings.TrimSpace(k.Name), 80)
	if checkVersion(in.Version) != nil || !isDir(s.store.VersionDir(slug, in.Version)) || k.Option == "" {
		http.Error(w, "keep needs a version and an option", http.StatusBadRequest)
		return
	}
	what := map[bool]string{true: "kept", false: "dropped"}[in.On] + " " + strings.TrimSpace(k.Part+" "+k.Option) + " on " + in.Version
	s.changeNotes(w, r, slug, what, func(p *ProjectNotes) { p.keep(in.Version, k, in.On) })
}

func (p *ProjectNotes) keep(ver string, k KeptOption, on bool) {
	list := slices.DeleteFunc(p.Kept[ver], func(x KeptOption) bool { return x.Part == k.Part && x.Option == k.Option })
	if on && len(list) < maxKept {
		list = append(list, k)
	}
	if p.Kept == nil {
		p.Kept = map[string][]KeptOption{}
	}
	p.Kept[ver] = list
	if len(list) == 0 {
		delete(p.Kept, ver)
	}
}

// optionsText lists the options of the newest version, when it has options, and marks
// the kept ones:
//
//	Options on v1. The user kept 2. Build the next round from them and drop the rest:
//	  Plans page: A Three cards, B Comparison table (kept), C Rows (kept)
func (s *Server) optionsText(slug string, kept map[string][]KeptOption) string {
	vers := s.store.Versions(slug)
	if len(vers) == 0 || !vers[len(vers)-1].Options {
		return ""
	}
	ver := vers[len(vers)-1].Name
	o := readOptions(s.store.VersionDir(slug, ver))
	if o == nil {
		return ""
	}
	isKept := map[string]bool{}
	for _, k := range kept[ver] {
		isKept[k.Part+"\n"+k.Option] = true
	}
	var b strings.Builder
	n := 0
	for _, p := range o.Parts {
		var opts []string
		for _, x := range p.Options {
			line := x.ID + " " + x.Name
			if isKept[p.Title+"\n"+x.ID] {
				line += " (kept)"
				n++
			}
			opts = append(opts, line)
		}
		b.WriteString("  " + p.Title + ": " + strings.Join(opts, ", ") + "\n")
	}
	head := fmt.Sprintf("Options on %s. The user kept none yet: they keep options in the viewer.\n", ver)
	if n > 0 {
		head = fmt.Sprintf("Options on %s. The user kept %d. Build the next round from them and drop the rest:\n", ver, n)
	}
	return head + b.String()
}
