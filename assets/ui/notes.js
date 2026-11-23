// Notes in the viewer. With the list open, a click on the page pins a note on that spot.
// The agent reads the open notes in get_project, and answer_notes marks them done in a
// version or replies. The list shows the open notes of the version on screen first, also
// the ones from older versions, then the notes that this version fixed. On a public link
// with notes on, visitors add notes with their name (notes-box.js).

const notesOf = () => app.detail[app.route.slug]?.notes || { notes: [] }
const currentPage = () => app.route.page || 'index.html'
const touch = matchMedia('(pointer: coarse)').matches

// canNote: the owner always; a visitor when the link takes notes, which sends the notes.
const canNote = () => !share || Boolean(app.detail[share]?.notes)

// openNotes were open on the version on screen: written on it or before, and not fixed yet.
function openNotes() {
  const at = versionNumber(app.route.ver)
  return notesOf().notes.filter((n) => versionNumber(n.version) <= at && (!n.done || versionNumber(n.done) > at))
}

const fixedNotes = () => notesOf().notes.filter((n) => n.done === app.route.ver)

function notesButton() {
  if (!canNote()) return ''
  const count = openNotes().length
  return `<button class="ib nb${app.notesOpen ? ' on' : ''}" data-act="notes" aria-label="Notes" title="Notes (N)">
    ${icon('note')}${count && !app.notesOpen ? `<b class="num">${count}</b>` : ''}
  </button>`
}

function notesPanel() {
  const { ver } = app.route
  const open = openNotes()
  const fixed = fixedNotes()
  return `<div class="nhead"><b>Notes on ${ver}</b><span class="num">${open.length ? open.length + ' open' : ''}</span>
      <div class="sp"></div><button class="ib sm close" data-act="notes" aria-label="Close">${icon('x')}</button></div>
    <div class="nscroll">${keptGroup()}
      <ul class="nlist">${open.map(noteRow).join('')}</ul>
      <p class="nhint">${noteHint()}</p>
      ${fixed.length ? `<div class="ngroup"><b>Fixed in ${ver}</b><span class="num">${fixed.length}</span></div>
      <ul class="nlist fixed">${fixed.map(fixedRow).join('')}</ul>` : ''}
    </div>
    ${notesFoot()}`
}

function noteHint() {
  if (gridMode()) return 'Open an option to pin a note on it.'
  return `${touch ? 'Tap' : 'Click'} the page${splitOn() ? ' or the phone' : ''} to pin a note.`
}

function notesFoot() {
  if (share) return visitorFoot()
  const { ver } = app.route
  const approved = notesOf().approved === ver
  return `<div class="nfoot">
    <button class="btn wide${approved ? ' done' : ''}" data-act="approve">${icon('check')}${approved ? `${ver} approved` : `Approve ${ver}`}</button>
  </div>`
}

function noteRow(n, i) {
  const del = !share || mine(n.id)
  return `<li data-note="${n.id}"><i class="pin num">${i + 1}</i>
    <div class="ntext"><p>${esc(n.text)}</p><span class="trunc">${noteMeta(n)}</span>${reply(n)}</div>
    ${del ? `<button class="ib sm" data-del="${n.id}" aria-label="Delete note" title="Delete">${icon('x')}</button>` : ''}</li>`
}

function fixedRow(n) {
  const picked = app.picked === n.id ? ' class="picked"' : ''
  return `<li data-note="${n.id}"${picked}><i class="pin ok">${icon('check')}</i>
    <div class="ntext"><p>${esc(n.text)}</p><span class="trunc">${noteMeta(n)}</span>${reply(n)}</div>
    ${share ? '' : `<button class="ib sm" data-reopen="${n.id}" aria-label="Reopen" title="Reopen">${icon('undo')}</button>`}</li>`
}

// noteMeta: who wrote it, the version it came from, and where it is.
function noteMeta(n) {
  const page = optionName(n.page) || (n.page === 'index.html' ? '' : n.page)
  const from = n.version === app.route.ver ? '' : `from ${n.version}`
  const lead = [n.view === 'phone' && 'Phone', n.by, from].filter(Boolean).map((x) => `<em>${esc(x)}</em>`)
  return [...lead, ...[page, n.target].filter(Boolean).map(esc)].join(' · ')
}

const reply = (n) => (n.reply ? `<div class="reply">${esc(n.reply)}</div>` : '')

// renderNotes draws the bar button, the list and the pins for the route on screen.
function renderNotes() {
  const panel = document.querySelector('.npanel')
  if (!panel) return
  document.querySelector('.nb')?.replaceWith(htmlElement(notesButton()))
  panel.hidden = !app.notesOpen || (app.draft && innerWidth <= 640)
  if (app.notesOpen) {
    const scroll = panel.querySelector('.nscroll')?.scrollTop || 0
    panel.innerHTML = notesPanel()
    panel.querySelector('.nscroll').scrollTop = scroll
  }
  aimCursor()
  placePins()
  renderKept()
}

function toggleNotes() {
  if (!canNote()) return
  app.notesOpen = !app.notesOpen
  app.draft = null
  app.picked = null
  renderNotes()
}

// ---------- changes ----------

// sendNotes changes the notes and shows the notes that moki sends back.
async function sendNotes(path, method, body, headers = {}) {
  const slug = app.route.slug
  const base = share ? `/s/${share}/` : `/api/projects/${encodeURIComponent(slug)}/`
  let notes = null
  try {
    const res = await fetch(base + path, {
      method,
      headers: { 'Content-Type': 'application/json', ...headers },
      body: body && JSON.stringify(body),
    })
    if (res.ok && app.detail[slug]) notes = app.detail[slug].notes = await res.json()
  } catch (e) {} // the list stays as it was
  if (app.route.slug === slug) renderNotes()
  return notes
}

function approve() {
  const { ver } = app.route
  sendNotes('approve', 'POST', { version: notesOf().approved === ver ? '' : ver })
}

function deleteNote(id) {
  if (!share) return sendNotes(`notes/${id}`, 'DELETE')
  sendNotes(`notes/${id}`, 'DELETE', null, { 'X-Moki-Key': visitor().key })
}

// openNote shows the pin of a note in the list: on its page, scrolled into view.
// A fixed note shows its green pin only while it is picked.
function openNote(id) {
  const n = notesOf().notes.find((x) => x.id === id)
  if (!n) return
  app.picked = n.done === app.route.ver ? id : null
  if (n.page !== currentPage()) {
    location.hash = viewerHash(app.route.slug, app.route.ver, n.page === 'index.html' ? '' : n.page)
    return
  }
  renderNotes()
  const at = spot(n, false)
  if (at) at.frame.contentWindow.scrollBy({ top: at.fy - at.frame.clientHeight / 3, behavior: 'smooth' })
  const pin = document.querySelector(`.pins [data-note="${id}"]`)
  if (pin) replay(pin, 'flash')
}

// notesChanged applies a "notes" event from another viewer or the agent.
async function notesChanged(e) {
  if (app.route.name !== 'view' || e.slug !== app.route.slug) return
  await loadProject(e.slug, true)
  renderNotes()
}

document.addEventListener('click', (e) => {
  const del = e.target.closest('[data-del]')
  if (del) return deleteNote(del.dataset.del)
  const reopen = e.target.closest('[data-reopen]')
  if (reopen) return sendNotes(`notes/${reopen.dataset.reopen}/reopen`, 'POST')
  const row = e.target.closest('[data-note]')
  if (row) openNote(row.dataset.note)
})
