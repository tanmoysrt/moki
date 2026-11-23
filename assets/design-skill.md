---
name: moki-design
description: "How to design mockups for moki: screens, decks, short videos, diagrams and art. Taste, page layout, real-looking data and self-review. Read before you make or change a mockup in moki."
---

# moki design skill

This skill is guidance, not a format. moki shows any page you make. Use what helps the user decide, and leave out the rest.

Mockup first: before you change UI, refactor a screen or design a new page, make the mockup and get the user's OK. Read this skill again at the start of each chat.

## Before you start

1. Search the memory: `list_memory` with a `query` for this kind of screen or part, for example "table" or "invite". Follow what the user decided before. For a project that exists, `get_project` gives you its memory.
2. Look at the user's product. Open the screens near the one you design, or read the codebase: design tokens, components, fonts, icon set. Match them, unless the brief asks for something new. A project memory with a `uses:` line points to the shared rules of the product.
3. To start from a real page, save it as static HTML with its CSS and images, and change that copy. Never use a screenshot of it as the mockup.
4. When the brief leaves the main question open, ask the user one short question before you design.

## The target

- Calm, minimal, bold where it counts. One clear idea per screen.
- Few elements, each with a job. When a line, label or badge repeats what the screen already says, remove it.
- Balanced: not blank and not dense. When a card looks empty, make it smaller. Do not fill it with filler text.
- Plain titles and copy. No hype.

## Visual rules

- Neutral surfaces. Color only for meaning: state, progress, success, the one primary action.
- Hierarchy with weight, size and spacing, not with borders and boxes.
- No eyebrows: no small label or number above a heading. No subtitle that repeats the title. Show a fact once.
- One-line descriptions that end with an ellipsis when they are too long.
- Numbers use `font-variant-numeric: tabular-nums`.
- Cards are soft surfaces with the same elevation on one screen. Never put a card inside a card.
- Inner radius = outer radius minus the padding between them.
- At most one primary button per screen. Put it where the task continues, not in the header.
- Outline icons with a thin stroke, in square tiles with a rounded corner.
- Controls of one kind have the same height, padding and radius on every screen. Check this across the options too.
- No control that shows only on hover, and no motion on hover. The user must see every action, also on a phone.
- Motion: fade and a small translate. Never animate the size of text that the user reads.
- Phone: 16px side gutter, no sideways scroll, touch targets of 40px or more.

## The page

Show the ideas of a round as options. moki draws them as a grid of cards. The user opens one option at full size, steps through the options of a part with `[` and `]`, sees its states, and keeps the ones worth another round. `get_project` lists the options and marks the kept ones.

Options must differ in idea, not only in color. In the first round, give at least 5 options for each part. Give more when the user wants to explore. Brainstorm first: list 8 to 10 ideas, and show the 5 or more that differ most.

What an option can be:
- Screen: a layout (cards, table, rows, one item first, a wizard), or where the main action sits.
- Video: the opening (a word, a number, the product, a terminal) and the style (calm, fast, bold).
- Deck: the cover, and one key slide such as the numbers or the plan.
- Drawing or art: the style (hand-drawn, clean, flat color, line art), or the metaphor.
- Also: logos, emails, headlines and copy, charts, empty states and onboarding.

Narrow down with each version:
1. v1 explores: 5 or more options for each part. The user keeps the ones worth another round.
2. v2 takes only the kept options, with 2 or 3 variants of each.
3. v3 is one option with all its states: empty, full, error, owner and visitor.
4. Then the user approves, and you build it.

Put each option on its own page, for example `plans.html?v=a`, and each state too: `plans.html?v=a&s=empty`. Then give the options in the `options` field of `upload`. moki writes them to `options.json` in the version. With `dir` or tar, write `options.json` yourself:

```json
{
  "parts": [
    {
      "title": "Plans page",
      "size": "1280x560",
      "options": [
        { "id": "A", "name": "Three cards", "note": "Free, Pro and Team side by side.", "page": "plans.html?v=a" },
        { "id": "B", "name": "Comparison table", "note": "Plans are columns.", "page": "plans.html?v=b", "rec": true,
          "states": [{ "name": "Empty", "page": "plans.html?v=b&s=empty" }] }
      ]
    }
  ]
}
```

- A part is one question of the round: the plans page, the intro video, the deck cover.
- `size` is the card of a screen. Crop it to the part that differs. A deck or a video card takes the size of the page.
- `note` says in one line what is different. `rec: true` marks the one you recommend.
- `upload` refuses options that do not follow the format, and files that are not part of a web page. It saves nothing and lists each problem. Fix them and upload again. The guide has each rule, in section Options.
- `index.html` still gives the title and the note of the version. The viewer shows the grid in its place.
- `screenshot` with `pages: ["options"]` takes each option page.

