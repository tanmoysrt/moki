// Options. A version with options.json holds many ideas for each part of a round. The
// viewer shows them as a grid at the version root, and each option at full size on its
// own page, with a bar to step through the part, its states, and Keep. The agent reads
// the kept options in get_project. Keys: G the grid, [ and ] the options, K keep.

const optionsCache = {} // "slug/v2" -> the options of the version, or null

async function loadOptions(slug, ver, reload = false) {
  const key = slug + '/' + ver
  if (key in optionsCache && !reload) return
  const has = app.detail[slug]?.list.find((v) => v.version === ver)?.options
  let opts = null
  try {
    if (has) opts = await (await fetch(pageURL(slug, ver, 'options.json'), { cache: 'no-store' })).json()
  } catch (e) {} // the viewer shows index.html
  optionsCache[key] = opts?.parts?.length ? opts : null
}

const versionOptions = () => optionsCache[app.route.slug + '/' + app.route.ver] || null
const gridMode = () => Boolean(versionOptions()) && !app.route.page
const isKept = (part, id) => keptOf().some((k) => k.part === part && k.option === id)

// optionAt is the option on screen: its part, its index in the part, and the state.
function optionAt(page = app.route.page || 'index.html') {
  for (const part of versionOptions()?.parts || []) {
    for (const [i, opt] of part.options.entries()) {
      if (opt.page === page) return { part, opt, i, state: null }
      const state = (opt.states || []).find((s) => s.page === page)
      if (state) return { part, opt, i, state }
    }
  }
  return null
}

// optionName names the option of a page for the notes list: "Plans page B".
function optionName(page) {
  const at = optionAt(page)
  return at ? `${at.part.title} ${at.opt.id}${at.state ? ', ' + at.state.name : ''}` : ''
}

// ---------- the grid ----------

function optionGrid() {
  return `<div class="board">${versionOptions().parts.map(partSection).join('')}</div>`
}

function partSection(part) {
  return `<section><h2>${esc(part.title)}<span>${partCount(part)}</span></h2>
    <div class="ogrid">${part.options.map((o) => optionCard(part, o)).join('')}</div></section>`
}

function partCount(part) {
  const kept = part.options.filter((o) => isKept(part.title, o.id)).length
  return `${part.options.length} options${kept ? ` · ${kept} kept` : ''}`
}

function optionCard(part, o) {
  const { slug, ver } = app.route
  const [w, h] = (part.size || '1280x800').split('x')
  const kept = isKept(part.title, o.id)
  return `<div class="ocard${kept ? ' on' : ''}">
    <div class="shot" data-w="${w}" data-h="${h}">
      <iframe src="${esc(pageURL(slug, ver, o.page))}" width="${w}" height="${h}" loading="lazy" tabindex="-1" title="${esc(o.name)}" onload="cardReady(this)"></iframe>
    </div>
    <div class="meta"><div><b><i>${esc(o.id)}</i>${esc(o.name)}</b>
      <p>${o.rec ? '<em>Recommended</em>' : ''}${o.rec && o.note ? ' · ' : ''}${esc(o.note || '')}</p></div>
      ${keepButton(part, o)}</div>
    <a class="ohit" href="${viewerHash(slug, ver, o.page)}" aria-label="Open ${esc(o.id + ' ' + o.name)}"></a>
  </div>`
}

// keepButton keeps or drops an option. A visitor sees only what is kept.
function keepButton(part, o) {
  const kept = isKept(part.title, o.id)
  if (share && !kept) return ''
  return `<button class="okeep${kept ? ' on' : ''}" data-keep="${esc(o.id)}" data-part="${esc(part.title)}" data-name="${esc(o.name)}"
    ${share ? 'disabled' : ''} title="${kept ? 'Kept. Drop it' : 'Keep it for the next round'} (K)">${kept ? icon('check') + 'Kept' : 'Keep'}</button>`
}

