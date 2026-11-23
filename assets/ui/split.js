// Desktop and phone side by side. With split on, the viewer shows the page at desktop
// width, scaled to fit its pane, and the same page in a phone on the right. A click on
// either one pins a note; a note on the phone has view "phone". Split is not on a narrow
// screen, and not for decks and videos: they have one size. The browser keeps the choice.

const PHONE = { w: 390, h: 844 }
const DESKTOP = 1280
const kinds = {} // "slug/v2/page" -> the kind of the page, so a deck does not show the phone while it loads
const kindKey = () => app.route.slug + '/' + app.route.ver + '/' + app.route.page

function savedSplit() {
  try {
    return localStorage.getItem('moki-split') === '1'
  } catch (e) {
    return false
  }
}

function splitButton() {
  if (!splitAllowed()) return ''
  return `<button class="ib dev${app.split ? ' on' : ''}" data-act="split" aria-label="Desktop and phone" title="Desktop and phone (D)">
    ${icon('devices')}
  </button>`
}

function splitAllowed() {
  const kind = pageKind()
  return innerWidth >= 1100 && kind !== 'deck' && kind !== 'video' && !gridMode()
}

// pageKind is the kind of the page of the route, while the frame of the last page still shows.
function pageKind() {
  const loaded = shownFrame()?.dataset.at === app.route.ver + '/' + app.route.page
  return loaded ? pageKit().kind : kinds[kindKey()]
}

const splitOn = () => app.split && splitAllowed()
const phoneFrame = () => document.querySelector('.ppane iframe')

// viewFrame is the frame of a note: the phone for a phone note while split is on.
const viewFrame = (view) => (view === 'phone' && phoneFrame()) || shownFrame()

function toggleSplit() {
  if (!splitAllowed()) return
  app.split = !app.split
  try {
    localStorage.setItem('moki-split', app.split ? '1' : '')
  } catch (e) {}
  document.querySelector('.vbar .dev')?.classList.toggle('on', app.split)
  if (app.draft?.view === 'phone') app.draft = null
  applySplit()
  renderNotes()
}

// applySplit adds or removes the phone, and scales the desktop page to its pane.
// It runs on render, when a page loads, and on resize.
function applySplit() {
  const viewer = document.querySelector('.viewer')
  if (!viewer) return
  kinds[kindKey()] = pageKind()
  if (Boolean(document.querySelector('.vbar .dev')) !== splitAllowed()) renderBar()
  const on = splitOn()
  viewer.classList.toggle('split', on)
  const pane = document.querySelector('.ppane')
  if (on && !pane) document.querySelector('.canvas').insertAdjacentHTML('afterend', phonePane())
  if (!on) pane?.remove()
  const hint = document.querySelector('.nhint')
  if (hint) hint.textContent = noteHint()
  fitSplit()
  resizes.observe(document.querySelector('.canvas')) // the notes list opens and closes
  placePins()
}

const resizes = new ResizeObserver(() => {
  fitSplit()
  placePins()
})

// fitSplit scales the phone to the height of its pane, then the desktop page to the
// width that is left. A pane wider than 1280 px shows the page at its own width.
function fitSplit() {
  const canvas = document.querySelector('.canvas')
  const pane = document.querySelector('.ppane')
  if (pane) pane.style.setProperty('--pscale', Math.min(1, (pane.clientHeight - 32) / PHONE.h))
  if (!canvas) return
  const width = Math.max(DESKTOP, canvas.clientWidth)
  canvas.style.setProperty('--dw', width + 'px')
  canvas.style.setProperty('--dscale', canvas.clientWidth / width)
}

function phonePane() {
  const { slug, ver, page } = app.route
  return `<div class="ppane"><div class="pwrap"><div class="phone">
    <iframe src="${pageURL(slug, ver, page)}" data-at="${esc(ver + '/' + page)}" title="${esc(slug)} ${ver} on a phone" onload="phoneReady(this)"></iframe>
  </div></div></div>`
}

function phoneReady(frame) {
  setFrameTheme(frame)
  followPage(frame)
  notesFrame(frame)
  try {
    frame.contentDocument.addEventListener('keydown', forwardKey)
  } catch (e) {}
}

// syncFrames shows the page of the route in both frames, after a link opened another
// page in one of them.
function syncFrames(from) {
  const phone = phoneFrame()
  if (from === phone) return showVersion()
  const at = app.route.ver + '/' + app.route.page
  if (!phone || phone.dataset.at === at) return
  phone.dataset.at = at
  phone.src = pageURL(app.route.slug, app.route.ver, app.route.page)
}

addEventListener('resize', applySplit)
