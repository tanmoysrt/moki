// Version viewer. moki shows the version index.html as the agent made it, in a same-origin
// iframe. moki adds only the bar: back, title, versions, theme and open in a new tab.

function viewer() {
  return `<div class="viewer">
    <div class="vbar">${viewerBar()}</div>
    <div class="canvas">${pageFrame()}</div>
    <div class="stepper num">${stepper()}</div>
  </div>`
}

function viewerBar() {
  const { slug, ver } = app.route
  const { title, list } = app.detail[slug]
  return `<a class="ib" href="#/" aria-label="Back" title="Back (Esc)">${icon('back')}</a>
    <div class="vtitle trunc">${esc(title || slug)}</div>
    ${list.length > 1 ? `<div class="vers num">${list.slice().reverse().map(pill).join('')}</div>` : ''}
    <div class="sp"></div>
    ${themeButton()}
    <a class="ib" href="${pageURL(slug, ver)}" target="_blank" aria-label="Open in a new tab" title="Open in a new tab">
      ${icon('ext')}
    </a>`
}

function pill(v) {
  const { slug, ver } = app.route
  const classes = []
  if (v.version === ver) classes.push('on')
  if (app.fresh[slug]?.[v.version]) classes.push('new')
  const tip = [v.note || v.title, ago(v.time)].filter(Boolean).join(' · ')
  return `<button class="${classes.join(' ')}" data-ver="${v.version}" title="${esc(tip)}">${v.version}</button>`
}

// stepper replaces the pills on a phone.
function stepper() {
  const { slug, ver } = app.route
  const { list } = app.detail[slug]
  if (list.length < 2) return ''
  const i = list.findIndex((v) => v.version === ver)
  const unseen = Object.keys(app.fresh[slug] || {}).length > 0
  return `<button class="ib" data-step="-1" aria-label="Older version" ${i === list.length - 1 ? 'disabled' : ''}>
      ${icon('left')}
    </button>
    <span>${ver} of ${list[0].version}</span>
    ${unseen ? '<i class="dot"></i>' : ''}
    <button class="ib" data-step="1" aria-label="Newer version" ${i === 0 ? 'disabled' : ''}>
      ${icon('right')}
    </button>`
}

function pageFrame(scroll = 0) {
  const { slug, ver } = app.route
  return `<iframe src="${pageURL(slug, ver)}" title="${esc(slug)} ${ver}" data-scroll="${scroll}" onload="pageReady(this)"></iframe>`
}

const viewerTitle = () => `${app.detail[app.route.slug].title || app.route.slug} ${app.route.ver} · moki`

// showVersion updates the open viewer to app.route without a render: the bar, the stepper,
// and the page. The new page loads hidden on top of the old one, at the same scroll
// position, and fades in when it is ready. reload swaps the page also when its URL is the same.
function showVersion(reload = false) {
  document.querySelector('.vbar').innerHTML = viewerBar()
  document.querySelector('.stepper').innerHTML = stepper()
  document.title = viewerTitle()
  revealPill()

  const canvas = document.querySelector('.canvas')
  const shown = [...canvas.querySelectorAll('iframe.ready')].pop()
  if (!reload && shown?.getAttribute('src') === pageURL(app.route.slug, app.route.ver)) return
  canvas.querySelectorAll('iframe:not(.ready)').forEach((f) => f.remove()) // still loading, and out of date
  canvas.insertAdjacentHTML('beforeend', pageFrame(pageScroll(shown)))
  canvas.lastElementChild.classList.add('incoming')
}

// pageReady runs when a page loads in the viewer. The page gets the moki theme, the scroll
// position of the page before it, and the moki keys. Then it fades in over the old page.
function pageReady(frame) {
  const scroll = Number(frame.dataset.scroll)
  frame.dataset.scroll = 0 // a link inside the page opens a new document at the top
  setFrameTheme(frame)
  try {
    if (scroll) frame.contentWindow.scrollTo(0, scroll)
    frame.contentDocument.addEventListener('keydown', forwardKey)
  } catch (e) {}
  frame.classList.add('ready')
  removeOlder(frame, 200)
}

function pageScroll(frame) {
  try {
    return Math.round(frame.contentWindow.scrollY)
  } catch (e) {
    return 0
  }
}

// forwardKey lets the arrow keys and Esc work while the focus is in the page,
// unless the page uses the key: in a form field, or when it calls preventDefault.
function forwardKey(e) {
  const own = e.defaultPrevented || e.target.closest?.('input, textarea, select, [contenteditable]')
  const modified = e.metaKey || e.ctrlKey || e.altKey
  if (own || modified || !['Escape', 'ArrowLeft', 'ArrowRight'].includes(e.key)) return
  document.dispatchEvent(new KeyboardEvent('keydown', { key: e.key }))
}

function revealPill() {
  document.querySelector('.vers .on')?.scrollIntoView({ block: 'nearest', inline: 'nearest' })
}

function step(direction) {
  const { slug, ver } = app.route
  const list = app.detail[slug].list
  const next = list[list.findIndex((v) => v.version === ver) - direction]
  if (next) location.hash = viewerHash(slug, next.version)
}

// followChanges applies version events to the open viewer. On the newest version the
// viewer follows a new one. On an older version it stays, and the new pill gets a dot.
// A change to the version on screen swaps the page in place.
async function followChanges(events) {
  const { slug, ver } = app.route
  const mine = events.filter((e) => e.slug === slug)
  if (!mine.length || !app.detail[slug]) return

  const newestBefore = app.detail[slug].list[0].version
  const detail = await loadProject(slug, true)
  if (app.route.slug !== slug) return
  if (!detail) return location.replace('#/')

  const fresh = (app.fresh[slug] ||= {})
  for (const e of mine) if (versionNumber(e.version) > versionNumber(newestBefore)) fresh[e.version] = true
  for (const v in fresh) if (!detail.list.some((x) => x.version === v)) delete fresh[v]

  const newest = detail.list[0].version
  if (!detail.list.some((v) => v.version === ver)) return location.replace(viewerHash(slug, newest))
  if (ver === newestBefore && newest !== ver) {
    location.hash = viewerHash(slug, newest)
    return
  }
  showVersion(mine.some((e) => e.version === ver))
}
