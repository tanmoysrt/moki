---
name: moki-design
description: How to design mockups for moki: screens, decks, short videos, diagrams and art. Taste, page layout, real-looking data and self-review. Read before you make or change a mockup in moki.
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

Keep one version to one `index.html` that shows every idea of the round. A suggested layout:

1. A title and one line on what the round is about and where the data comes from.
2. Sections, one for each question: for example "Team page as the owner" or "Invite flow".
3. Options in each section: a letter, a short name, and one line on what is different. Mark the one you recommend.
4. Frames for each option: desktop (1280) and phone (390) next to each other, scaled down. Add more frames for more states, such as owner, member and visitor.

Options must differ in idea, not only in color. Give 3 to 5 options for a new screen.

Put each screen in its own file, for example `page.html?v=a&as=owner`, and show it in the frames with `<iframe>`. Then the frames are live: the user can click in them.

In a frame, do not call `scrollIntoView`. It also scrolls the page around the frame. Set `scrollTop` on the scroll container instead.

`/kit/board.js` draws this layout for you. Optional:

```html
<script>
window.BOARD = {
  title: 'Team page and invites',
  intro: 'One line on the round.',
  frame: 'page.html',
  sections: [
    { short: '1 Owner', title: '1. Team page as the owner', opts: [
      { l: 'A', n: 'One page', rec: true, note: 'What is different.', fr: [['', 'v=a&as=owner']] },
      { l: 'B', n: 'Seats', note: 'What is different.', fr: [['Owner', 'v=b&as=owner'], ['Member', 'v=b&as=member']] },
    ]},
  ],
}
</script>
<script src="/kit/board.js"></script>
```

`board.js` shows only the phone frames on a phone, and it follows the moki theme. It has no theme button. You can also make your own page: a switcher, tabs, one long screen. Pick what makes the choice easy.

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
2. Call `screenshot` with sizes desktop and phone and `themes: "both"`. Add `full: true` for long pages. For a deck or a video, use size `slide`.
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
- Keep the options that the user liked. Drop the ones the user rejected.
- When the user makes the same complaint for 2 or 3 rounds, small changes do not work. Offer new layouts instead.
- Do not change a version that the user approved.
- Write the decisions to memory with `write_memory` and `append: true`: the feedback, what you kept, what the user rejected, and the approved version. The guide has more on memory.
