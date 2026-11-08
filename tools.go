package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type slugIn struct {
	Slug string `json:"slug" jsonschema:"project slug, for example team-page"`
}

type memoryIn struct {
	Slug    string `json:"slug" jsonschema:"project slug"`
	Content string `json:"content" jsonschema:"markdown: the brief, the decisions, the feedback, the approved version"`
	Append  bool   `json:"append,omitempty" jsonschema:"add content at the end instead of replacing the memory"`
}

type downloadIn struct {
	Slug    string `json:"slug" jsonschema:"project slug"`
	Version string `json:"version,omitempty" jsonschema:"for example v3. Without it, the whole project"`
	Dir     string `json:"dir,omitempty" jsonschema:"absolute folder on the moki machine to write the files into, instead of returning an archive"`
}

type none struct{}

// addTool adds a tool whose func returns content. A returned error becomes a tool
// error that the agent reads and can fix. Each call prints an activity line.
func addTool[In any](s *Server, srv *mcp.Server, name, desc string, f func(context.Context, In) ([]mcp.Content, error)) {
	run := func(ctx context.Context, r *mcp.CallToolRequest, in In) (*mcp.CallToolResult, any, error) {
		start := time.Now()
		var text string
		content, err := f(context.WithValue(ctx, noteKey{}, &text), in)
		activity.Call(s.agent(r), name, text, time.Since(start), err)
		if err != nil {
			return nil, nil, err
		}
		return &mcp.CallToolResult{Content: content}, nil, nil
	}
	mcp.AddTool(srv, &mcp.Tool{Name: name, Description: desc}, run)
}

// agent is the token name of the agent that sent the request.
func (s *Server) agent(r *mcp.CallToolRequest) string {
	if r.Extra == nil || r.Extra.Header == nil {
		return "agent"
	}
	return s.tokens.Name(r.Extra.Header.Get("Authorization"))
}

func (s *Server) addTools(srv *mcp.Server) {
	addTool(s, srv, "list_projects",
		"List the projects: slug, title, description, version count, time and has_memory. "+
			"At the start of a chat, read the guide and the design skill first (moki://llm-guide, moki://design-skill).",
		s.listProjects)
	addTool(s, srv, "get_project",
		"Get the context to start work on a project: title, description, memory, and each version "+
			"with note, time, files and viewer URL. Call it before you work on a project, "+
			"after you read the guide and the design skill in this chat.",
		s.getProject)
	addTool(s, srv, "upload", uploadDesc, s.upload)
	addTool(s, srv, "screenshot", shotDesc, s.screenshot)
	addTool(s, srv, "download_project",
		"Get the files of a version, or of the whole project, as .tar.gz (up to 20 MB). "+
			"With dir, write them into that folder on the moki machine instead.",
		s.download)
	addTool(s, srv, "list_memory",
		"List the memory files: slug, time and first line.",
		s.listMemory)
	addTool(s, srv, "read_memory",
		"Read the memory of a project. Empty when there is none.",
		s.readMemory)
	addTool(s, srv, "write_memory",
		"Write the memory of a project. Call it after each round of feedback, with append: true.",
		s.writeMemory)
}

func (s *Server) listProjects(ctx context.Context, _ none) ([]mcp.Content, error) {
	type item struct {
		Project
		HasMemory bool `json:"has_memory"`
	}
	list := []item{}
	for _, p := range s.store.Projects() {
		list = append(list, item{p, s.store.ReadMemory(p.Slug) != ""})
	}
	note(ctx, "%s", count(len(list), "project"))
	return append([]mcp.Content{s.readFirst()}, jsonContent(list)...), nil
}

func (s *Server) getProject(ctx context.Context, in slugIn) ([]mcp.Content, error) {
	if err := checkSlug(in.Slug); err != nil {
		return nil, err
	}
	p, vers, err := s.store.Project(in.Slug)
	if err != nil {
		return nil, err
	}
	type ver struct {
		Version
		Files  []string `json:"files"`
		Viewer string   `json:"viewer"`
	}
	out := struct {
		Project
		Memory string `json:"memory"`
		List   []ver  `json:"list"`
	}{Project: p, Memory: s.store.ReadMemory(in.Slug)}
	for _, v := range vers {
		out.List = append(out.List, ver{v, s.store.Files(in.Slug, v.Name), s.viewerURL(in.Slug, v.Name)})
	}
	note(ctx, "%s · %s", in.Slug, count(len(vers), "version"))
	return append([]mcp.Content{s.readFirst()}, jsonContent(out)...), nil
}

