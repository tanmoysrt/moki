// Kept options in the notes list. The user keeps options in the grid or in the bar
// (options.js), and moki stores them with the notes. Visitors only see them.

// A visitor without notes gets the kept options on their own.
const keptOf = () => (notesOf().kept || app.detail[app.route.slug]?.kept)?.[app.route.ver] || []

function keptGroup() {
  const kept = keptOf()
  if (!kept.length) return ''
  return `<div class="ngroup lead"><b>Kept</b><span class="num">${kept.length}</span></div>
    <ul class="kept">${sortKept(kept).map(keptRow).join('')}</ul>
    <div class="ngroup"><b>Open</b><span class="num">${openNotes().length || ''}</span></div>`
}

// sortKept puts the kept options in the order of the grid.
function sortKept(kept) {
  const order = (versionOptions()?.parts || []).flatMap((p) => p.options.map((o) => p.title + '/' + o.id))
  const at = (k) => (order.indexOf(k.part + '/' + k.option) + 1 || order.length + 1)
  return [...kept].sort((a, b) => at(a) - at(b))
}

// keptRow opens its option, and drops it.
function keptRow(k) {
  const page = versionOptions()?.parts.find((p) => p.title === k.part)?.options.find((o) => o.id === k.option)?.page
  const text = `<p>${esc(k.name || k.option)}</p>${k.part ? `<span class="trunc">${esc(k.part)}</span>` : ''}`
  const drop = share ? '' : `<button class="ib sm" data-drop="${esc(k.option)}" data-part="${esc(k.part)}" aria-label="Drop" title="Drop">${icon('x')}</button>`
  return `<li><i class="tag">${esc(k.option.slice(0, 2))}</i>
    ${page ? `<a class="ntext" href="${viewerHash(app.route.slug, app.route.ver, page)}">${text}</a>` : `<div class="ntext">${text}</div>`}${drop}</li>`
}

document.addEventListener('click', (e) => {
  const drop = e.target.closest('[data-drop]')
  if (!drop) return
  const { drop: option, part } = drop.dataset
  sendNotes('keep', 'POST', { version: app.route.ver, part, option, on: false })
})
