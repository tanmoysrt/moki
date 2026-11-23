// Notes: the pins over the page. moki draws them over the page, so the page stays as the
// agent made it. A pin keeps the element it is on (a CSS path) and the spot inside it, so
// it stays on the element when the page reflows.

function shownFrame() {
  return [...document.querySelectorAll('.canvas iframe.ready')].pop()
}

// placePins puts each open note of this page on its element, the fixed note that is
// picked, and the draft. It runs on each scroll and resize of the page.
function placePins() {
  const layer = document.querySelector('.pins')
  if (!layer) return
  const doc = frameDoc(shownFrame())
  const show = app.notesOpen && doc
  const open = openNotes()
  let html = ''
  if (show) {
    open.forEach((n, i) => (html += pinAt(doc, n, 'num', i + 1)))
    const picked = fixedNotes().find((n) => n.id === app.picked)
    if (picked) html += pinAt(doc, picked, 'ok', icon('check'))
    const at = app.draft && spot(doc, app.draft)
    if (at) html += `<i class="pin num open" style="left:${at.x}px;top:${at.y}px">${open.length + 1}</i>`
  }
  layer.innerHTML = html
  placeDraft(show && app.draft && spot(doc, app.draft))
}

function pinAt(doc, n, kind, label) {
  if (n.page !== currentPage()) return ''
  const at = spot(doc, n)
  return at ? `<i class="pin ${kind}" data-note="${n.id}" style="left:${at.x}px;top:${at.y}px">${label}</i>` : ''
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
  if (!canNote()) return
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
  app.picked = null
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
