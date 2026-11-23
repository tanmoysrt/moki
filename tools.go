package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
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

type memoryQueryIn struct {
	Query string `json:"query,omitempty" jsonschema:"words to find in every memory, in any case. Without it, list the files"`
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
	addTool(s, srv, "share", shareDesc, s.share)
	addTool(s, srv, "list_memory",
		"List the memory files: slug, time and first line. With query, search every memory instead: "+
			"each line that has all the words, with its heading and the lines next to it. "+
			"Search before a new project, to follow what the user decided in other projects.",
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
		Share  string `json:"share,omitempty"` // the public link, when there is one
		Memory string `json:"memory"`
		List   []ver  `json:"list"`
	}{Project: p, Memory: s.store.ReadMemory(in.Slug)}
	if id := s.shares.All()[in.Slug]; id != "" {
		out.Share = s.cfg.URL + sharePath(id)
	}
	for _, v := range vers {
		out.List = append(out.List, ver{v, s.store.Files(in.Slug, v.Name), s.viewerURL(in.Slug, v.Name)})
	}
	note(ctx, "%s · %s", in.Slug, count(len(vers), "version"))
	return append([]mcp.Content{s.readFirst()}, jsonContent(out)...), nil
}

func (s *Server) listMemory(ctx context.Context, in memoryQueryIn) ([]mcp.Content, error) {
	if strings.TrimSpace(in.Query) != "" {
		return s.searchMemory(ctx, in.Query)
	}
	list := s.store.ListMemory()
	note(ctx, "%s", count(len(list), "file"))
	return jsonContent(list), nil
}

func (s *Server) searchMemory(ctx context.Context, query string) ([]mcp.Content, error) {
	hits, more := s.store.SearchMemory(query)
	note(ctx, "%q · %s", query, count(len(hits), "line"))
	out := struct {
		Hits []MemoryHit `json:"hits"`
		More bool        `json:"more,omitempty"` // true when moki stopped at maxHits
	}{hits, more}
	return jsonContent(out), nil
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
	if size := len(s.store.ReadMemory(in.Slug)); size > maxMemory {
		return textContent("Saved memory/%s.md. It is %d KB, which is long. Read it and write a shorter memory "+
			"without append. Keep each decision, and drop what the next chat does not need.", in.Slug, size>>10), nil
	}
	return textContent("Saved memory/%s.md.", in.Slug), nil
}

func jsonContent(v any) []mcp.Content {
	b, _ := json.MarshalIndent(v, "", "  ")
	return []mcp.Content{&mcp.TextContent{Text: string(b)}}
}

func textContent(format string, a ...any) []mcp.Content {
	return []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf(format, a...)}}
}
