// moki UI: one state object, hash routes, one render, event delegation.
// Routes: #/ is the projects home, #/p/<slug>/<version> is the viewer.
// Data comes from /api/projects and /api/projects/<slug>. /api/events says when a version changes.
// A public link, /s/<id>/#/<version>, shows only the viewer of one project, from /s/<id>/api,
// with no password and no live updates.

const share = location.pathname.match(/^\/s\/([a-z2-7]+)\//)?.[1] || ''

const app = {
  dark: document.documentElement.classList.contains('dark'),
  projects: [], // newest first
  agent: '', // the agent that connected last, '' before the first one
  detail: {}, // slug -> project with its versions in `list`, newest first
  fresh: {}, // slug -> {v4: true}: versions that arrived while the user looked at an older one
  route: { name: 'home' },
  query: '',
  dialog: false,
  linkOpen: false, // the card of the viewer padlock
  down: false, // moki does not answer
  homeScroll: 0,
}

async function start() {
  if (share) return route()
  try {
    await Promise.all([loadProjects(), loadAgent()])
  } catch (e) {
    app.down = true
  }
  listen()
  route()
  setInterval(updateAges, 30_000)
}

// ---------- routes and render ----------

function parseRoute() {
  const parts = location.hash.split('/')
  if (share) return { name: 'view', slug: share, ver: parts[1] || '' }
  const [, kind, slug, ver = ''] = parts
  return kind === 'p' && slug ? { name: 'view', slug, ver } : { name: 'home' }
}

async function route() {
  const prev = app.route
  const next = parseRoute()
  if (prev.name === 'home' && next.name === 'view') app.homeScroll = scrollY
  app.route = next
  if (next.slug !== prev.slug) app.linkOpen = false

  if (next.name === 'view' && !app.down) {
    let detail
    try {
      detail = await loadProject(next.slug)
    } catch (e) {
      return setDown()
    }
    if (app.route !== next) return // the user went on while it loaded
    if (!detail?.list.length) return share ? showLinkOff() : location.replace('#/')
    if (!detail.list.some((v) => v.version === next.ver)) {
      return location.replace(viewerHash(next.slug, detail.list[0].version))
    }
    delete app.fresh[next.slug]?.[next.ver]
    if (prev.name === 'view' && prev.slug === next.slug && document.querySelector('.viewer')) return showVersion()
  }
  render()
}

function render() {
  const viewing = app.route.name === 'view' && !app.down
  if (app.down) document.body.innerHTML = (share ? '' : topBar()) + downNote()
  else if (viewing) document.body.innerHTML = viewer()
  else document.body.innerHTML = topBar() + home()
  if (app.dialog) document.body.insertAdjacentHTML('beforeend', connectDialog())
  document.title = viewing ? viewerTitle() : 'moki'

  if (viewing) return revealPill()
  filter()
  scrollTo(0, app.homeScroll)
}

const downNote = () =>
  `<main class="wrap"><p class="none">moki does not answer. ${share ? 'Try again later.' : 'This page updates when moki is back.'}</p></main>`

// ---------- data ----------

async function getJSON(url) {
  const res = await fetch(url)
  if (res.status === 404) return null
  if (!res.ok) throw new Error(`${url}: ${res.status}`)
  return res.json()
}

async function loadProjects() {
  const list = await getJSON('/api/projects')
  app.projects = list.sort((a, b) => new Date(b.time) - new Date(a.time))
}

async function loadAgent() {
  app.agent = (await getJSON('/api/agent'))?.name || ''
}

async function loadProject(slug, reload = false) {
  if (app.detail[slug] && !reload) return app.detail[slug]
  const p = await getJSON(share ? `/s/${share}/api` : `/api/projects/${encodeURIComponent(slug)}`)
  if (!p) {
    delete app.detail[slug]
    return null
  }
  p.list.sort((a, b) => versionNumber(b.version) - versionNumber(a.version))
  return (app.detail[slug] = p)
}

const project = (slug) => app.projects.find((p) => p.slug === slug)
const versionNumber = (ver) => Number(ver.slice(1))
const pageURL = (slug, ver) => (share ? `/s/${share}/${ver}/index.html` : `/p/${slug}/${ver}/index.html`)
const viewerHash = (slug, ver) => (share ? `#/${ver}` : `#/p/${slug}/${ver}`)

// ---------- live update ----------
// The server sends "change" when a version is new, changed or gone, "thumb" when its
// thumbnail is ready, and "share" when a project goes public or private.
// EventSource reconnects by itself when moki restarts.

function listen() {
  const events = new EventSource('/api/events')
  let lost = false
  events.onerror = () => {
    if (lost) return
    lost = true
    queueRefresh()
  }
  events.onopen = () => {
    if (!lost) return
    lost = false
    queueRefresh()
  }
  events.addEventListener('change', (e) => queueRefresh(JSON.parse(e.data)))
  events.addEventListener('thumb', () => queueRefresh())
  events.addEventListener('agent', () => queueRefresh())
  events.addEventListener('share', (e) => queueRefresh(JSON.parse(e.data)))
}

let queued = []
let refreshTimer = 0

// queueRefresh collects events for 100 ms, so a burst of events makes one refresh.
function queueRefresh(event) {
  if (event) queued.push(event)
  clearTimeout(refreshTimer)
  refreshTimer = setTimeout(refresh, 100)
}

async function refresh() {
  const events = queued
  const before = app.projects
  queued = []
  try {
    await Promise.all([loadProjects(), loadAgent()])
  } catch (e) {
    return setDown()
  }
  if (app.down) {
    app.down = false
    app.detail = {}
    return route()
  }
  for (const e of events) if (e.slug !== app.route.slug) delete app.detail[e.slug]

  if (app.route.name === 'view') return followChanges(events).catch(setDown)
  if (!before.length || !app.projects.length || !document.querySelector('.grid')) return render()
  syncGrid(before)
}

function setDown() {
  if (app.down) return
  app.down = true
  render()
}

function updateAges() {
  document.querySelectorAll('[data-time]').forEach((el) => (el.textContent = ago(el.dataset.time)))
}

function ago(time) {
  const minutes = Math.round((Date.now() - new Date(time)) / 60_000)
  if (minutes < 1) return 'just now'
  if (minutes < 60) return `${minutes} min ago`
  if (minutes < 60 * 24) return `${Math.round(minutes / 60)} h ago`
  return new Date(time).toLocaleDateString('en', { month: 'short', day: 'numeric' })
}

// ---------- actions ----------

const actions = {
  theme: toggleTheme,
  connect: openConnect,
  close: closeConnect,
  link: toggleLink,
}

async function copy(button) {
  const text = button.dataset.copy
  try {
    await navigator.clipboard.writeText(text)
  } catch (e) {
    copyWithSelection(text) // the clipboard API needs https or localhost
  }
  const html = button.innerHTML
  button.innerHTML = icon('check') + (button.textContent.trim() ? 'Copied' : '')
  setTimeout(() => (button.innerHTML = html), 1200)
}

function copyWithSelection(text) {
  const area = document.createElement('textarea')
  area.value = text
  document.body.append(area)
  area.select()
  document.execCommand('copy')
  area.remove()
}

document.addEventListener('click', (e) => {
  if (app.linkOpen && !e.target.closest('.pop, [data-act="link"]')) toggleLink() // a click outside the card
  const target = e.target.closest('[data-act], [data-ver], [data-step], [data-copy]')
  if (!target) return
  if (target.classList.contains('scrim') && e.target !== target) return // a click inside the dialog
  if (target.dataset.act) actions[target.dataset.act]()
  else if (target.dataset.ver) location.hash = viewerHash(app.route.slug, target.dataset.ver)
  else if (target.dataset.step) step(Number(target.dataset.step))
  else if (target.dataset.copy) copy(target)
})

document.addEventListener('input', (e) => {
  if (e.target.id !== 'q') return
  app.query = e.target.value
  filter()
})

document.addEventListener('keydown', (e) => {
  const typing = e.target instanceof Element && e.target.matches('input, textarea, select')
  if (e.key === 'Escape') {
    if (app.dialog) return actions.close()
    if (app.linkOpen) return toggleLink()
    if (typing) return e.target.blur()
    if (app.route.name === 'view' && !share) location.hash = '#/'
    return
  }
  if (typing || e.metaKey || e.ctrlKey || e.altKey) return

  const search = document.getElementById('q')
  if (e.key === '/' && search) {
    e.preventDefault()
    search.focus()
  }
  if (app.route.name !== 'view') return
  if (e.key === 'ArrowLeft') step(-1)
  if (e.key === 'ArrowRight') step(1)
})

addEventListener('hashchange', route)

// ---------- helpers ----------

function esc(value) {
  return String(value ?? '').replace(/[&<>"']/g, (c) => `&#${c.charCodeAt(0)};`)
}

function htmlElement(html) {
  const template = document.createElement('template')
  template.innerHTML = html.trim()
  return template.content.firstElementChild
}

// replay restarts a CSS animation.
function replay(el, name) {
  el.classList.remove(name)
  void el.offsetWidth
  el.classList.add(name)
}

// removeOlder removes the siblings before el after a fade of ms.
function removeOlder(el, ms) {
  const older = []
  for (let x = el.previousElementSibling; x; x = x.previousElementSibling) older.push(x)
  setTimeout(() => older.forEach((x) => x.remove()), ms)
}

start()
