// Notes: the pins over the page. moki draws them over the page, so the page stays as the
// agent made it. A pin keeps the element it is on (a CSS path) and the spot inside it, so
// it stays on the element when the page reflows.

function shownFrame() {
  return [...document.querySelectorAll('.canvas iframe.ready')].pop()
}

// placePins puts each open note of this page on its element, the fixed note that is
// picked, and the draft. It runs on each scroll and resize of the page. The pins layer
// covers the desktop page and the phone, so a spot is in the coordinates of the layer.
function placePins() {
  const layer = document.querySelector('.pins')
  if (!layer) return
  const show = app.notesOpen && shownFrame()
  const open = openNotes()
  let html = ''
  if (show) {
    open.forEach((n, i) => (html += pinAt(n, 'num', i + 1)))
    const picked = fixedNotes().find((n) => n.id === app.picked)
    if (picked) html += pinAt(picked, 'ok', icon('check'))
    const at = app.draft && spot(app.draft)
    if (at) html += `<i class="pin num open" style="left:${at.x}px;top:${at.y}px">${open.length + 1}</i>`
  }
  layer.innerHTML = html
  placeDraft(show && app.draft && spot(app.draft))
}

function pinAt(n, kind, label) {
  if (n.page !== currentPage()) return ''
  const at = spot(n)
  return at ? `<i class="pin ${kind}" data-note="${n.id}" style="left:${at.x}px;top:${at.y}px">${label}</i>` : ''
}

// spot is where a note sits in the viewer: in its frame (fx, fy), and on the pins layer
// (x, y). A note in a frame of the page, such as a board option, has a selector for each
// frame: "iframe >>> button". With clip, a spot out of view is null.
function spot(n, clip = true) {
  const frame = viewFrame(n.view)
  const doc = frameDoc(frame)
  if (!doc) return null
  const el = n.el?.isConnected ? n.el : findEl(doc, n.selector)
  const r = el && rectIn(el, doc)
  if (!r || (!r.width && !r.height)) return null
  const fx = r.left + n.x * r.width
  const fy = r.top + n.y * r.height
  if (clip && (fx < 0 || fy < 0 || fx > frame.clientWidth || fy > frame.clientHeight)) return null
  const f = frame.getBoundingClientRect()
  const base = document.querySelector('.pins').getBoundingClientRect()
  const scale = frame.clientWidth ? f.width / frame.clientWidth : 1
  return { x: f.left - base.left + fx * scale, y: f.top - base.top + fy * scale, fx, fy, frame }
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

// pageDocs is the page on screen, on the desktop and the phone, and the pages in its frames.
const pageDocs = () => [shownFrame(), phoneFrame()].flatMap((f) => docsIn(frameDoc(f)))

function docsIn(doc) {
  if (!doc) return []
  return [doc, ...[...doc.querySelectorAll('iframe')].flatMap((f) => docsIn(frameDoc(f)))]
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
  // not instanceof Element: the page in the frame has its own Element
  const el = e.target.nodeType === 1 ? e.target : e.target.parentElement
  if (!app.notesOpen) return
  e.preventDefault()
  e.stopPropagation()
  const r = el.getBoundingClientRect()
  let top = el.ownerDocument.defaultView.frameElement // the frame of the viewer that holds el
  while (top && top.ownerDocument !== document) top = top.ownerDocument.defaultView.frameElement
  app.picked = null
  app.draft = {
    el,
    view: top?.closest('.ppane') ? 'phone' : '',
    page: currentPage(),
    selector: framePath(el),
    target: describe(el) + inFrame(el),
    x: r.width ? round((e.clientX - r.left) / r.width) : 0,
    y: r.height ? round((e.clientY - r.top) / r.height) : 0,
  }
  renderNotes()
}
