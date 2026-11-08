---
name: moki-design
description: How to design UI mockups for moki: taste, page layout, real-looking data and self-review. Read before you make or change a mockup in moki.
---

# moki design skill

This skill is guidance, not a format. moki shows any page you make. Use what helps the user decide, and leave out the rest.

Mockup first: before you change UI, refactor a screen or design a new page, make the mockup and get the user's OK. Read this skill again at the start of each chat.

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

## Theme

- Do not add a light/dark toggle to your page. The moki bar has one, and it switches your page too.
- Style dark mode with the `dark` class on `<html>`. moki sets that class from its own theme.
- Load `/kit/theme.js` so that the page also follows the moki theme when it is opened alone.
- Frames: pass the theme on with `?dark=1` or `?dark=0`. `/kit/theme.js` in the frame page reads it.

## Data

- Make the data look real: names, usernames, numbers, dates, avatars. Use the data of the user's product when you can get it.
- Never use lorem ipsum.
- Generated avatars, inline SVG and images from other hosts are all fine.
- Say in the intro which data is real and which is made up.

## Check your work

1. Upload, then read the sheet and the warnings that come back.
2. Call `screenshot` with sizes desktop and phone and `themes: "both"`. Add `full: true` for long pages.
3. Check each screen:
   - One primary button. No eyebrow, no repeated fact, no card in a card.
   - Color only for meaning. Radius and inset line up.
   - No sideways scroll on the phone. Long text truncates.
   - Empty, loading and error states exist when the screen needs them.
4. Fix what is wrong before you send the URL.

## Rounds

- Each round of feedback is a new version.
- Keep the options that the user liked. Drop the ones the user rejected.
- Do not change a version that the user approved.
- Write the decisions to memory with `write_memory` and `append: true`.
