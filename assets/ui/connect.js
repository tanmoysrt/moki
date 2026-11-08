// Connect dialog: opening it makes a token, and it shows the text to paste into any
// agent. One page load makes one token. When this browser may not make tokens, the
// dialog shows "moki token register" for the moki machine.

const connect = {
  text: '', // the guide with the token in it
  denied: false, // this browser may not make tokens
  error: '',
}

async function openConnect() {
  app.dialog = true
  document.body.insertAdjacentHTML('beforeend', connectDialog())
  if (!connect.text && !connect.denied) await makeToken()
  redrawConnect()
}

function closeConnect() {
  app.dialog = false
  connect.error = ''
  document.querySelector('.scrim')?.remove()
}

async function makeToken() {
  try {
    const res = await fetch('/api/tokens', { method: 'POST' })
    if (res.status === 403) connect.denied = true
    else if (!res.ok) connect.error = (await res.text()).trim()
    else connect.text = (await res.json()).text
  } catch (e) {
    connect.error = 'moki does not answer.'
  }
}

function redrawConnect() {
  const dialog = document.querySelector('.dialog')
  if (dialog) dialog.innerHTML = connectBody()
}

// ---------- markup ----------

function connectDialog() {
  return `<div class="scrim" data-act="close">
    <div class="dialog" role="dialog" aria-modal="true" aria-label="Connect an agent">${connectBody()}</div>
  </div>`
}

function connectBody() {
  return `<header>
      <h2>Connect an agent</h2>
      <button class="ib" data-act="close" aria-label="Close">${icon('x')}</button>
    </header>
    ${connectContent()}`
}

function connectContent() {
  if (connect.error) return `<p class="error">${esc(connect.error)}</p>`
  if (connect.denied) {
    return `<p>Run this where moki runs, and paste what it prints into any agent.</p>
      ${codeBox('moki token register', '$ moki token register')}`
  }
  if (!connect.text) return ''
  return `<div class="guide">
      <pre class="code">${esc(connect.text)}</pre>
      <button class="btn primary" data-copy="${esc(connect.text)}">${icon('copy')}Copy</button>
    </div>
    <p class="note">Copy and paste it into your agent's chat.</p>`
}

function codeBox(text, shown = text) {
  return `<div class="code">${esc(shown)}<button class="ib" data-copy="${esc(text)}" aria-label="Copy">${icon('copy')}</button></div>`
}
