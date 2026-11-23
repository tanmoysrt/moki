// Notes: the box to type a note, next to the draft pin. On a public link a visitor gives
// a name with the first note. The browser keeps the name, a secret and the ids of the
// visitor's notes: moki lets only the browser with the secret delete a note.

function placeDraft(at) {
  let box = document.querySelector('.nbox')
  if (!app.draft) return box?.remove()
  if (!box) {
    const ask = share && !visitor().name
    box = htmlElement(`<div class="nbox">
      ${ask ? '<input class="nname" placeholder="Your name" aria-label="Your name" maxlength="60">' : ''}
      <textarea rows="2" placeholder="Add a note" aria-label="Note"></textarea>
      <div class="nrow"><span class="trunc">${esc(app.draft.target ? 'On ' + app.draft.target : app.draft.page)}</span>
      <span class="kbd">Enter</span></div>
      ${ask ? '<p class="nwhy">Your name shows on your notes. This browser keeps it.</p>' : ''}</div>`)
    document.querySelector('.canvas').append(box)
    box.addEventListener('keydown', draftKey)
    box.querySelector(ask ? '.nname' : 'textarea').focus()
  }
  if (!at) return
  const canvas = document.querySelector('.canvas')
  box.style.left = `${Math.max(8, Math.min(at.x - 8, canvas.clientWidth - box.offsetWidth - 8))}px`
  box.style.top = `${Math.min(at.y + 20, canvas.clientHeight - box.offsetHeight - 8)}px`
}

function draftKey(e) {
  e.stopPropagation()
  if (e.key === 'Escape') return cancelDraft()
  if (e.key !== 'Enter' || e.shiftKey) return
  e.preventDefault()
  const box = e.currentTarget
  const name = box.querySelector('.nname')
  if (name && !name.value.trim()) return name.focus()
  if (e.target === name) return box.querySelector('textarea').focus()
  saveDraft(box.querySelector('textarea').value, name?.value.trim())
}

function cancelDraft() {
  app.draft = null
  renderNotes()
}

async function saveDraft(text, name) {
  if (!text.trim()) return cancelDraft()
  const { el, ...note } = app.draft
  app.draft = null
  if (!share) return sendNotes('notes', 'POST', { ...note, version: app.route.ver, text })
  const me = visitor()
  if (name) me.name = name
  const notes = await sendNotes('notes', 'POST', { ...note, version: app.route.ver, text, by: me.name, key: me.key })
  const mineNow = notes?.notes.findLast((n) => n.by === me.name && n.text === text.trim())
  if (mineNow) me.ids.push(mineNow.id)
  saveVisitor(me)
  renderNotes()
}

// ---------- the visitor of a public link ----------

function visitorFoot() {
  const { name } = visitor()
  if (!name) return ''
  return `<div class="nfoot"><div class="me"><span class="trunc">Notes as <b>${esc(name)}</b></span>
    <button class="textbtn" data-act="rename">Change</button></div></div>`
}

// rename forgets the name, so the next note asks for it again.
function rename() {
  const me = visitor()
  me.name = ''
  saveVisitor(me)
  renderNotes()
}

const mine = (id) => visitor().ids.includes(id)

let visitorCache = null

function visitor() {
  if (visitorCache) return visitorCache
  let saved = {}
  try {
    saved = JSON.parse(localStorage.getItem('moki-visitor')) || {}
  } catch (e) {}
  const key = saved.key || [...crypto.getRandomValues(new Uint8Array(16))].map((b) => b.toString(16).padStart(2, '0')).join('')
  return (visitorCache = { name: saved.name || '', key, ids: saved.ids || [] })
}

function saveVisitor(me) {
  visitorCache = me
  try {
    localStorage.setItem('moki-visitor', JSON.stringify(me))
  } catch (e) {} // a private window: the name lasts until the tab closes
}
