// Export: a menu in the viewer bar with the files that fit the page. The kit scripts set
// window.moki in the page, so the viewer tells a deck or a video from a screen. moki makes
// the file in its own Chrome, in the theme on screen; the menu shows progress until it is ready.

const EXPORTS = {
  deck: (m) => [
    ['pdf', 'PDF', `${m.slides} slides, ${size(m)}`],
    ['zip', 'Slides as images', `${m.slides} PNG files in a .zip, at 2×`],
    ['png', 'PNG', `Slide ${m.at?.() || 1}, at 2×`],
  ],
  video: (m) => [
    ['mp4', 'MP4', `${m.duration} s, ${size(m)}, ${m.fps} fps`],
    ['png', 'PNG', 'This frame, at 2×'],
  ],
  drawing: () => [
    ['png', 'PNG', 'The page, at 2×'],
    ['svg', 'SVG', 'The drawing as a vector'],
    ['pdf', 'PDF', 'The page'],
  ],
  screen: () => [
    ['png', 'PNG', 'This page, full height, at 2×'],
    ['pdf', 'PDF', 'This page'],
  ],
}

const EXPORT_ICONS = { pdf: 'pdf', zip: 'slides', png: 'img', mp4: 'film', svg: 'pen', files: 'box' }

function exportButton() {
  const open = app.exportOpen
  const html = `<div class="xwrap">
    <button class="ib${open ? ' on' : ''}" data-act="export" aria-label="Export" title="Export (E)">${icon('down')}</button>
    ${open ? `<div class="xscrim" data-act="export"></div><div class="xmenu${app.exportFade ? ' fade' : ''}">${exportRows()}</div>` : ''}
  </div>`
  app.exportFade = false
  return html
}

function exportRows() {
  const m = pageKit()
  const rows = (EXPORTS[m.kind] || EXPORTS.screen)(m)
  const files = ['files', 'Files', `${app.route.ver} as .tar.gz, to build or edit`]
  return rows.map(exportRow).join('') + '<div class="xline"></div>' + exportRow(files)
}

function exportRow([format, name, what]) {
  const job = app.job?.format === format ? app.job : null
  const tile = `<div class="tile">${icon(EXPORT_ICONS[format])}</div>`
  if (job && !job.error) {
    return `<div class="xrow making">${tile}
      <div class="what"><b>Making ${name}…</b><span class="num">${progressText(job)}</span>
        <div class="xprog"><i style="width:${Math.round(job.done * 100)}%"></i></div></div>
      <button class="ib stop" data-act="stopExport" aria-label="Stop" title="Stop">${icon('x')}</button>
    </div>`
  }
  const line = job ? `<span class="err">${esc(job.error)}</span>` : `<span class="trunc">${what}</span>`
  return `<button class="xrow" data-act="exportAs" data-format="${format}">${tile}<div class="what"><b>${name}</b>${line}</div></button>`
}

function progressText({ done, format, kit }) {
  if (!done) return 'Opening the page…'
  if (format === 'mp4') return `${(done * kit.duration).toFixed(1)} s of ${kit.duration} s`
  if (format === 'zip') return `Slide ${Math.round(done * kit.slides)} of ${kit.slides}`
  return 'Almost done…'
}

function toggleExport() {
  app.exportOpen = !app.exportOpen
  app.exportFade = app.exportOpen
  if (app.job?.error) app.job = null
  renderBar()
}

// exportAs starts an export of the page on screen: the slide or the frame it shows too.
async function exportAs(button) {
  if (app.job && !app.job.error) return
  const kit = pageKit()
  const format = button.dataset.format
  const { slug, ver, page } = app.route
  const q = new URLSearchParams({ page: page || 'index.html', format, theme: app.dark ? 'dark' : 'light' })
  if (format === 'png' && kit.kind === 'deck') q.set('slide', kit.at?.() || 1)
  if (format === 'png' && kit.kind === 'video') q.set('time', kit.now?.() || 0)
  const job = (app.job = { format, kit, done: 0 })
  renderExport()
  try {
    const res = await fetch(`/api/export/${slug}/${ver}?${q}`, { method: 'POST' })
    if (!res.ok) throw new Error((await res.text()).trim())
    job.id = (await res.json()).id
    await waitExport(job)
  } catch (e) {
    if (app.job === job) job.error = e.message
  }
  renderExport()
}

// waitExport asks moki how far the export is until the file is ready, then saves it.
async function waitExport(job) {
  while (app.job === job) {
    await new Promise((ok) => setTimeout(ok, 300))
    const state = await (await fetch(`/api/export/${job.id}`)).json()
    if (app.job !== job) return
    job.done = state.done
    if (state.error) throw new Error(state.error)
    if (state.ready) {
      saveFile(`/api/export/${job.id}/file`)
      app.job = null
      app.exportOpen = false
      return renderBar()
    }
    renderExport()
  }
}

function stopExport() {
  const job = app.job
  app.job = null
  if (job?.id) fetch(`/api/export/${job.id}`, { method: 'DELETE' })
  renderExport()
}

function renderExport() {
  const menu = document.querySelector('.xmenu')
  if (menu) menu.innerHTML = exportRows()
}

// pageKit is window.moki of the page on screen, or a screen without it.
function pageKit() {
  try {
    const kit = shownFrame()?.contentWindow.moki
    if (kit?.kind) return kit
  } catch (e) {} // a page from another origin
  return { kind: 'screen' }
}

function saveFile(url) {
  const a = document.createElement('a')
  a.href = url
  a.download = ''
  a.click()
}

const size = (m) => m.size.replace('x', '×')