func (s *Server) listMemory(ctx context.Context, _ none) ([]mcp.Content, error) {
	list := s.store.ListMemory()
	note(ctx, "%s", count(len(list), "file"))
	return jsonContent(list), nil
}

func (s *Server) readMemory(ctx context.Context, in slugIn) ([]mcp.Content, error) {
	if err := checkSlug(in.Slug); err != nil {
		return nil, err
	}
	note(ctx, "%s", in.Slug)
	return []mcp.Content{&mcp.TextContent{Text: s.store.ReadMemory(in.Slug)}}, nil
}

func (s *Server) writeMemory(ctx context.Context, in memoryIn) ([]mcp.Content, error) {
	if err := checkSlug(in.Slug); err != nil {
		return nil, err
	}
	if err := s.store.WriteMemory(in.Slug, in.Content, in.Append); err != nil {
		return nil, err
	}
	how := "replaced"
	if in.Append {
		how = "appended"
	}
	note(ctx, "%s · %s", in.Slug, how)
	return textContent("Saved memory/%s.md.", in.Slug), nil
}

const maxDownload = 20 << 20

func (s *Server) download(ctx context.Context, in downloadIn) ([]mcp.Content, error) {
	if err := checkSlug(in.Slug); err != nil {
		return nil, err
	}
	if _, _, err := s.store.Project(in.Slug); err != nil {
		return nil, err
	}
	rel := in.Slug
	if in.Version != "" {
		if err := checkVersion(in.Version); err != nil {
			return nil, err
		}
		rel = filepath.Join(in.Slug, in.Version)
		if _, err := os.Stat(filepath.Join(s.cfg.Dir, rel)); err != nil {
			return nil, fmt.Errorf("version %s of %q does not exist. Call get_project to see the versions", in.Version, in.Slug)
		}
	}
	if in.Dir != "" {
		dir, err := absPath(in.Dir)
		if err != nil {
			return nil, err
		}
		files, err := copyTree(filepath.Join(s.cfg.Dir, rel), filepath.Join(dir, rel))
		if err != nil {
			return nil, err
		}
		note(ctx, "%s · %s to %s", rel, count(len(files), "file"), tildePath(dir))
		return textContent("Wrote %d files into %s:\n%s", len(files), filepath.Join(dir, rel), joinLines(files)), nil
	}
	b, err := tarGz(s.cfg.Dir, rel)
	if err != nil {
		return nil, err
	}
	if len(b) > maxDownload {
		return nil, fmt.Errorf("the archive is %d MB, above the 20 MB limit. Use dir to write the files on the moki machine", len(b)>>20)
	}
	note(ctx, "%s · %s", rel, byteSize(len(b)))
	name := filepath.Base(rel) + ".tar.gz"
	if in.Version != "" {
		name = in.Slug + "-" + in.Version + ".tar.gz"
	}
	return []mcp.Content{
		&mcp.TextContent{Text: fmt.Sprintf("%s: %d bytes. The layout is %s/...", name, len(b), filepath.ToSlash(rel))},
		&mcp.EmbeddedResource{Resource: &mcp.ResourceContents{URI: "moki://download/" + name, MIMEType: "application/gzip", Blob: b}},
	}, nil
}

// tarGz packs dir/rel with paths that start at rel, for example teams/v3/index.html.
func tarGz(dir, rel string) ([]byte, error) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	err := filepath.Walk(filepath.Join(dir, rel), func(p string, fi os.FileInfo, err error) error {
		if err != nil || !fi.Mode().IsRegular() {
			return err
		}
		name, _ := filepath.Rel(dir, p)
		if err := tw.WriteHeader(&tar.Header{Name: filepath.ToSlash(name), Mode: 0o644, Size: fi.Size(), ModTime: fi.ModTime()}); err != nil {
			return err
		}
		f, err := os.Open(p)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = io.Copy(tw, f)
		if buf.Len() > maxDownload {
			return fmt.Errorf("the archive is above the 20 MB limit. Use dir to write the files on the moki machine")
		}
		return err
	})
	if err != nil {
		return nil, err
	}
	tw.Close()
	gz.Close()
	return buf.Bytes(), nil
}

func jsonContent(v any) []mcp.Content {
	b, _ := json.MarshalIndent(v, "", "  ")
	return []mcp.Content{&mcp.TextContent{Text: string(b)}}
}

func textContent(format string, a ...any) []mcp.Content {
	return []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf(format, a...)}}
}
