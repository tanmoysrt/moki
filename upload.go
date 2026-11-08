package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const uploadDesc = `Save a version of a project. Without version, moki makes the next version.
Follow the design skill (moki://design-skill). Read it in this chat before your first upload.
Send files, or dir when you run on the moki machine. base: "v3" copies v3 first, so you send only the changed files; delete removes paths from it.
version: "v3" replaces v3 for a small fix: then base defaults to v3.
moki returns the viewer URL, warnings and a sheet of index.html at desktop and phone size. Fix the warnings with version.`

type fileIn struct {
	Path    string `json:"path" jsonschema:"path in the version, for example index.html or img/logo.png"`
	Content string `json:"content" jsonschema:"the text of the file, or base64 when base64 is true"`
	Base64  bool   `json:"base64,omitempty" jsonschema:"true for fonts, images and other binary files"`
}

type uploadIn struct {
	Slug    string   `json:"slug" jsonschema:"project slug: lowercase letters, digits and -, for example team-page"`
	Files   []fileIn `json:"files,omitempty" jsonschema:"the files to write"`
	Dir     string   `json:"dir,omitempty" jsonschema:"absolute folder on the moki machine to copy as the version. Symlinks and .git are skipped"`
	Base    string   `json:"base,omitempty" jsonschema:"a version to copy first, for example v3"`
	Delete  []string `json:"delete,omitempty" jsonschema:"paths to remove after the copy of base"`
	Version string   `json:"version,omitempty" jsonschema:"an existing version to replace, for a small fix"`
	Check   *bool    `json:"check,omitempty" jsonschema:"take a desktop and phone sheet of index.html. Default true"`
}

var uploadMu sync.Mutex

func (s *Server) upload(ctx context.Context, in uploadIn) ([]mcp.Content, error) {
	if err := checkSlug(in.Slug); err != nil {
		return nil, err
	}
	if len(in.Files) == 0 && in.Dir == "" && in.Base == "" && in.Version == "" {
		return nil, fmt.Errorf("nothing to save. Send files, dir or base")
	}
	uploadMu.Lock()
	ver, err := s.write(in)
	uploadMu.Unlock()
	if err != nil {
		return nil, err
	}
	s.hub.Changed(s.cfg.Dir, in.Slug, ver)

	files := s.store.Files(in.Slug, ver)
	warns := s.pageWarnings(in.Slug, ver, files)
	if strings.TrimSpace(s.store.ReadMemory(in.Slug)) == "" {
		warns = append(warns, "the project has no memory yet. Call write_memory with the brief and the decisions of this round")
	}
	var images [][]byte
	if (in.Check == nil || *in.Check) && hasFile(files, "index.html") {
		run, err := s.run(ctx, s.checkShots(in.Slug, ver), true)
		if err != nil {
			warns = append(warns, "no screenshot check: "+err.Error())
		}
		warns = append(warns, run.Warns...)
		images = run.Images
	}
	note(ctx, "%s %s · %s%s", in.Slug, ver, count(len(files), "file"), warnNote(warns))
	text := fmt.Sprintf("Saved %s %s with %s.\nViewer: %s\n\n%s",
		in.Slug, ver, count(len(files), "file"), s.viewerURL(in.Slug, ver), warnList(warns))
	return withImages(text, images), nil
}

func (s *Server) checkShots(slug, ver string) []Shot {
	url := s.pageURL(slug, ver, "index.html")
	return []Shot{
		{URL: url, W: 1280, H: 800, Scale: 1, Label: shotLabel(ver, "desktop")},
		{URL: url, W: 390, H: 844, Scale: 1, Label: shotLabel(ver, "phone")},
	}
}

// write builds the version in a temporary folder and renames it into place,
// so the UI never sees half a version.
func (s *Server) write(in uploadIn) (string, error) {
	ver, err := s.targetVersion(&in)
	if err != nil {
		return "", err
	}
	proj := filepath.Join(s.cfg.Dir, in.Slug)
	if err := os.MkdirAll(proj, 0o755); err != nil {
		return "", err
	}
	tmp := filepath.Join(proj, ".tmp-"+rand.Text()[:10])
	defer os.RemoveAll(tmp)
	if err := os.Mkdir(tmp, 0o755); err != nil {
		return "", err
	}
	if err := s.fill(tmp, in); err != nil {
		return "", err
	}
	final := s.store.VersionDir(in.Slug, ver)
	if _, err := os.Stat(final); err == nil {
		old := tmp + "-old"
		if err := os.Rename(final, old); err != nil {
			return "", err
		}
		defer os.RemoveAll(old)
	}
	if err := os.Rename(tmp, final); err != nil {
		return "", err
	}
	now := time.Now()
	os.Chtimes(final, now, now)
	return ver, nil
}

