# moki plan

moki is a local mockup viewer. Agents write mockups over MCP. You look at them in the browser. Each mockup is plain HTML, CSS and JS in `~/.moki`.

The UI design is in `var/mockups/moki-ui/`:

- `app.html`: the clickable prototype. Its `app.js`, `viewer.js`, `kit.js` and CSS are the start for `assets/ui/`.
- `final.html`: every screen and state in light and dark.
- `index.html`: the round 1 options.
- `board.js`: the start for `/kit/board.js`.

The guide drafts are in `assets/llm-guide.txt` and `assets/design-skill.md`.

## Decisions

All decisions are from 2026-11-08.

### Product

1. moki does not decide how a mockup looks. The viewer shows the version `index.html` as the agent made it. Rejected as too much structure: device modes (Fit, Desktop, Phone, Both), compare, a board outline, `board.json`.
2. A version is one round of ideas. One version can hold many mockups, options and states of one thing.
3. Guidance goes into the design skill, not into rules that moki checks. Upload warnings never block.
4. moki does not warn about files from other hosts. Made-up avatars and images from anywhere are fine.
5. The UI is read-only. Only MCP and the CLI write.
6. Local first: one binary, plain HTTP, no accounts, no database. Basic auth is optional.
7. The default port is 9999. The default host is `127.0.0.1`.

### Agents

8. To connect an agent, the user clicks "Connect agent" in moki. That makes a token, and moki shows the guide with the token in it. The user copies it into any agent, which needs no moki command.
9. When the browser cannot make tokens, the user runs `moki token register` where moki runs and pastes the output: the same guide with the token. `MOKI_URL` sets the URL in that output. In the browser, the URL is the one in the address bar.
10. MCP is the main interface: upload, screenshot, download and memory, in eight tools.
11. `upload` with `base` sends only the changed files. `upload` with `dir` copies a local folder and keeps big files out of the agent context.
12. `upload` checks its own result: it returns a screenshot sheet and warnings in the same call.
13. `screenshot` takes many shots in one call and returns one labeled sheet. `full: true` handles pages with an inner scroller. The agent does not send a large height.
14. Memory is one `memory/<slug>.md` for each project. Only MCP reads and writes it. `get_project` returns it.
15. There is no delete over MCP. The user deletes folders.
16. Mockup first is a rule, not a hint: before a UI change, a refactor of a screen or a new page, the agent makes a mockup and gets the user's OK. A new chat forgets, so moki repeats the rule and "read the guide and the design skill at the start of each chat" in the MCP instructions, the tool descriptions, the first content of `list_projects` and `get_project`, the guide and the skill. The guide also has the agent save both rules in its standing instructions (CLAUDE.md, AGENTS.md).

### CLI and tokens

17. The CLI has only `moki`, `moki token register`, `moki token list`, `moki token revoke`, `moki llm-guide`, `moki version` and `moki help`. The design skill has no CLI command: moki serves it at `/design-skill.txt`.
18. The connect dialog makes tokens. The CLI makes, lists and revokes them. The browser may make tokens only on the moki machine or past basic auth.

### UI

19. Projects home is a gallery of cards. No primary button, because the UI makes nothing.
20. The viewer bar is 44 px high: back, title, version pills, theme, open in a new tab. Rejected: the screenshot button, mode labels that change the bar width.
21. No eyebrows, no subtitles that repeat a fact, no folder tree on the first-run page.
22. One theme for moki and the mockup inside it. Mockups have no theme toggle of their own.
23. Live update: the viewer follows the newest version with a cross-fade. On an older version it stays and shows a blue dot on the new pill.
24. Daily use: the arrow keys and Esc also work when the focus is inside the page. Stepping versions keeps the scroll position. The tab title names the project and version. Every response is `no-cache` with an `ETag`.

### Code and release

25. Go standard library `net/http`, not Echo v5. The MCP Go SDK and chromedp are the only dependencies.
26. chromedp starts Chrome once for a batch of shots, then stops it.
27. Package `main`, files under 300 lines, simple code, no tests.
28. GitHub Actions builds the release on a `v*` tag. `install.sh` and `go install` install it.

## Goals

- One binary. Run `moki` and open `http://127.0.0.1:9999`.
- Local first. No accounts, no database, no build step for the UI.
- The UI is read-only. Only MCP and the CLI write.
- moki does not decide how a mockup looks. The agent writes any page: a board of options, a switcher, one screen, made-up avatars and data. moki shows `index.html` as it is.
- A version is a new set of ideas. One version can hold many mockups of one thing.
- Simple, clean code. No tests.

