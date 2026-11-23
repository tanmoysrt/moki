package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// moki keeps mockups: web pages and what they load. An upload with another kind of
// file, a file above 10 MB, or options that do not fit is refused, and nothing is saved.

const maxFile = 10 << 20

var webExts = map[string]bool{
	".html": true, ".htm": true, ".css": true, ".js": true, ".mjs": true, ".json": true, ".map": true,
	".svg": true, ".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".webp": true, ".avif": true, ".ico": true,
	".woff": true, ".woff2": true, ".ttf": true, ".otf": true,
	".mp4": true, ".webm": true, ".mp3": true, ".wav": true, ".ogg": true, ".m4a": true,
	".csv": true, ".xml": true, ".wasm": true, ".glb": true, ".gltf": true, ".pdf": true,
}

// refused is the error of an upload that moki does not save.
func refused(files, opts []string) error {
	var b strings.Builder
	b.WriteString("refused, nothing was saved. Fix these and upload again:\n")
	for _, p := range append(files, opts...) {
		b.WriteString("  " + p + "\n")
	}
	if len(files) > 0 {
		b.WriteString("moki keeps pages and what they load: html, css, js, json, images, fonts, audio and video, up to 10 MB each.\n")
	}
	if len(opts) > 0 {
		b.WriteString("The options format is in the options field of upload, and in the guide, section Options.\n")
	}
	return errors.New(strings.TrimSpace(b.String()))
}

// incomingProblems checks the files of an upload before moki writes them.
func incomingProblems(in uploadIn) []string {
	var out []string
	for _, f := range in.Files {
		size := len(f.Content)
		if f.Base64 {
			size = size * 3 / 4
		}
		if msg := fileProblem(f.Path, int64(size)); msg != "" {
			out = append(out, msg)
		}
	}
	if in.Dir == "" {
		return out
	}
	dir, err := absPath(in.Dir)
	if err != nil || !isDir(dir) {
		return out // fill says what is wrong with dir
	}
	filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(dir, p)
		if rel != "." && isHidden(filepath.ToSlash(rel)) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		if fi, err := d.Info(); err == nil {
			if msg := fileProblem(filepath.ToSlash(rel), fi.Size()); msg != "" {
				out = append(out, msg)
			}
		}
		return nil
	})
	return out
}

func fileProblem(name string, size int64) string {
	switch {
	case isHidden(name):
		return ""
	case !webExts[strings.ToLower(filepath.Ext(name))]:
		return name + ": not a file of a web page"
	case size > maxFile:
		return fmt.Sprintf("%s: %d MB, above the limit of 10 MB", name, size>>20)
	}
	return ""
}

// optionProblems checks the options of the version that moki is about to save.
func optionProblems(dir string) []string {
	data, err := os.ReadFile(filepath.Join(dir, optionsFile))
	if err != nil {
		return nil
	}
	o, err := parseOptions(data)
	if err != nil {
		return []string{err.Error()}
	}
	return o.problems(dir)
}

// remoteScriptRe finds a script from another site, which breaks offline and in exports.
var remoteScriptRe = regexp.MustCompile(`(?i)<script[^>]+src=["']?(https?:)?//`)

func hasRemoteScript(path string) bool {
	if filepath.Ext(path) != ".html" {
		return false
	}
	b, _ := os.ReadFile(path)
	return remoteScriptRe.Match(b)
}

// writeOptions writes the options field of upload as options.json.
func writeOptions(dir string, o *Options) error {
	if o == nil {
		return nil
	}
	data, err := json.MarshalIndent(o, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, optionsFile), data, 0o644)
}

// optionWarnings: the first round of options is the first version that has them.
func (s *Server) optionWarnings(slug, ver string) []string {
	o := readOptions(s.store.VersionDir(slug, ver))
	if o == nil {
		return nil
	}
	first := true
	for _, v := range s.store.Versions(slug) {
		if versionNum(v.Name) < versionNum(ver) && v.Options {
			first = false
		}
	}
	return o.warnings(first)
}
