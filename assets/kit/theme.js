// moki theme for a mockup page: sets the `dark` class on <html>.
// Load it in <head>: <script src="/kit/theme.js"></script>
//
// Order: ?theme=dark|light (or ?dark=1|0), then the moki theme in localStorage, then the
// system theme. An open page follows when the theme changes in moki or in the system.
;(() => {
  const root = document.documentElement
  const query = new URLSearchParams(location.search)
  const fixed = query.get('theme') || { 1: 'dark', 0: 'light' }[query.get('dark')]
  const system = matchMedia('(prefers-color-scheme: dark)')

  const saved = () => {
    try {
      return localStorage.getItem('moki-theme')
    } catch (e) {
      return null
    }
  }
  const apply = () => {
    const theme = fixed || saved()
    root.classList.toggle('dark', theme ? theme === 'dark' : system.matches)
  }

  apply()
  if (fixed) return
  addEventListener('storage', (e) => e.key === 'moki-theme' && apply())
  system.addEventListener('change', apply)
})()