## Stack

| Part | Choice | Why |
|---|---|---|
| HTTP | Go standard library `net/http` | Go 1.22 routes match method and wildcards. File serving and basic auth are built in. Echo v5 adds dependencies and nothing that moki needs. |
| MCP | `github.com/modelcontextprotocol/go-sdk` | The official SDK. It has a streamable HTTP handler and makes tool schemas from Go structs. |
| Screenshots | `github.com/chromedp/chromedp` | One Chrome process takes many screenshots, then stops. It controls viewport, scale, dark mode and full page. |
| UI | Embedded `index.html`, `app.css`, `app.js` | Hash routes, no framework, no build. |

moki needs Chrome or Chromium on the machine for screenshots. Everything else works without it.

## Files on disk

```
~/.moki/                       # or MOKI_HOME, or --dir
  tokens                       # one token per line
  memory/<slug>.md             # agent memory for one project
  .cache/<slug>/<version>.png  # light thumbnail, safe to delete
  .cache/<slug>/<version>.dark.png
  <slug>/
    v1/index.html              # plus any css, js, fonts or images
    v2/index.html
```

- A slug matches `[a-z0-9-]+`. The slug `memory` is reserved. A version matches `v[0-9]+`.
- A save makes the next version. An approved version does not change.
- moki stores no metadata files. It reads the metadata from the page:
  - The project title is the `<title>` of the newest version.
  - The version note is `<meta name="description">` of that version.
  - The version time is the modification time of its folder.

### Tokens

- A line in `tokens` is `<RFC 3339 time> <token> <name> [<first use>]`. moki adds the first use when an agent sends its first MCP request with the token. moki picks the name, such as `brave-otter`, unique among the tokens, and the token starts with it. For example: `2026-11-08T12:40:00Z mk_brave-otter_7HQ2XV9PLR3A91FK4M2N brave-otter`.
- `POST /api/tokens` makes a token. The CLI makes, lists and revokes them. Lists show the name and the time.
- The browser may make tokens when the request comes from a moki page (the `Origin` host is the `Host`), and either basic auth is set or the request comes from loopback to a loopback name. The name check stops DNS rebinding. In Docker every request comes from the gateway, so the browser cannot make tokens there without basic auth.
- moki reads `tokens` on each MCP request, so a revoke takes effect at once.

### Memory

- `memory/<slug>.md` holds what an agent must remember about one project: the brief, the decisions, the user feedback, the approved version.
- `get_project` returns it with the project, so the agent has it before it starts work. The agent writes it after each round of feedback.
- Only MCP reads and writes memory. The UI does not show it, and moki does not serve it over HTTP.

## CLI

| Command | Action |
|---|---|
| `moki` | Starts the server on port 9999. When moki already runs on that port, it prints the URL and exits. |
| `moki llm-guide` | Prints the LLM guide |
| `moki token register` | Makes a token with a new name and prints the guide with it |
| `moki token list` | Lists tokens: name and date |
| `moki token revoke <name>` | Deletes the token with that name |
| `moki version` | Prints the version |

Server flags:

| Flag | Env | Default |
|---|---|---|
| `--host` | `MOKI_HOST` | `127.0.0.1` |
| `--port` | `MOKI_PORT` | `9999` |
| `--dir` | `MOKI_HOME` | `~/.moki` |
| `--username` | `MOKI_USERNAME` | none |
| `--password` | `MOKI_PASSWORD` | none |
| `--url` | `MOKI_URL` | `http://<host>:<port>` |

- When `--username` and `--password` are set, basic auth protects every route except `/mcp`.
- When `--host` is not a loopback address and there is no basic auth, moki prints a warning.
- Use the env vars for the password, so that it does not show in `ps` or the shell history.

`moki token register` prints the URL that agents use to reach moki. It takes the URL from `MOKI_URL`. Without it, it uses `MOKI_HOST` and `MOKI_PORT`. When that host is a loopback address and moki runs on a server, set `MOKI_URL`, for example `MOKI_URL=http://devbox:9999`.

`moki token register` prints the guide with the token in "1. Set up" on stdout: add the MCP server with the token, then get the design skill. The name of the token and the revoke command go to stderr, so `moki token register | pbcopy` copies only the guide.

