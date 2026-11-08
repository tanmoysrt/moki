// Theme. One theme for moki and the pages in it, saved as moki-theme. Pages that load
// /kit/theme.js read the same key; moki also sets the class on each page directly.

function toggleTheme() {
  app.dark = !app.dark
  try {
    localStorage.setItem('moki-theme', app.dark ? 'dark' : 'light')
  } catch (e) {}
  applyTheme()
}

function applyTheme() {
  document.documentElement.classList.toggle('dark', app.dark)
  document.querySelectorAll('iframe').forEach(setFrameTheme)
  document.querySelectorAll('[data-act=theme]').forEach((b) => (b.innerHTML = icon(app.dark ? 'sun' : 'moon')))
  document.querySelectorAll('.tile').forEach((el) => setThumb(el.querySelector('.thumb'), project(el.dataset.slug)))
}

function setFrameTheme(frame) {
  try {
    frame.contentDocument.documentElement.classList.toggle('dark', app.dark)
  } catch (e) {}
}

function savedTheme() {
  try {
    return localStorage.getItem('moki-theme')
  } catch (e) {
    return null
  }
}

// Another moki tab changed the theme.
addEventListener('storage', (e) => {
  if (e.key !== 'moki-theme' || !e.newValue) return
  app.dark = e.newValue === 'dark'
  applyTheme()
})

// The system theme changed, and the user never picked one.
matchMedia('(prefers-color-scheme: dark)').addEventListener('change', (e) => {
  if (savedTheme()) return
  app.dark = e.matches
  applyTheme()
})
