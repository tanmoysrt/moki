package main

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const instructions = `moki stores UI mockups and shows them in the browser. You write plain HTML, CSS and JS.
moki does not decide how a mockup looks: it shows the version index.html as you made it.

Rules:
- Mockup first. Before you change UI, refactor a screen or a flow, or design a new page or feature,
  make a mockup in moki and get the user's OK. Write the real code only after that. This is not optional.
- At the start of each chat, before your first mockup, read the guide and the design skill:
    http://127.0.0.1:9999/llm-guide.txt      (resource moki://llm-guide)
    http://127.0.0.1:9999/design-skill.txt   (resource moki://design-skill)
  Do not work from what you remember of an earlier chat.

Each round:
1. get_project(slug) before you work on a project. It has the memory of past rounds.
2. Follow the design skill (resource moki://design-skill).
3. upload a new version. Use base with only the changed files, or dir when you run on the moki machine.
   Give index.html a <title> (the project title) and <meta name="description"> (what is new in this round).
4. Read the sheet and the warnings that upload returns. Fix them with upload version: "vN".
5. screenshot with themes: "both", and full: true for long pages.
6. write_memory with append: true: the brief, the decisions, the feedback.
7. Send the user the viewer URL.

Do not add a light/dark toggle to pages. Style dark mode with the "dark" class on <html> and load /kit/theme.js.
There is no delete tool. A new round is a new version. Do not change a version that the user approved.`

// newMCPHandler serves MCP over streamable HTTP. It is stateless, so a moki restart
// does not break the sessions of connected agents.
func newMCPHandler(s *Server) http.Handler {
	srv := mcp.NewServer(&mcp.Implementation{Name: "moki", Version: version}, &mcp.ServerOptions{Instructions: s.withURL(instructions)})
	s.addTools(srv)
	s.addResources(srv)
	s.addPrompts(srv)
	return mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return srv },
		&mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true, DisableLocalhostProtection: true})
}

// withURL puts the URL of this moki into a text written for http://127.0.0.1:9999.
func (s *Server) withURL(text string) string {
	return strings.ReplaceAll(text, "http://127.0.0.1:9999", s.cfg.URL)
}

// readFirst opens the reply of the tools that start a chat, so that an agent in a
// new chat reads the guides before it works.
func (s *Server) readFirst() mcp.Content {
	return &mcp.TextContent{Text: s.withURL(`Read first, once per chat: the guide http://127.0.0.1:9999/llm-guide.txt ` +
		`and the design skill http://127.0.0.1:9999/design-skill.txt. ` +
		`Mockup first: get the user's OK on a mockup before you change real UI code.`)}
}

func (s *Server) addResources(srv *mcp.Server) {
	text := func(uri, mime, body string) *mcp.ReadResourceResult {
		return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{{URI: uri, MIMEType: mime, Text: body}}}
	}
	srv.AddResource(&mcp.Resource{
		URI:         "moki://llm-guide",
		Name:        "llm-guide",
		Description: "How to set up and use moki. Read it at the start of each chat.",
		MIMEType:    "text/plain",
	},
		func(ctx context.Context, r *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
			return text(r.Params.URI, "text/plain", guideText(s.cfg.URL, nil)), nil
		})
	srv.AddResource(&mcp.Resource{
		URI:         "moki://design-skill",
		Name:        "design-skill",
		Description: "How to design mockups for moki. Read it at the start of each chat, before the first mockup.",
		MIMEType:    "text/markdown",
	},
		func(ctx context.Context, r *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
			return text(r.Params.URI, "text/markdown", skillText()), nil
		})
	srv.AddResourceTemplate(&mcp.ResourceTemplate{
		URITemplate: "moki://memory/{slug}",
		Name:        "memory",
		Description: "What you remember about one project",
		MIMEType:    "text/markdown",
	},
		func(ctx context.Context, r *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
			slug := strings.TrimPrefix(r.Params.URI, "moki://memory/")
			if checkSlug(slug) != nil {
				return nil, mcp.ResourceNotFoundError(r.Params.URI)
			}
			return text(r.Params.URI, "text/markdown", s.store.ReadMemory(slug)), nil
		})
}

func (s *Server) addPrompts(srv *mcp.Server) {
	arg := func(name, desc string) *mcp.PromptArgument {
		return &mcp.PromptArgument{Name: name, Description: desc, Required: true}
	}
	srv.AddPrompt(&mcp.Prompt{Name: "new", Description: "Start a new moki project",
		Arguments: []*mcp.PromptArgument{arg("slug", "project slug, for example team-page"), arg("brief", "what to design")}},
		func(ctx context.Context, r *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
			a := r.Params.Arguments
			return prompt("Start a moki project", s.newPrompt(a["slug"], a["brief"])), nil
		})
	srv.AddPrompt(&mcp.Prompt{Name: "iterate", Description: "Start the next round of a moki project",
		Arguments: []*mcp.PromptArgument{arg("slug", "project slug"), arg("feedback", "the feedback of the user")}},
		func(ctx context.Context, r *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
			a := r.Params.Arguments
			return prompt("Next round of a moki project", s.iteratePrompt(a["slug"], a["feedback"])), nil
		})
}

func prompt(desc, body string) *mcp.GetPromptResult {
	message := &mcp.PromptMessage{Role: "user", Content: &mcp.TextContent{Text: body}}
	return &mcp.GetPromptResult{Description: desc, Messages: []*mcp.PromptMessage{message}}
}

func (s *Server) newPrompt(slug, brief string) string {
	return fmt.Sprintf(`Start a new moki project with the slug %q.

Brief:
%s

Steps:
1. Read the design skill below and follow it.
2. Make one index.html that shows the options of this round. Give it a <title> and a <meta name="description">.
3. Call upload with slug %q. Read the sheet and the warnings, and fix them with upload version: "v1".
4. Call screenshot with themes: "both". Add full: true for long pages.
5. Call write_memory with the brief and the decisions of this round.
6. Send me the viewer URL and a short list of the options. Recommend one.

--- design skill ---
%s`, slug, brief, slug, skillText())
}

func (s *Server) iteratePrompt(slug, feedback string) string {
	ctx := "The project does not exist yet. Use the new prompt instead."
	if p, vers, err := s.store.Project(slug); err == nil {
		last := vers[len(vers)-1].Name
		ctx = fmt.Sprintf("Title: %s\nNewest version: %s (%s)\nFiles: %s\nViewer: %s\n\nMemory:\n%s",
			p.Title, last, p.Description,
			strings.Join(s.store.Files(slug, last), ", "),
			s.viewerURL(slug, last),
			orNone(s.store.ReadMemory(slug)))
	}
	return fmt.Sprintf(`Start the next round of the moki project %q.

Feedback:
%s

%s

Steps:
1. Read the memory and the newest version. Call download_project or screenshot when you need to see it.
2. Make the next version. Keep what I liked, drop what I rejected. Follow the design skill (moki://design-skill).
3. Call upload with base set to the newest version and only the changed files. Update <meta name="description">.
4. Read the sheet and the warnings, and fix them. Then call screenshot with themes: "both".
5. Call write_memory with append: true: the feedback and the decisions of this round.
6. Send me the viewer URL and what changed.`, slug, feedback, ctx)
}

func (s *Server) viewerURL(slug, ver string) string {
	return s.cfg.URL + "/#/p/" + slug + "/" + ver
}

func orNone(s string) string {
	if strings.TrimSpace(s) == "" {
		return "(none yet)"
	}
	return s
}