## Connect an agent

The user clicks "Connect agent" in moki. That makes a token, and moki shows the guide with a setup part for it: add the MCP server, get the design skill, and call the tools over HTTP until the MCP server loads. The user copies the text into any agent. One page load makes one token.

When the browser cannot make tokens, the user runs `moki token register` on the moki machine and pastes the output.

An agent on the moki machine can also set itself up. The user pastes one line into the agent:

```
Run "moki llm-guide" and follow it.
```

The guide tells the agent to do these steps:

1. Start moki in the background: `moki --bg`. It does nothing when moki already runs.
2. Make a token: `moki token register`. It prints the guide with the token in it.
3. Add the MCP server with the token.
4. Get the design skill.
5. Until the tool loads the MCP server, call the tools over HTTP: one `POST /mcp` with `tools/call` is one call. moki runs MCP stateless, so there is no session to start.

`moki --bg` starts moki again without `--bg`, in its own session, with its output in `<dir>/moki.log`. It waits until `/health` answers, then prints the URL, the log and the command that stops it. When the child stops first, it prints the last line of the log.

Both setup parts tell the agent to skip them when the `moki` MCP server already works.

## HTTP routes

All routes use plain HTTP.

| Route | Returns |
|---|---|
| `GET /` | The UI |
| `GET /p/<slug>/<version>/...` | Files of one version. moki serves only paths of this shape, so `tokens`, `memory` and `.cache` stay private. |
| `GET /api/projects` | Projects with title, description, version count, time and thumbnail URLs |
| `GET /api/projects/<slug>` | Versions with note and time |
| `GET /api/events` | Server-sent events. One `change` event for each new or changed version, and one `agent` event when an agent uses its token for the first time. |
| `GET /health` | `moki <version>`. The guide and `moki` use it to check that the server runs. |
| `GET /thumb/<slug>/<version>.png?dark=1` | The cached thumbnail for the theme |
| `GET /llm-guide.txt` | The LLM guide |
| `GET /design-skill.txt` | The design skill |
| `GET /kit/board.js`, `/kit/theme.js`, `/kit/inter.woff2` | Optional helpers: the options board, the theme helper and the Inter font |
| `GET /_sheet?ids=` | The internal page that moki takes a shot of to make a sheet |
| `POST /api/tokens` | Makes a token. Returns its name and the guide with the token in it, set to the URL in the browser. 403 when this browser may not make tokens. |
| `GET /api/agent` | The name of the token an agent used first most recently, or `""` before any agent connects. The first-run page shows it. |
| `POST /mcp` | MCP. Needs `Authorization: Bearer <token>`. |

