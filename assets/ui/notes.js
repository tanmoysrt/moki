// Notes in the viewer. With the list open, a click on the page pins a note on that spot.
// The agent reads the notes in get_project. moki draws the pins and the note box over
// the page, so the page stays as the agent made it. A pin keeps the element it is on
// (a CSS path) and the spot inside it, so it stays on the element when the page reflows.

const notesOf = () => app.detail[app.route.slug]?.notes || { notes: [] }
const versionNotes = () => notesOf().notes.filter((n) => n.version === app.route.ver)
const currentPage = () => app.route.page || 'index.html'
const touch = matchMedia('(pointer: coarse)').matches

function notesButton() {
  const count = versionNotes().length
  return `<button class="ib nb${app.notesOpen ? ' on' : ''}" data-act="notes" aria-label="Notes" title="Notes (N)">
    ${icon('note')}${count && !app.notesOpen ? `<b class="num">${count}</b>` : ''}
  </button>`
}

function notesPanel() {
  const { ver } = app.route
  const list = versionNotes()
  const approved = notesOf().approved === ver
  return `<div class="nhead"><b>Notes on ${ver}</b><span class="num">${list.length || ''}</span>
      <div class="sp"></div><button class="ib sm close" data-act="notes" aria-label="Close">${icon('x')}</button></div>
    <ul class="nlist">${list.map(noteRow).join('')}</ul>
    <p class="nhint">${touch ? 'Tap' : 'Click'} the page to pin a note.</p>
    <div class="nfoot">
      <button class="btn wide${approved ? ' done' : ''}" data-act="approve">${icon('check')}${approved ? `${ver} approved` : `Approve ${ver}`}</button>
    </div>`
}

function noteRow(n, i) {
  const where = [n.page, n.target].filter(Boolean).join(' · ')
  return `<li data-note="${n.id}"><i class="pin num">${i + 1}</i>
    <div class="ntext"><p>${esc(n.text)}</p><span class="trunc">${esc(where)}</span></div>
    <button class="ib sm" data-del="${n.id}" aria-label="Delete note">${icon('x')}</button></li>`
}

// renderNotes draws the bar button, the list and the pins for the route on screen.
function renderNotes() {
  const panel = document.querySelector('.npanel')
  if (!panel) return
  document.querySelector('.nb')?.replaceWith(htmlElement(notesButton()))
  panel.hidden = !app.notesOpen || (app.draft && innerWidth <= 640)
  if (app.notesOpen) panel.innerHTML = notesPanel()
  aimCursor()
  placePins()
}

function toggleNotes() {
  app.notesOpen = !app.notesOpen
  app.draft = null
  renderNotes()
}

// ---------- pins over the page ----------

function shownFrame() {
  return [...document.querySelectorAll('.canvas iframe.ready')].pop()
}

// placePins puts each pin of this page, and the draft, on its element. It runs on
// each scroll and resize of the page.
function placePins() {
  const layer = document.querySelector('.pins')
  if (!layer) return
  const doc = frameDoc(shownFrame())
  const show = app.notesOpen && doc
  const list = versionNotes()
  let html = ''
  if (show) {
    list.forEach((n, i) => {
      if (n.page !== currentPage()) return
      const at = spot(doc, n)
      if (at) html += `<i class="pin num" data-note="${n.id}" style="left:${at.x}px;top:${at.y}px">${i + 1}</i>`
    })
    const at = app.draft && spot(doc, app.draft)
    if (at) html += `<i class="pin num open" style="left:${at.x}px;top:${at.y}px">${list.length + 1}</i>`
  }
  layer.innerHTML = html
  placeDraft(show && app.draft && spot(doc, app.draft))
}

// spot is where a note sits on the page in the viewer. A note in a frame of the page,
// such as a board option, has a selector for each frame: "iframe >>> button".
function spot(doc, n) {
  const el = n.el?.isConnected ? n.el : findEl(doc, n.selector)
  const r = el && rectIn(el, doc)
  if (!r || (!r.width && !r.height)) return null
  return { x: r.left + n.x * r.width, y: r.top + n.y * r.height }
}

// notesFrame hooks a page that loaded in the viewer: clicks pin notes while the list
// is open, and the pins follow scroll and resize.
function notesFrame(frame) {
  if (share) return
  hookDoc(frameDoc(frame))
  frame.contentWindow.addEventListener('resize', placePins)
  aimCursor()
  placePins()
}

// hookDoc hooks a page and the frames in it, also the frames that load later.
function hookDoc(doc) {
  if (!doc || doc.mokiNotes) return
  doc.mokiNotes = true
  doc.addEventListener('click', pageClick, true)
  doc.addEventListener('scroll', placePins, { capture: true, passive: true })
  doc.addEventListener('load', (e) => e.target.tagName === 'IFRAME' && frameLoaded(e.target), true)
  doc.querySelectorAll('iframe').forEach(frameLoaded)
}

