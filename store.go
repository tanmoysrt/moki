package main

import (
	"fmt"
	"html"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Store reads and writes projects, versions and memory in the moki folder.
// It keeps no metadata files: titles and notes come from the pages.
type Store struct {
	dir string
}

type Project struct {
	Slug        string    `json:"slug"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Versions    int       `json:"versions"`
	Version     string    `json:"version"`
	Time        time.Time `json:"time"`
}

type Version struct {
	Name  string    `json:"version"`
	Title string    `json:"title"`
	Note  string    `json:"note"`
	Time  time.Time `json:"time"`
}

var (
	slugRe    = regexp.MustCompile(`^[a-z0-9-]+$`)
	versionRe = regexp.MustCompile(`^v[0-9]+$`)
)

// Projects lists every project with at least one version, newest first.
func (s *Store) Projects() []Project {
	entries, _ := os.ReadDir(s.dir)
	var list []Project
	for _, e := range entries {
		if !e.IsDir() || checkSlug(e.Name()) != nil {
			continue
		}
		if p, _, err := s.Project(e.Name()); err == nil {
			list = append(list, p)
		}
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Time.After(list[j].Time) })
	return list
}

// Project returns the project and its versions, oldest first.
func (s *Store) Project(slug string) (Project, []Version, error) {
	vers := s.Versions(slug)
	if len(vers) == 0 {
		return Project{}, nil, fmt.Errorf("project %q does not exist. Call list_projects to see the projects", slug)
	}
	last := vers[len(vers)-1]
	p := Project{Slug: slug, Title: last.Title, Description: last.Note, Versions: len(vers), Version: last.Name, Time: last.Time}
	if p.Title == "" {
		p.Title = slug
	}
	return p, vers, nil
}

// Versions lists the versions of a project, oldest first.
func (s *Store) Versions(slug string) []Version {
	entries, _ := os.ReadDir(filepath.Join(s.dir, slug))
	var list []Version
	for _, e := range entries {
		if !e.IsDir() || !versionRe.MatchString(e.Name()) {
			continue
		}
		dir := filepath.Join(s.dir, slug, e.Name())
		title, note := pageMeta(filepath.Join(dir, "index.html"))
		list = append(list, Version{Name: e.Name(), Title: title, Note: note, Time: stamp(dir)})
	}
	sort.Slice(list, func(i, j int) bool { return versionNum(list[i].Name) < versionNum(list[j].Name) })
	return list
}

// Files lists the paths in one version, with "/" separators.
func (s *Store) Files(slug, ver string) []string {
	root := s.VersionDir(slug, ver)
	var files []string
	filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err == nil && d.Type().IsRegular() {
			rel, _ := filepath.Rel(root, p)
			files = append(files, filepath.ToSlash(rel))
		}
		return nil
	})
	return files
}

func (s *Store) VersionDir(slug, ver string) string { return filepath.Join(s.dir, slug, ver) }

// NextVersion is the name after the newest version.
func (s *Store) NextVersion(slug string) string {
	vers := s.Versions(slug)
	if len(vers) == 0 {
		return "v1"
	}
	return "v" + strconv.Itoa(versionNum(vers[len(vers)-1].Name)+1)
}

func checkSlug(slug string) error {
	if !slugRe.MatchString(slug) {
		return fmt.Errorf(`slug %q is not valid. Use lowercase letters, digits and "-", for example "my-app"`, slug)
	}
	if slug == "memory" {
		return fmt.Errorf(`slug "memory" is reserved. Pick another slug, for example "memory-page"`)
	}
	return nil
}

func checkVersion(ver string) error {
	if !versionRe.MatchString(ver) {
		return fmt.Errorf(`version %q is not valid. Use "v" and a number, for example "v2"`, ver)
	}
	return nil
}

func versionNum(v string) int {
	n, _ := strconv.Atoi(strings.TrimPrefix(v, "v"))
	return n
}

// stamp is the time of a version: its folder or its index.html, whichever changed last.
// The index.html time catches edits in place, which do not touch the folder.
func stamp(dir string) time.Time {
	var t time.Time
	if fi, err := os.Stat(dir); err == nil {
		t = fi.ModTime()
	}
	if fi, err := os.Stat(filepath.Join(dir, "index.html")); err == nil && fi.ModTime().After(t) {
		t = fi.ModTime()
	}
	return t
}

var (
	titleRe = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
	metaRe  = regexp.MustCompile(`(?is)<meta\s[^>]*>`)
	nameRe  = regexp.MustCompile(`(?is)\bname\s*=\s*["']?description["'\s/>]`)
	contRe  = regexp.MustCompile(`(?is)\bcontent\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s"'>]+))`)
)

// pageMeta reads the <title> and <meta name="description"> from the head of a page.
func pageMeta(path string) (title, desc string) {
	f, err := os.Open(path)
	if err != nil {
		return "", ""
	}
	defer f.Close()
	b, _ := io.ReadAll(io.LimitReader(f, 64<<10))
	if m := titleRe.FindSubmatch(b); m != nil {
		title = clean(string(m[1]))
	}
	for _, tag := range metaRe.FindAll(b, -1) {
		if !nameRe.Match(tag) {
			continue
		}
		if m := contRe.FindSubmatch(tag); m != nil {
			desc = clean(string(m[1]) + string(m[2]) + string(m[3]))
		}
		break
	}
	return title, desc
}

func clean(s string) string {
	return strings.Join(strings.Fields(html.UnescapeString(s)), " ")
}
