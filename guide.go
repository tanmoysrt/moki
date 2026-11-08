package main

// The guide for agents. With a token, the guide sets the agent up with it: the user
// pastes it into any agent, and the agent needs no moki command. Without one, an
// agent on the moki machine starts moki and makes its own token.

import (
	"strings"
)

// guideText is the guide with the URL of this moki, set up for tok when it is not nil.
func guideText(url string, tok *Token) string {
	b, _ := assets.ReadFile("assets/llm-guide.txt")
	setup := setupWithCLI()
	if tok != nil {
		setup = setupWithToken(tok.Value)
	}
	text := strings.Replace(string(b), "{{setup}}", setup, 1)
	return strings.ReplaceAll(text, "http://127.0.0.1:9999", url)
}

func skillText() string {
	b, _ := assets.ReadFile("assets/design-skill.md")
	return string(b)
}

func setupWithToken(token string) string {
	return `1. Set up

Skip this part when the "moki" MCP server already works for you.

1. ` + scopeStep + `
2. ` + mcpStep(token) + `
3. ` + skillStep + `
4. ` + rememberStep + `
5. ` + httpStep(token)
}

func setupWithCLI() string {
	return `1. Set up

Skip this part when the "moki" MCP server already works for you.

When you run on the user's machine and can run commands, as Claude Code, Codex and
other coding agents do, set yourself up:

1. Start moki in the background. When moki already runs, this does nothing:
     moki --bg
2. Make your token:
     moki token register
   It prints this guide again, with the token in it. Take the token from there.
3. ` + scopeStep + `
4. ` + mcpStep("<token>") + `
5. ` + skillStep + `
6. ` + rememberStep + `
7. ` + httpStep("<token>") + `

If moki runs on another machine, you cannot run these commands. Ask the user to click
"Connect agent" in moki and to paste the text to you.`
}

// scopeStep lets the user choose where the setup goes: for one project, or for all.
const scopeStep = `Before you change any file, always ask the user this question and wait for the answer:
     Set up moki for this project only (the default), or for every project?
   Below, each place comes in two forms: for this project (in the project folder), and for every
   project (in your home folder). When the user has no preference, use this project.`

func mcpStep(token string) string {
	return strings.ReplaceAll(mcpSetup, "{{token}}", token)
}

// mcpSetup adds moki in the tools that agents use most, and says what to do in others.
const mcpSetup = `Add moki as a remote MCP server (streamable HTTP) named "moki":
     url:     http://127.0.0.1:9999/mcp
     header:  Authorization: Bearer {{token}}
   How to add it in your tool. Create the file or the key when it is missing:
   - Claude Code, run with --scope project for this project, or --scope user for every project:
       claude mcp add --scope project --transport http moki http://127.0.0.1:9999/mcp --header "Authorization: Bearer {{token}}"
   - Codex: .codex/config.toml in the project, or ~/.codex/config.toml:
       [mcp_servers.moki]
       url = "http://127.0.0.1:9999/mcp"
       http_headers = { "Authorization" = "Bearer {{token}}" }
   - Cursor: "mcpServers" in .cursor/mcp.json in the project, or ~/.cursor/mcp.json:
       "moki": { "url": "http://127.0.0.1:9999/mcp", "headers": { "Authorization": "Bearer {{token}}" } }
   - OpenCode: "mcp" in opencode.json in the project, or ~/.config/opencode/opencode.json:
       "moki": { "type": "remote", "url": "http://127.0.0.1:9999/mcp", "oauth": false,
                 "headers": { "Authorization": "Bearer {{token}}" } }
   - Antigravity: "mcpServers" in mcp_config.json (agent panel menu: MCP Servers, Manage MCP
     Servers, View raw config). It applies to every project:
       "moki": { "serverUrl": "http://127.0.0.1:9999/mcp", "headers": { "Authorization": "Bearer {{token}}" } }
   - Any other tool: add a remote MCP server with the url and the header above, in its MCP settings.
   For this project, the file holds the token. Always add it to the project's .gitignore, so the
   token does not leak in a commit: .mcp.json, .codex/config.toml, .cursor/mcp.json or opencode.json.`

const skillStep = `Get the design skill and keep it:
     curl -fs http://127.0.0.1:9999/design-skill.txt
   Save it as a skill if your tool has skills. Claude Code: .claude/skills/moki-design/SKILL.md in
   the project, or ~/.claude/skills/moki-design/SKILL.md. Otherwise read it before each mockup.`

// rememberStep keeps the rules in new chats, which do not see this guide.
const rememberStep = `Keep the rules for every chat. Add these lines where your tool keeps standing instructions:
   Claude Code: CLAUDE.md in the project, or ~/.claude/CLAUDE.md. Codex: AGENTS.md in the project,
   or ~/.codex/AGENTS.md. OpenCode: AGENTS.md in the project, or ~/.config/opencode/AGENTS.md.
   Cursor: .cursor/rules/moki.mdc in the project, or a user rule. Other tools: their rules file.
     ## moki
     - Mockup first: before you change UI, refactor a screen or design a new page, make a mockup
       in moki (MCP server "moki") and get my OK. Then write the code.
     - At the start of each chat, read http://127.0.0.1:9999/llm-guide.txt and
       http://127.0.0.1:9999/design-skill.txt before the first mockup.`

// httpStep lets an agent with a shell use moki before its tool loads the MCP server.
// moki answers each MCP request on its own, so one POST is one call.
func httpStep(token string) string {
	return `Your tool may need a restart before the moki tools show. If you can run commands, do not
   wait: call the tools over HTTP. One POST is one call, with no session:
     curl -s http://127.0.0.1:9999/mcp \
       -H "Authorization: Bearer ` + token + `" \
       -H "Content-Type: application/json" -H "Accept: application/json, text/event-stream" \
       -d '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"list_projects","arguments":{}}}'
   "method":"tools/list" lists the tools and their arguments. When moki runs on your machine,
   upload with dir: write the files to disk first, so the request stays small.`
}