The UI gets changes from `/api/events`. See [Live update](#live-update).

Every response has `Cache-Control: no-cache` and an `ETag`, the UI files and the version files too. The browser checks each file again, so it never shows an old file after an upload.

## MCP

The MCP server is the main way to work with moki. It is for upload, screenshots, download and memory. Each tool does a full job in one call, so an agent needs few calls per round.

### What an agent gets when it connects

- **Instructions.** The MCP `initialize` reply has a short summary of the workflow and the rules. The agent knows them before it reads the guide.
- **Resources.** `moki://llm-guide`, `moki://design-skill` and `moki://memory/<slug>`.
- **Prompts.** Claude Code shows MCP prompts as slash commands:
  - `new` with `slug` and `brief`: starts a new project. It loads the design skill and the steps for round 1.
  - `iterate` with `slug` and `feedback`: starts the next round. It loads the memory, the newest version and the steps.

### Tools

| Tool | Input | Output |
|---|---|---|
| `list_projects` | none | Slug, title, description, version count, time and `has_memory` for each project |
| `get_project` | `slug` | The context to start work: title, description, memory text, and each version with note, time, files and viewer URL |
| `upload` | `slug`, `files` or `dir`, `base?`, `delete?`, `version?`, `check?` | The version, its viewer URL, warnings and a screenshot sheet |
| `screenshot` | `slug`, `versions?`, `pages?`, `sizes?`, `themes?`, `full?`, `selector?`, `sheet?` | A screenshot sheet or single images, and console errors |
| `download_project` | `slug`, `version?`, `dir?` | The files as `.tar.gz`. Without `version`, the whole project folder. |
| `list_memory` | none | Slug, time and first line of each memory file |
| `read_memory` | `slug` | The text of `memory/<slug>.md`, or empty |
| `write_memory` | `slug`, `content`, `append?` | Replaces the memory, or adds `content` at the end when `append` is true |

### upload

- `files` is a list of `{path, content, base64?}`. Use `base64` for fonts and images.
- `dir` is a folder on the machine that runs moki. moki copies it as the version. An agent on the same machine writes the files to disk, then uploads them in one short call. This keeps big files out of the agent context.
- `base: "v3"` copies v3 into the new version first. Then moki writes `files` over it and removes the paths in `delete`. The agent sends only the files that changed.
- Without `version`, moki makes the next version. With `version`, it replaces that version, for small fixes before review.
- `check` is true by default. After the write, moki takes `index.html` at desktop and phone size and returns one sheet. The agent sees the result in the same call.
- moki returns warnings that the agent can fix. A warning never blocks the upload:
  - A page has no `<title>` or no `<meta name="description">`.
  - A local file is missing: a link, a script, a style or an image that gives 404.
  - The console has errors.
  - The page scrolls sideways at 390 px.
- moki writes into a temporary folder, then renames it. The UI never sees half a version.
- moki rejects a path that leaves the version folder. A `dir` copy skips symlinks.
- After the write, moki sends a `change` event and queues light and dark thumbnails.

### download_project

- Without `dir`, it returns the `.tar.gz` as an MCP embedded resource (`application/gzip`). The limit is 20 MB. Above that, use `dir`.
- With `dir`, moki writes the files into that folder on the machine that runs moki and returns the file list.
- The archive has the same layout as on disk: `<slug>/<version>/...`.

### Errors

Each error says how to fix it. For example: `slug "My App" is not valid. Use lowercase letters, digits and "-", for example "my-app".`

There is no delete tool. To delete a project or a version, delete its folder.

## Live update

The UI updates by itself when a version arrives. It does not reload the page and nothing flashes.

Server:

1. One goroutine checks the modification times of the slug and version folders every second. This finds uploads and also files that an agent writes directly to disk.
2. `upload` sends its event at once, without the wait.
3. Each change goes to all clients on `/api/events` as `{"slug": "teams", "version": "v4"}`.

The browser `EventSource` reconnects by itself when moki restarts.

Projects home:

- The card of the changed project moves to the top with a short fade and translate.
- The new thumbnail loads in the background, then fades over the old one.
- The age changes to "just now".

Version viewer:

- When the user is on the newest version, the viewer follows the new version. It loads the page in a hidden iframe. When that iframe fires `load`, it fades in over the old one in 150 ms, and the old one is removed.
- When the same version changes in place, the viewer swaps it the same way and keeps the scroll position.
- When the user is on an older version, the viewer does not move. The new version pill gets a blue dot, the live color. On a phone, the stepper gets the dot.
- The dot goes away when the user opens that version.

## Theme

moki and the mockup inside it use one theme. When the user selects dark, the mockup also shows dark.

- moki starts with `prefers-color-scheme`. The theme button switches between light and dark. moki keeps the choice in `localStorage` under `moki-theme`.
- Mockups load from the same origin as the UI, so they can read the same `localStorage` key.
- `/kit/theme.js` sets the `dark` class on `<html>`. It reads `?theme=`, then `moki-theme`, then `prefers-color-scheme`. It listens for the `storage` event, so an open mockup tab follows a change.
- The viewer also sets the `dark` class on the iframe `<html>`, for pages that do not load `theme.js`.
- The design skill tells agents not to add a theme toggle, to style dark mode with `.dark` on `<html>` and to load `/kit/theme.js`.
- A page without a dark mode stays light. moki does not change its colors.

## Screenshots

### The screenshot tool

All inputs except `slug` are optional:

| Input | Values | Default |
|---|---|---|
| `versions` | `"v3"`, `["v2", "v3"]` or `"all"` | The newest version |
| `pages` | `["index.html", "b.html"]` or `"all"` (every `.html` file) | `["index.html"]` |
| `sizes` | Names or `"WxH"`. See the table below. | `["desktop", "phone"]` |
| `themes` | `"light"`, `"dark"` or `"both"` | `"light"` |
| `full` | `true` takes the whole page height | `false` |
| `selector` | A CSS selector, to take one element, for example one option on a board | none |
| `sheet` | `true` puts all shots on one image with labels | `true` when there is more than one shot |

| Size name | Viewport |
|---|---|
| `desktop` | 1280 x 800 |
| `laptop` | 1440 x 900 |
| `tablet` | 834 x 1194 |
| `phone` | 390 x 844 |
| `phone-landscape` | 844 x 390 |

moki takes every combination of versions, pages, sizes and themes. The limit is 48 shots in one call.

The output:

- With `sheet`, one image with a grid of all shots. Each shot has a label such as `v3 · index.html · phone · dark`. This costs much less context than many images.
- Without `sheet`, one image for each shot.
- A list of the shots, each with the URL of the full-size PNG in the cache.
- The console errors of each page.

Images for the agent use scale 1. A long side above 1600 px gets split into more images, because a vision model shrinks larger images until text cannot be read.

### Full-page shots

The agent does not need to send a large height. `full: true` is enough:

1. For a normal page, chromedp `FullScreenshot` uses the CDP `captureBeyondViewport` option. It takes the whole document at its scroll height.
2. Some pages are app shells: `body` is `100%` high with `overflow: hidden`, and an inner element scrolls. The csem mockups work like this. For these pages the document is only one screen high, so step 1 shows one screen. Before the shot, moki finds the tallest scrollable element. Then it makes the viewport taller by the hidden part of that element, up to 12000 px. The sidebar and the fixed parts stay in place.
3. moki splits a tall shot into parts of 1600 px for the agent, in order. The sheet shows the parts side by side.

A size such as `"1280x3000"` still works when the agent wants an exact height.

### Capture

One function takes a batch of shots, then stops Chrome:

```go
type Shot struct {
	URL      string
	W, H     int
	Scale    float64
	Dark     bool
	Full     bool
	Selector string
}

// Capture starts Chrome, takes every shot, and stops Chrome.
func Capture(ctx context.Context, shots []Shot) ([]Result, error)
```

For each shot, `Capture` does these steps:

1. Set the viewport to `W` x `H` at `Scale`. Set mobile and touch when `W` is less than 768.
2. Set `prefers-color-scheme` with `emulation.SetEmulatedMedia`. Open the URL with `?theme=dark` or `?theme=light`.
3. Wait for `document.fonts.ready` and for the network to go quiet, then wait 300 ms more.
4. Take the shot: the element for `Selector`, the full page for `Full`, or else the viewport.
5. Keep the console errors, the failed requests and the sideways-scroll check for the warnings.

Rules:

- Do not wait for a fixed selector such as `#app`. A mockup can have any DOM.
- Return errors. Do not panic.
- Use `CHROME` as the Chrome path when it is set. Otherwise chromedp finds Chrome.
- One batch runs at a time. A batch uses up to 4 tabs in parallel.
- Each batch has a 60 second timeout.

### Sheets

moki makes a sheet without an image library. It writes the shots to `.cache/shots/`, opens an internal page `/_sheet?ids=...` that shows them in a grid with labels, and takes one shot of that page in the same Chrome session.

### Thumbnails

One worker goroutine makes thumbnails. A save adds the version to a pending set. The worker takes all pending versions, captures each one at 1280 x 800, scale 1, in light and dark, in one batch, and writes them to `.cache`. When Chrome is not installed, the UI shows a scaled live frame of the page.

## Guides

The drafts are in `assets/llm-guide.txt` and `assets/design-skill.md`.

`llm-guide.txt` is the only text that an agent needs to start. `moki llm-guide` prints the same text. It has these parts:

1. Setup. With a token from the connect dialog: add the MCP server with it, then get the design skill. Without one: the five steps in [Connect an agent](#connect-an-agent).
2. The MCP tools and the folder layout. Prefer `upload` with `dir` when the agent runs on the same machine.
3. Memory: call `get_project` before work on a project, because it has the memory. Call `write_memory` after each round of feedback.
4. Rules for each round:
   - Make a new version. Use `upload` with `base` and send only the changed files.
   - Give each page a `<title>` and a `<meta name="description">` that says what changed.
   - Read the sheet and the warnings that `upload` returns. Fix the problems before the reply.
   - Before the reply, call `screenshot` with `themes: "both"`, and with `full: true` for long pages.
   - Send the user the viewer URL.
5. Load the design skill and follow it.

`design-skill.txt` is a general version of `csemv2/skills/ui-design-guide.md`. It is guidance, not a format that moki checks:

- The taste rules: calm, one primary action, color only for meaning, the same radius and inset everywhere.
- No eyebrows and no subtitles that repeat the title. Show a fact once.
- Keep the page minimal: one `index.html` that shows all the options of the round.
- A suggested board layout: sections, then options with a letter, a name and one line on what is different, then desktop and phone frames of each option. `/kit/board.js` draws this layout from a small `window.BOARD` object. It shows only the phone frames on a phone and follows the moki theme. Its use is optional.
- Dark mode with `.dark` on `<html>`. moki sets that class. `/kit/theme.js` does the same when the page is opened alone.
- Make the data look real: names, numbers, avatars. Do not use lorem ipsum.
- Check every screen at 1280 and 390 wide, in light and dark.

## UI

The final design has these screens and states. No screen has eyebrows or subtitles.

1. Projects home, route `#/`. The heading "Projects", then a gallery of cards. A card has the newest thumbnail in the current theme, the title and age on one line, and the description below. Press `/` to search.
2. Version viewer, route `#/p/<slug>/<version>`. The version `index.html` fills the screen in a same-origin iframe, as the agent made it. One bar on top, 44 px high: back, the project title and the version pills, then theme and open in a new tab. The arrow keys step versions and keep the scroll position, so the user compares two versions at the same place. Esc goes back to the projects. These keys also work when the focus is inside the page, because moki listens on the same-origin page too. moki skips a key that the page uses: in an input, or when the page calls `preventDefault`. The browser tab title is `<project> v3 · moki`. On a phone, a floating stepper replaces the pills. moki has no device modes and no compare: the agent puts frames, options and switchers in its own page when it wants them, and takes other sizes with `screenshot`.
3. Live update. See [Live update](#live-update).
4. Connect dialog. Opening it makes a token. It shows "Paste this into any agent.", a copy button and the guide with the token. Nothing else. When the browser cannot make tokens, it shows `moki token register` instead.
5. First run. One card with the logo, "No mockups yet" and a "Connect agent" button.

## Repository

```
main.go          # config and serve
cli.go           # commands, flags and env
help.go          # moki help
server.go        # routes, basic auth, events
tokenapi.go      # the token API behind the connect dialog
guide.go         # the guide, set up for a token or for the CLI
store.go         # read and write projects, versions and memory
mcp.go           # MCP tools, instructions, resources, prompts
shot.go          # chromedp capture, full page, warnings
sheet.go         # sheets and the thumbnail worker
tokens.go        # make, list, revoke and check tokens
names.go         # token names such as brave-otter
assets/
  ui/            # index.html, app.css, app.js
  kit/           # board.js, theme.js, inter.woff2
  llm-guide.txt
  design-skill.md
install.sh
Dockerfile
.github/workflows/release.yml
.github/workflows/docker.yml
README.md
```

All files are in package `main`. Each file stays under 300 lines. `go:embed` puts `assets/` in the binary.

## Build and install

The GitHub workflow runs on a `v*` tag:

1. Build for linux, darwin and windows, on amd64 and arm64, with `CGO_ENABLED=0` and `-ldflags "-s -w -X main.version=<tag>"`.
2. Upload the binaries and a `checksums.txt` to the GitHub release.

Install:

```sh
curl -fsSL https://raw.githubusercontent.com/tanmoysrt/moki/develop/install.sh | sh
# or
go install github.com/tanmoysrt/moki@latest
```

`install.sh` finds the OS and the architecture, gets the newest release, checks the checksum and puts `moki` in `~/.local/bin`.

## Todos

Do the milestones in order. Each milestone ends with a working binary.

### M0. Repository

- [ ] `git init`, `.gitignore` with `var/` and `dist/`.
- [ ] Add the dependencies to `go.mod`: the MCP Go SDK and chromedp.
- [ ] Move the guide drafts and the prototype files to their places in `assets/`.

### M1. Server and storage

- [ ] `main.go`: commands, flags and env vars. Make `~/.moki` when it is missing.
- [ ] `moki`: when `/health` answers on the port, print the URL and exit.
- [ ] `store.go`: list projects and versions. Read the title and the description from the page, the time from the folder.
- [ ] Check slugs and versions. Reserve the slug `memory`.
- [ ] `GET /p/<slug>/<version>/...`: serve only paths of this shape.
- [ ] `GET /api/projects`, `GET /api/projects/<slug>`, `GET /health`.
- [ ] Middleware: `Cache-Control: no-cache` and `ETag` on every response.
- [ ] Middleware: basic auth for every route except `/mcp`. Warn when the host is not loopback and there is no basic auth.

### M2. UI

- [ ] Move the prototype to `assets/ui/` and load the data from `/api`.
- [ ] Projects home: gallery, search with `/`, first-run steps when there are no projects: connect an agent (it says Connected once one has), then ask it for a mockup.
- [ ] Viewer: the page in an iframe, version pills, the phone stepper, the 44 px bar.
- [ ] Keys: left and right arrows, Esc, also from inside the page. Keep the scroll position between versions.
- [ ] Tab title `<project> v3 · moki`.
- [ ] Connect dialog with the agent line, the remote line and the guide links.
- [ ] Theme: button, `moki-theme` in `localStorage`, the `dark` class on the iframe `<html>`.
- [ ] Thumbnail fallback: a scaled live frame when the cache has no thumbnail.
- [ ] Check every screen at 1280 and 390 wide, in light and dark.

### M3. Live update

- [ ] A goroutine that checks the slug and version folders every second.
- [ ] A hub that sends each change to all `/api/events` clients.
- [ ] Home: the changed card moves to the top, the new thumbnail fades in, the age reads "just now".
- [ ] Viewer: follow the newest version with a cross-fade. Swap in place and keep the scroll for a changed version. Blue dot on the new pill when the user is on an older version.

### M4. Tokens and CLI

- [ ] `tokens.go`: make, list, revoke and check tokens. Read the file on each check.
- [ ] `moki token register`: print the guide with a new token.
- [ ] `moki token list`, `moki token revoke <name>`.
- [ ] `moki llm-guide`, `moki version`.

### M5. MCP

- [ ] `POST /mcp` with the streamable HTTP handler and bearer token check.
- [ ] Instructions in the `initialize` reply. Resources `moki://llm-guide`, `moki://design-skill`, `moki://memory/<slug>`. Prompts `new` and `iterate`.
- [ ] `list_projects`, `get_project` with memory, files and viewer URLs.
- [ ] `upload`: `files` with `base64`, `dir`, `base`, `delete`, `version`.
- [ ] `upload`: write to a temporary folder and rename. Reject paths that leave the version folder. Skip symlinks in `dir`. Send a `change` event.
- [ ] `download_project`: `.tar.gz` as an embedded resource up to 20 MB, or files into `dir`.
- [ ] `list_memory`, `read_memory`, `write_memory` with `append`.
- [ ] Error messages that say how to fix the problem.

### M6. Screenshots

- [ ] `shot.go`: `Capture` for a batch. One batch at a time, up to 4 tabs, 60 second timeout, `CHROME` env var.
- [ ] For each shot: viewport and scale, mobile below 768 px, theme, wait for fonts and network.
- [ ] `selector` shots and full-page shots. Make the viewport taller for an inner scroller, up to 12000 px.
- [ ] Split tall images into parts of 1600 px.
- [ ] Collect warnings: missing title or description, 404s, console errors, sideways scroll at 390 px.
- [ ] `sheet.go`: the `/_sheet` page and one shot of it.
- [ ] `screenshot` tool: every combination of versions, pages, sizes and themes, up to 48 shots.
- [ ] `upload` check: desktop and phone sheet of `index.html` with the warnings.
- [ ] Thumbnail worker: pending set, light and dark, `.cache`, `GET /thumb/...`.

### M7. Guides and kit

- [ ] Finish `assets/llm-guide.txt` and `assets/design-skill.md` against the real tools.
- [ ] `/kit/board.js` from the prototype.
- [ ] `/kit/theme.js`: `?theme=`, then `moki-theme`, then `prefers-color-scheme`. Follow the `storage` event.
- [ ] `/kit/inter.woff2`.

### M8. Release

- [ ] `.github/workflows/release.yml`: six builds and `checksums.txt` on a `v*` tag.
- [ ] `install.sh`: OS and architecture, newest release, checksum, `~/.local/bin`.
- [ ] `README.md`: install, run, connect an agent, flags.
- [ ] Do one full round with a real agent: connect, new project, upload, screenshot, feedback, second version.

## Out of scope

- HTTPS.
- Accounts, roles or edits from the UI.
- Tokens or memory in the UI.
- Comments or annotations on mockups.
- Delete over MCP.