function frameLoaded(frame) {
  hookDoc(frameDoc(frame))
  aimCursor()
  placePins()
}

// pageDocs is the page on screen and the pages in its frames.
function pageDocs(doc = frameDoc(shownFrame())) {
  if (!doc) return []
  return [doc, ...[...doc.querySelectorAll('iframe')].flatMap((f) => pageDocs(frameDoc(f)))]
}

// aimCursor shows a crosshair on the page while a click pins a note.
function aimCursor() {
  for (const doc of pageDocs()) {
    if (!doc.head) continue
    let style = doc.getElementById('moki-aim')
    if (app.notesOpen && !style) {
      style = doc.createElement('style')
      style.id = 'moki-aim'
      style.textContent = '*, *::before, *::after { cursor: crosshair !important }'
      doc.head.append(style)
    }
    if (!app.notesOpen) style?.remove()
  }
}

function pageClick(e) {
  if (!app.notesOpen) return
  e.preventDefault()
  e.stopPropagation()
  // not instanceof Element: the page in the frame has its own Element
  const el = e.target.nodeType === 1 ? e.target : e.target.parentElement
  const r = el.getBoundingClientRect()
  app.draft = {
    el,
    page: currentPage(),
    selector: framePath(el),
    target: describe(el) + inFrame(el),
    x: r.width ? round((e.clientX - r.left) / r.width) : 0,
    y: r.height ? round((e.clientY - r.top) / r.height) : 0,
  }
  renderNotes()
}

// ---------- the note box ----------

function placeDraft(at) {
  let box = document.querySelector('.nbox')
  if (!app.draft) return box?.remove()
  if (!box) {
    box = htmlElement(`<div class="nbox">
      <textarea rows="2" placeholder="Add a note" aria-label="Note"></textarea>
      <div class="nrow"><span class="trunc">${esc(app.draft.target ? 'On ' + app.draft.target : app.draft.page)}</span>
      <span class="kbd">Enter</span></div></div>`)
    document.querySelector('.canvas').append(box)
    box.querySelector('textarea').addEventListener('keydown', draftKey)
    box.querySelector('textarea').focus()
  }
  if (!at) return
  const canvas = document.querySelector('.canvas')
  box.style.left = `${Math.max(8, Math.min(at.x - 8, canvas.clientWidth - box.offsetWidth - 8))}px`
  box.style.top = `${Math.min(at.y + 20, canvas.clientHeight - box.offsetHeight - 8)}px`
}

function draftKey(e) {
  e.stopPropagation()
  if (e.key === 'Escape') cancelDraft()
  if (e.key === 'Enter' && !e.shiftKey) {
    e.preventDefault()
    saveDraft(e.target.value)
  }
}

function cancelDraft() {
  app.draft = null
  renderNotes()
}

async function saveDraft(text) {
  if (!text.trim()) return cancelDraft()
  const { el, ...note } = app.draft
  app.draft = null
  await sendNotes('notes', 'POST', { ...note, version: app.route.ver, text })
}

// ---------- changes ----------

async function sendNotes(path, method, body) {
  const slug = app.route.slug
  try {
    const res = await fetch(`/api/projects/${encodeURIComponent(slug)}/${path}`, {
      method,
      headers: { 'Content-Type': 'application/json' },
      body: body && JSON.stringify(body),
    })
    if (res.ok && app.detail[slug]) app.detail[slug].notes = await res.json()
  } catch (e) {} // the list stays as it was
  if (app.route.slug === slug) renderNotes()
}

function approve() {
  const { ver } = app.route
  sendNotes('approve', 'POST', { version: notesOf().approved === ver ? '' : ver })
}

// openNote shows the pin of a note in the list: on its page, scrolled into view.
function openNote(id) {
  const n = versionNotes().find((x) => x.id === id)
  if (!n) return
  if (n.page !== currentPage()) {
    location.hash = viewerHash(app.route.slug, app.route.ver, n.page === 'index.html' ? '' : n.page)
    return
  }
  const frame = shownFrame()
  const at = spot(frameDoc(frame), n)
  if (at) frame.contentWindow.scrollBy({ top: at.y - frame.clientHeight / 3, behavior: 'smooth' })
  const pin = document.querySelector(`.pins [data-note="${id}"]`)
  if (pin) replay(pin, 'flash')
}

// notesChanged applies a "notes" event from another viewer.
async function notesChanged(e) {
  if (app.route.name !== 'view' || e.slug !== app.route.slug) return
  await loadProject(e.slug, true)
  renderNotes()
}

document.addEventListener('click', (e) => {
  const del = e.target.closest('[data-del]')
  if (del) return sendNotes(`notes/${del.dataset.del}`, 'DELETE')
  const row = e.target.closest('[data-note]')
  if (row) openNote(row.dataset.note)
})