// targetVersion picks the version to write and checks base.
func (s *Server) targetVersion(in *uploadIn) (string, error) {
	next := s.store.NextVersion(in.Slug)
	ver := next
	if in.Version != "" {
		if err := checkVersion(in.Version); err != nil {
			return "", err
		}
		if !isDir(s.store.VersionDir(in.Slug, in.Version)) && in.Version != next {
			return "", fmt.Errorf("version %s of %q does not exist. Leave version out to make %s", in.Version, in.Slug, next)
		}
		ver = in.Version
		if in.Base == "" && isDir(s.store.VersionDir(in.Slug, ver)) {
			in.Base = ver
		}
	}
	if in.Base != "" {
		if err := checkVersion(in.Base); err != nil {
			return "", err
		}
		if !isDir(s.store.VersionDir(in.Slug, in.Base)) {
			return "", fmt.Errorf("base %s of %q does not exist. Call get_project to see the versions", in.Base, in.Slug)
		}
	}
	return ver, nil
}

// fill writes base, then dir, then files, then removes delete.
func (s *Server) fill(tmp string, in uploadIn) error {
	if in.Base != "" {
		if _, err := copyTree(s.store.VersionDir(in.Slug, in.Base), tmp); err != nil {
			return err
		}
	}
	if in.Dir != "" {
		dir, err := absPath(in.Dir)
		if err != nil {
			return err
		}
		if !isDir(dir) {
			return fmt.Errorf("dir %q is not a folder on the moki machine. Send files instead when you run on another machine", in.Dir)
		}
		if _, err := copyTree(dir, tmp); err != nil {
			return err
		}
	}
	root, err := os.OpenRoot(tmp)
	if err != nil {
		return err
	}
	defer root.Close()
	for _, f := range in.Files {
		if err := writeFile(root, f); err != nil {
			return err
		}
	}
	for _, p := range in.Delete {
		rel, err := relPath(p)
		if err != nil {
			return err
		}
		if err := root.RemoveAll(rel); err != nil {
			return fmt.Errorf("could not delete %s: %v", p, err)
		}
	}
	return nil
}

func writeFile(root *os.Root, f fileIn) error {
	rel, err := relPath(f.Path)
	if err != nil {
		return err
	}
	data := []byte(f.Content)
	if f.Base64 {
		if data, err = base64.StdEncoding.DecodeString(strings.TrimSpace(f.Content)); err != nil {
			return fmt.Errorf("%s: content is not valid base64: %v", f.Path, err)
		}
	}
	if d := path.Dir(rel); d != "." {
		if err := root.MkdirAll(d, 0o755); err != nil {
			return fmt.Errorf("%s: %v", f.Path, err)
		}
	}
	root.Remove(rel) // a file copied from base may be read-only
	if err := root.WriteFile(rel, data, 0o644); err != nil {
		return fmt.Errorf("%s: %v", f.Path, err)
	}
	return nil
}

// relPath checks that a path stays inside the version folder.
func relPath(p string) (string, error) {
	rel := path.Clean(strings.ReplaceAll(p, "\\", "/"))
	if p == "" || path.IsAbs(rel) || rel == "." || rel == ".." || strings.HasPrefix(rel, "../") {
		return "", fmt.Errorf("path %q leaves the version folder. Use a relative path such as index.html or css/app.css", p)
	}
	return rel, nil
}

func (s *Server) pageWarnings(slug, ver string, files []string) []string {
	if !hasFile(files, "index.html") {
		return []string{"there is no index.html. The viewer shows index.html, so add one"}
	}
	var out []string
	for _, f := range files {
		if hasLocalURL(filepath.Join(s.store.VersionDir(slug, ver), f)) {
			out = append(out, f+": a URL points at localhost or 127.0.0.1. It breaks on other machines and public links. "+
				"Upload the file with the mockup and use a relative path")
		}
		if !strings.HasSuffix(f, ".html") {
			continue
		}
		title, desc := pageMeta(filepath.Join(s.store.VersionDir(slug, ver), f))
		if title == "" {
			out = append(out, f+": no <title>. Add the project title")
		}
		if desc == "" {
			out = append(out, f+`: no <meta name="description">. Add what is new in this version`)
		}
	}
	return out
}

// localRe finds URLs to this machine, such as a dev server, in pages, styles and scripts.
var localRe = regexp.MustCompile(`(?i)\bhttps?://(localhost|127\.\d+\.\d+\.\d+|0\.0\.0\.0|\[::1\])\b`)

func hasLocalURL(path string) bool {
	switch filepath.Ext(path) {
	case ".html", ".css", ".js":
	default:
		return false
	}
	b, _ := os.ReadFile(path)
	return localRe.Match(b)
}
