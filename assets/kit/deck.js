// moki deck: slides in HTML. Each <section> in <main class="deck"> is a slide of
// 1280x720, or data-size="1080x1080". The slide scales to fit the window.
//
// Layouts, as a class on the section:
//   title    h1, p, small                      cover
//   split    two children side by side        text and a picture
//   cards    h2, p?, ul > li (i, b, text)     3 to 5 cards
//   stats    h2, ul > li (strong, text)       2 to 4 numbers
//   steps    h2, ol > li (b, text)            numbered points, in two columns from 4
//   compare  h2, two divs (h3, ul)            before and after
//   quote    blockquote, cite
//   full     no padding: an svg, image or frame fills the slide
// Layouts are only a start: page CSS wins over them. Colors: --deck-accent on .deck.
// data-footer="Name" on .deck puts the name and the slide number on each slide.
//
// Keys: → space step on, ← steps back, Home and End, G all slides, F full screen.
// A click or a swipe steps too. ?s=3 opens slide 3, ?overview=1 shows all slides.
// Print makes one page per slide. moki uses that for the PDF export, and show() for
// the zip of slide images. moki sets the export class on <html> while it exports.
;(() => {
  const deck = document.querySelector('.deck')
  const [W, H] = (deck.dataset.size || '1280x720').split('x').map(Number)
  const slides = [...deck.querySelectorAll(':scope > section')]
  const q = new URLSearchParams(location.search)

  const css = document.createElement('style')
  css.textContent = `
html, body { margin: 0; height: 100%; overflow: hidden; background: #e4e4e7; }
html.dark, html.dark body { background: #09090b; }
.deck { --deck-bg: #fff; --deck-hi: #09090b; --deck-mid: #52525b; --deck-dim: #a1a1aa; --deck-tile: #f4f4f5;
  --deck-line: #e4e4e7; --deck-accent: #2563eb; --deck-soft: #eff6ff;
  position: absolute; left: 50%; top: 50%; width: ${W}px; height: ${H}px;
  font-family: Inter, system-ui, sans-serif; color: var(--deck-hi); -webkit-font-smoothing: antialiased; }
html.dark .deck { --deck-bg: #18181b; --deck-hi: #fafafa; --deck-mid: #a1a1aa; --deck-dim: #71717a; --deck-tile: #222225;
  --deck-line: #2e2e33; --deck-accent: #60a5fa; --deck-soft: #1b2230; }
.deck > section { position: absolute; left: 0; top: 0; width: ${W}px; height: ${H}px; box-sizing: border-box; overflow: hidden; background: var(--deck-bg);
  opacity: 0; pointer-events: none; transform: translateX(20px); transition: opacity .28s, transform .28s; }
.deck > section.on { opacity: 1; pointer-events: auto; transform: none; }
.deck > section.past { transform: translateX(-20px); }
.deck-foot { position: absolute; left: 72px; right: 72px; bottom: 28px; display: flex; justify-content: space-between;
  font-size: 14px; color: var(--deck-dim); font-variant-numeric: tabular-nums; pointer-events: none; }
.deck-bar { position: fixed; left: 0; bottom: 0; height: 3px; background: #2563eb; transition: width .28s; }
html.dark .deck-bar { background: #60a5fa; }

:where(.deck > section) { padding: 64px 72px 84px; display: flex; flex-direction: column; }
:where(.deck) :where(h1) { margin: 0; font-size: 76px; line-height: 1.02; letter-spacing: -.035em; font-weight: 700; text-wrap: balance; }
:where(.deck) :where(h2) { margin: 0; font-size: 44px; line-height: 1.1; letter-spacing: -.028em; font-weight: 650; text-wrap: balance; }
:where(.deck) :where(h3) { margin: 0; font-size: 22px; font-weight: 600; }
:where(.deck) :where(p, li) { margin: 0; font-size: 22px; line-height: 1.45; color: var(--deck-mid); }
:where(.deck) :where(h2 + p) { margin-top: 12px; max-width: 900px; }
:where(.deck) :where(small) { font-size: 16px; color: var(--deck-dim); }
:where(.deck) :where(b) { color: var(--deck-hi); font-weight: 600; }
:where(.deck) :where(ul, ol) { margin: 0; padding: 0; list-style: none; }

:where(.deck .title) { justify-content: center; }
:where(.deck .title) :where(h1) { max-width: 900px; }
:where(.deck .title) :where(p) { margin-top: 20px; max-width: 760px; font-size: 26px; }
:where(.deck .title) :where(small) { margin-top: 36px; }

:where(.deck .split) { display: grid; grid-template-columns: 1fr 1fr; gap: 56px; align-items: center; }
:where(.deck .split) :where(h2 + p) { margin-top: 16px; }

:where(.deck .cards, .deck .steps, .deck .stats, .deck .compare) { justify-content: center; }
:where(.deck .cards) :where(ul) { margin-top: 40px; display: grid; grid-auto-flow: column; grid-auto-columns: 1fr; gap: 16px; }
:where(.deck .cards) :where(li) { padding: 28px 24px; border-radius: 16px; background: var(--deck-tile); font-size: 19px; line-height: 1.4; }
:where(.deck .cards) :where(li i) { width: 44px; height: 44px; margin-bottom: 28px; display: grid; place-items: center;
  border-radius: 12px; background: var(--deck-bg); color: var(--deck-accent); font-style: normal; }
:where(.deck .cards) :where(li i svg) { width: 22px; height: 22px; fill: none; stroke: currentColor; stroke-width: 1.7;
  stroke-linecap: round; stroke-linejoin: round; }
:where(.deck .cards) :where(li b) { display: block; margin-bottom: 4px; font-size: 24px; }

:where(.deck .stats) :where(ul) { margin-top: 56px; display: grid; grid-auto-flow: column; grid-auto-columns: 1fr; gap: 40px; }
:where(.deck .stats) :where(li) { padding-top: 20px; border-top: 2px solid var(--deck-line); font-size: 20px; line-height: 1.4; }
:where(.deck .stats) :where(strong) { display: block; margin-bottom: 8px; font-size: 88px; line-height: 1; letter-spacing: -.045em;
  font-weight: 700; color: var(--deck-hi); font-variant-numeric: tabular-nums; }
:where(.deck .stats li:first-child) { border-color: var(--deck-accent); }
:where(.deck .stats li:first-child strong) { color: var(--deck-accent); }

:where(.deck .steps) :where(ol) { margin-top: 40px; display: grid; gap: 28px 56px; counter-reset: step; }
:where(.deck .steps ol:has(> li:nth-child(4))) { grid-template-columns: 1fr 1fr; }
:where(.deck .steps) :where(li) { position: relative; padding-left: 60px; counter-increment: step; font-size: 20px; }
:where(.deck .steps) :where(li)::before { content: counter(step); position: absolute; left: 0; top: 0; width: 40px; height: 40px;
  display: grid; place-items: center; border-radius: 20px; background: var(--deck-soft); color: var(--deck-accent);
  font-size: 18px; font-weight: 650; }
:where(.deck .steps) :where(li b) { display: block; margin-bottom: 2px; font-size: 22px; }

:where(.deck .compare) { display: grid; grid-template-columns: 1fr 1fr; align-content: center; gap: 0 16px; }
:where(.deck .compare) :where(h2 + p) { grid-column: 1 / -1; }
:where(.deck .compare) :where(h2) { grid-column: 1 / -1; margin-bottom: 40px; }
:where(.deck .compare) :where(h2 + p) { margin: -28px 0 40px; }
:where(.deck .compare > div) { padding: 28px; border-radius: 16px; background: var(--deck-tile); }
:where(.deck .compare > div + div) { background: var(--deck-soft); }
:where(.deck .compare) :where(h3) { margin-bottom: 16px; }
:where(.deck .compare) :where(li) { padding: 14px 0; border-top: 1px solid var(--deck-line); font-size: 20px; }

:where(.deck .quote) { justify-content: center; padding-left: 120px; padding-right: 120px; }
:where(.deck .quote) :where(blockquote) { margin: 0; padding-left: 36px; border-left: 4px solid var(--deck-accent);
  font-size: 48px; line-height: 1.18; letter-spacing: -.022em; font-weight: 600; color: var(--deck-hi); text-wrap: balance; }
:where(.deck .quote) :where(cite) { margin-top: 28px; padding-left: 40px; font-style: normal; font-size: 20px; color: var(--deck-mid); }

:where(.deck .full) { padding: 0; }

.deck.overview { position: fixed; inset: 0; width: auto; height: auto; transform: none !important; overflow: auto; }
.deck.overview > section { opacity: 1; pointer-events: auto; transition: none; transform-origin: 0 0; cursor: pointer;
  border-radius: 20px; box-shadow: 0 0 0 1px rgb(0 0 0 / .06); }
.deck.overview > section.on { box-shadow: 0 0 0 6px #2563eb; }
html.export .deck-bar { display: none; }
html.export .deck > section { transition: none; }

@media print {
  @page { size: ${W}px ${H}px; margin: 0; }
  html, body { height: auto; overflow: visible; background: none; }
  .deck { position: static; transform: none !important; height: auto; }
  .deck > section { position: relative; height: ${H}px; opacity: 1; transform: none !important; break-after: page; transition: none; }
  .deck-bar { display: none; }
}`
  document.head.prepend(css)

  // the footer: the deck name and the slide number, on each slide but the first
  const name = deck.dataset.footer
  slides.forEach((s, i) => {
    if (!name || i === 0 || s.classList.contains('full')) return
    s.insertAdjacentHTML('beforeend', `<footer class="deck-foot"><span>${name}</span><span>${i + 1} / ${slides.length}</span></footer>`)
  })
  const bar = Object.assign(document.createElement('div'), { className: 'deck-bar' })
  document.body.append(bar)

  let at = 0
  let overview = false

  function fit() {
    if (overview) return layoutOverview()
    deck.style.transform = `translate(-50%, -50%) scale(${Math.min(innerWidth / W, innerHeight / H)})`
  }

  function show(n) {
    at = Math.max(0, Math.min(slides.length - 1, n))
    slides.forEach((s, i) => {
      s.classList.toggle('on', i === at)
      s.classList.toggle('past', i < at)
    })
    bar.style.width = `${((at + 1) / slides.length) * 100}%`
  }

  // the overview lays all slides out in a grid, scaled down
  function layoutOverview() {
    const gap = 16
    const cols = innerWidth < 640 ? 2 : Math.min(4, Math.ceil(Math.sqrt(slides.length)))
    const cell = (innerWidth - gap * (cols + 1)) / cols
    const scale = cell / W
    slides.forEach((s, i) => {
      const x = gap + (i % cols) * (cell + gap)
      const y = gap + Math.floor(i / cols) * (H * scale + gap)
      s.style.transform = `translate(${x}px, ${y}px) scale(${scale})`
    })
  }

  function toggleOverview(on = !overview) {
    overview = on
    deck.classList.toggle('overview', on)
    bar.hidden = on
    if (!on) slides.forEach((s) => (s.style.transform = ''))
    fit()
  }

  const keys = { ArrowRight: 1, ArrowDown: 1, ' ': 1, PageDown: 1, ArrowLeft: -1, ArrowUp: -1, PageUp: -1 }

  addEventListener('keydown', (e) => {
    if (e.metaKey || e.ctrlKey || e.altKey) return
    if (overview && (e.key === 'Escape' || e.key === 'Enter')) toggleOverview(false)
    else if (e.key in keys) show(at + keys[e.key])
    else if (e.key === 'Home') show(0)
    else if (e.key === 'End') show(slides.length - 1)
    else if (e.key === 'g') toggleOverview()
    else if (e.key === 'f') document.fullscreenElement ? document.exitFullscreen() : document.documentElement.requestFullscreen?.()
    else return
    e.preventDefault() // the deck owns these keys, not the moki viewer
  })

  addEventListener('click', (e) => {
    if (e.target.closest('a, button, input, select, textarea')) return
    if (overview) {
      const s = e.target.closest('.deck > section')
      if (s) show(slides.indexOf(s)), toggleOverview(false)
      return
    }
    show(at + (e.clientX < innerWidth / 3 ? -1 : 1))
  })

  let touchX = 0
  addEventListener('touchstart', (e) => (touchX = e.touches[0].clientX), { passive: true })
  addEventListener('touchend', (e) => {
    const dx = e.changedTouches[0].clientX - touchX
    if (!overview && Math.abs(dx) > 40) show(at + (dx < 0 ? 1 : -1))
  })

  addEventListener('resize', fit)
  show((+q.get('s') || 1) - 1)
  toggleOverview(q.get('overview') === '1')

  // moki reads this to offer the right export: PDF and PNG of each slide
  window.moki = { kind: 'deck', size: `${W}x${H}`, slides: slides.length, show: (n) => show(n - 1), at: () => at + 1 }
})()