// cardReady gives a card the theme, and the size of a deck or a video page.
function cardReady(frame) {
  setFrameTheme(frame)
  try {
    const size = frame.contentWindow.moki?.size
    if (size) [frame.parentElement.dataset.w, frame.parentElement.dataset.h] = size.split('x')
  } catch (e) {}
  fitCards()
}

function fitCards() {
  document.querySelectorAll('.board .shot').forEach((shot) => {
    const f = shot.firstElementChild
    const k = shot.clientWidth / shot.dataset.w
    f.width = shot.dataset.w
    f.height = shot.dataset.h
    shot.style.height = shot.dataset.h * k + 'px'
    f.style.transform = `scale(${k})`
  })
}

addEventListener('resize', fitCards)

// ---------- one option, in the bar ----------

function optionNav() {
  const at = optionAt()
  if (!at) return ''
  const { slug, ver } = app.route
  const { part, opt, i, state } = at
  const n = part.options.length
  const states = opt.states?.length
    ? `<div class="seg">${[{ name: 'Default', page: opt.page }, ...opt.states]
        .map((s) => `<a class="${(state || { page: opt.page }).page === s.page ? 'on' : ''}" href="${viewerHash(slug, ver, s.page)}">${esc(s.name)}</a>`)
        .join('')}</div>`
    : ''
  return `<div class="onav">
    <a class="ib" href="${viewerHash(slug, ver)}" aria-label="All options" title="All options (G)">${icon('grid')}</a>
    <div class="oname"><span class="trunc">${esc(part.title)}</span>${icon('right')}<i class="tag">${esc(opt.id)}</i><b class="trunc">${esc(opt.name)}</b></div>
    <button class="ib" data-step-opt="-1" aria-label="Previous option" title="Previous option ([)" ${i === 0 ? 'disabled' : ''}>${icon('left')}</button>
    <span class="ocount num">${i + 1} of ${n}</span>
    <button class="ib" data-step-opt="1" aria-label="Next option" title="Next option (])" ${i === n - 1 ? 'disabled' : ''}>${icon('right')}</button>
    ${states}${keepButton(part, opt)}
  </div>`
}

function stepOption(direction) {
  const at = optionAt()
  const next = at?.part.options[at.i + direction]
  if (next) location.hash = viewerHash(app.route.slug, app.route.ver, next.page)
}

function toggleKeep(part, id, name) {
  if (share) return
  sendNotes('keep', 'POST', { version: app.route.ver, part, option: id, name, on: !isKept(part, id) })
}

// renderKept shows what is kept after a change, in place, so the cards do not reload.
function renderKept() {
  const parts = versionOptions()?.parts || []
  document.querySelectorAll('.board section').forEach((sec, i) => {
    const part = parts[i]
    sec.querySelector('h2 span').textContent = partCount(part)
    sec.querySelectorAll('.ocard').forEach((card, j) => {
      const o = part.options[j]
      card.classList.toggle('on', isKept(part.title, o.id))
      card.querySelector('.okeep')?.remove()
      card.querySelector('.meta').insertAdjacentHTML('beforeend', keepButton(part, o))
    })
  })
  const nav = document.querySelector('.onav')
  if (nav) nav.outerHTML = optionNav()
}

// optionKey runs G, [, ] and K in the viewer. It is true when the key was an option key.
function optionKey(key) {
  if (!versionOptions()) return false
  const at = optionAt()
  if (key === 'g' && app.route.page) location.hash = viewerHash(app.route.slug, app.route.ver)
  else if ((key === '[' || key === ']') && at) stepOption(key === '[' ? -1 : 1)
  else if (key === 'k' && at) toggleKeep(at.part.title, at.opt.id, at.opt.name)
  else return false
  return true
}

document.addEventListener('click', (e) => {
  const keep = e.target.closest('[data-keep]')
  if (keep) return toggleKeep(keep.dataset.part, keep.dataset.keep, keep.dataset.name)
  const step = e.target.closest('[data-step-opt]')
  if (step) stepOption(Number(step.dataset.stepOpt))
})
