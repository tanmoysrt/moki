// Projects home: the top bar, the gallery and the first-run card.

function topBar() {
  const search = app.projects.length > 0
  return `<header class="top">
    <a class="brand" href="#/">${LOGO}moki</a>
    ${search ? searchBox() : ''}
    <div class="sp"></div>
    ${themeButton()}
    <button class="btn connect" data-act="connect" aria-label="Connect an agent">
      ${icon('plug')}<span class="label">Connect agent</span>
    </button>
  </header>`
}

function searchBox() {
  return `<label class="search">
    ${icon('search')}
    <input id="q" placeholder="Search projects" value="${esc(app.query)}" autocomplete="off" spellcheck="false">
    <kbd>/</kbd>
  </label>`
}

const themeButton = () =>
  `<button class="ib" data-act="theme" aria-label="Theme" title="Theme">${icon(app.dark ? 'sun' : 'moon')}</button>`

function home() {
  if (!app.projects.length) return emptyState()
  return `<main class="wrap">
    <h1>Projects</h1>
    <div class="grid">${app.projects.map(tile).join('')}</div>
    <p class="none" hidden></p>
  </main>`
}

function tile(p) {
  return `<a class="card tile" href="${viewerHash(p.slug, p.version)}" data-slug="${esc(p.slug)}">
    <div class="thumb">${thumbHTML(p)}${versionTag(p)}</div>
    <div class="meta">${tileMeta(p)}</div>
  </a>`
}

const versionTag = (p) => (p.versions > 1 ? `<span class="vtag num">${p.version}</span>` : '')

function tileMeta(p) {
  return `<div class="title">
      <b class="trunc">${esc(p.title || p.slug)}</b>
      <small class="age num" data-time="${p.time}">${ago(p.time)}</small>
    </div>
    <small class="desc trunc">${esc(p.description)}</small>`
}

// updateTile changes a card in place, so its thumbnail can fade instead of flash.
function updateTile(el, p) {
  el.href = viewerHash(p.slug, p.version)
  el.querySelector('.meta').innerHTML = tileMeta(p)
  const box = el.querySelector('.thumb')
  box.querySelector('.vtag')?.remove()
  box.insertAdjacentHTML('beforeend', versionTag(p))
  setThumb(box, p)
}

// syncGrid applies a new project list to the gallery: it updates the cards, moves the
// changed ones to the top with a short animation, and drops the deleted ones.
function syncGrid(before) {
  const grid = document.querySelector('.grid')
  const old = Object.fromEntries(before.map((p) => [p.slug, p]))
  app.projects.forEach((p, i) => {
    let el = grid.querySelector(`[data-slug="${CSS.escape(p.slug)}"]`)
    if (el) updateTile(el, p)
    else el = htmlElement(tile(p))
    if (grid.children[i] !== el) grid.insertBefore(el, grid.children[i] || null)
    const o = old[p.slug]
    if (!o || o.version !== p.version || o.time !== p.time) replay(el, 'arrive')
  })
  const slugs = new Set(app.projects.map((p) => p.slug))
  for (const el of [...grid.children]) if (!slugs.has(el.dataset.slug)) el.remove()
  filter()
}

// filter hides the cards that do not match the search, without a render.
function filter() {
  const grid = document.querySelector('.grid')
  if (!grid) return
  const query = app.query.trim().toLowerCase()
  let shown = 0
  for (const el of grid.children) {
    const p = project(el.dataset.slug)
    const hit = !query || `${p.title} ${p.slug} ${p.description}`.toLowerCase().includes(query)
    el.hidden = !hit
    if (hit) shown++
  }
  const none = document.querySelector('.none')
  none.hidden = shown > 0
  none.textContent = `No project matches "${app.query}".`
}

// ---------- thumbnails ----------
// A card shows the cached thumbnail in the current theme. Without one (Chrome is missing,
// or the thumbnail is not made yet) it shows the live page at 25%.

function thumbHTML(p) {
  const src = app.dark ? p.thumb_dark : p.thumb
  if (src) return `<img src="${src}" alt="" data-key="${thumbKey(p)}" onload="showThumb(this)" onerror="thumbFailed(this)">`
  return liveThumbHTML(pageURL(p.slug, p.version), thumbKey(p))
}

function liveThumbHTML(url, key) {
  return `<iframe src="${url}" data-key="${esc(key)}" tabindex="-1" aria-hidden="true" scrolling="no" loading="lazy"
    onload="setFrameTheme(this); showThumb(this)"></iframe>`
}

function thumbKey(p) {
  return (app.dark ? p.thumb_dark : p.thumb) || `live:${p.slug}/${p.version}@${p.time}`
}

// setThumb loads the new thumbnail on top of the old one when it differs.
function setThumb(box, p) {
  const newest = [...box.querySelectorAll('[data-key]')].pop()
  if (newest?.dataset.key === thumbKey(p)) return
  box.insertBefore(htmlElement(thumbHTML(p)), box.querySelector('.vtag')) // the tag stays on top
}

// showThumb fades a loaded thumbnail in and removes the ones under it.
function showThumb(el) {
  el.classList.add('shown')
  removeOlder(el, 300)
}

function thumbFailed(img) {
  const p = project(img.closest('.tile').dataset.slug)
  img.replaceWith(htmlElement(liveThumbHTML(pageURL(p.slug, p.version), img.dataset.key)))
}

// ---------- first run ----------

function emptyState() {
  return `<main class="wrap">
    <section class="card empty">
      <div class="mark">${LOGO}</div>
      <h1>No mockups yet</h1>
      <p>Connect an agent, then ask it for a mockup.</p>
      <button class="btn primary" data-act="connect">${icon('plug')}Connect agent</button>
    </section>
  </main>`
}