A round with one option and its states needs no options: show it in `index.html`. `/kit/board.js` draws a page of frames, desktop and phone side by side for each state. The comment at its top shows how.

In a frame, do not call `scrollIntoView`. It also scrolls the page around the frame. Set `scrollTop` on the scroll container instead.

## Decks, videos, diagrams and art

A mockup can be anything that HTML can show. The scripts in `/kit` do the hard parts, and the guide shows their HTML. The rules above apply too.

Deck (`/kit/deck.js`):
- One idea for each slide, in a title of 8 words or less. A slide is not a page of text: at most 5 points.
- Use the layouts: `title`, `split`, `cards`, `stats`, `steps`, `compare`, `quote`, `full`. Change them with page CSS when the brand needs it.
- Use real numbers when you have them, and say where they come from. Do not make up numbers for a real company.
- Start and end with a `title` slide. Use `data-footer` for the name of the deck.
- Check with `screenshot` at size `slide`, and look at every slide with `?s=N`. Look for empty slides and text that overflows.

Video (`/kit/video.js`):
- 5 to 20 seconds, one message. Plan it in scenes, for example 0–2 s the title, 2–5 s the product.
- `draw(t)` depends only on `t`. No CSS animation, timer or random value, or the MP4 loses frames.
- Ease the moves with `video.span`. Hold each text long enough to read: about 1 second for 3 words.
- Check frames with `?t=1`, `?t=4` and so on, and send the user the viewer URL to watch it play.

Diagram and art (`/kit/sketch.js`):
- Hand-drawn for ideas and early flows. `clean: true` for diagrams that go into docs.
- Few shapes, few colors, short labels. Arrows show the order: left to right or top to bottom.
- Give each drawing a fixed `seed`, so screenshots do not change between versions.
- Put the drawing in one `<svg>` with a `viewBox`, so the SVG export has all of it.

## Theme

- Do not add a light/dark toggle to your page. The moki bar has one, and it switches your page too.
- Style dark mode with the `dark` class on `<html>`. moki sets that class from its own theme.
- Load `/kit/theme.js` so that the page also follows the moki theme when it is opened alone.
- Frames: pass the theme on with `?dark=1` or `?dark=0`. `/kit/theme.js` in the frame page reads it.

## Data

- Make the data look real: names, usernames, numbers, dates, avatars. Use the data of the user's product when you can get it.
- Never use lorem ipsum.
- Generated avatars, inline SVG and images from public hosts are all fine.
- Never point an image at localhost, 127.0.0.1 or another local address. It breaks when the user opens moki on another machine or with a public link. Upload the image with the mockup and use a relative path, for example `img/logo.png`.
- Say in the intro which data is real and which is made up.

## Check your work

1. Upload, then read the sheet and the warnings that come back.
2. Call `screenshot` with sizes desktop and phone and `themes: "both"`. Add `full: true` for long pages. For a deck or a video, use size `slide`. For a round of options, add `pages: ["options"]`.
3. Check each screen:
   - One primary button. No eyebrow, no repeated fact, no card in a card.
   - Buttons, inputs and bars have the same height and padding on every screen.
   - Color only for meaning. Radius and inset line up.
   - No sideways scroll on the phone. Long text truncates.
   - Empty, loading and error states exist when the screen needs them.
4. Fix what is wrong before you send the URL.

## Rounds

- Read the memory and the notes before each round (`get_project`). The notes are pinned on the page: each says which element the user means.
- After you send the viewer URL, keep checking for new notes and an approval in the background while the user looks. When your client can run a background shell task, wait for the notes text to change, then call `get_project`:
  ```sh
  before=$(curl -s -H "Authorization: Bearer $TOKEN" "$MOKI/api/notes/<slug>")
  until [ "$(curl -s -H "Authorization: Bearer $TOKEN" "$MOKI/api/notes/<slug>")" != "$before" ]; do sleep 30; done
  ```
  Without a background task, call `get_project` again before each reply. When notes come in, tell the user and start the next round. When the user approves a version, that is the one to build.
- Each round of feedback is a new version.
- Build the next round from the kept options. Drop the others. A note on an option names it, for example `Plans page B, Empty`. A note pinned on the phone frame of the viewer says `phone`: fix the phone layout.
- When the user makes the same complaint for 2 or 3 rounds, small changes do not work. Offer new layouts instead.
- Do not change a version that the user approved.
- Write the decisions to memory with `write_memory` and `append: true`: the feedback, what you kept, what the user rejected, and the approved version. The guide has more on memory.
