// Options board. A page sets window.BOARD and loads this script at the end of <body>:
//
//   window.BOARD = {
//     title: 'Team page',
//     intro: 'Three ways to show members and invites.',
//     frame: 'page.html',  // default page for frames given as a query
//     sections: [
//       { title: 'Layout', short: 'Layout', opts: [
//         { l: 'A', n: 'Tabs', note: 'Members, requests and settings in tabs.', rec: true,
//           fr: [['', 'v=tabs'], ['Empty state', 'b.html#empty']] },
//       ] },
//     ],
//   }
//
// A frame is a query for BOARD.frame, or a path to a page. Each frame shows the page at
// desktop (1280) and phone (390) width, at 62%. On a phone only the phone frames show, at 88%.
// The board follows the moki theme and passes it to the frames as ?dark=1 or ?dark=0.
;(() => {
  const B = window.BOARD
  const root = document.documentElement
  let dark = initialDark()
  root.classList.toggle('dark', dark)

  document.head.insertAdjacentHTML('beforeend', `<style>${css()}</style>`)
  document.body.innerHTML = `
    <nav class="bar">${B.sections.map((s, i) => `<a href="#s${i + 1}">${s.short || s.title}</a>`).join('')}</nav>
    <h1>${B.title}</h1>
    <p class="intro">${B.intro || ''}</p>
    ${B.sections.map(section).join('')}`

  // Another tab changed the moki theme.
  addEventListener('storage', (e) => {
    if (e.key === 'moki-theme' && e.newValue) root.classList.toggle('dark', e.newValue === 'dark')
  })

  // moki sets the class on this page when its theme changes: reload the frames in that theme.
  new MutationObserver(() => {
    if (root.classList.contains('dark') === dark) return
    dark = !dark
    document.querySelectorAll('iframe').forEach((f) => (f.src = frameURL(f.dataset.u)))
  }).observe(root, { attributes: true, attributeFilter: ['class'] })

  function section(s, i) {
    return `<h2 id="s${i + 1}">${s.title}</h2>${s.opts.map(option).join('')}`
  }

  function option(o) {
    const name = `${o.l ? o.l + '. ' : ''}${o.n}`
    return `<h3>${name}${o.rec ? '<em>recommended</em>' : ''}</h3>
      <p class="note">${o.note || ''}${o.flag ? `<span class="flag">${o.flag}</span>` : ''}</p>
      ${(o.fr || []).map(frame).join('')}`
  }

  function frame([caption, u]) {
    const src = frameURL(u)
    return `${caption ? `<h4>${caption}</h4>` : ''}
      <div class="row">
        <iframe class="d" loading="lazy" data-u="${u}" src="${src}"></iframe>
        <iframe class="p" loading="lazy" data-u="${u}" src="${src}"></iframe>
      </div>`
  }

  // frameURL adds the theme before any #hash.
  function frameURL(u) {
    const [path, hash] = (u.includes('.html') ? u : `${B.frame || 'page.html'}?${u}`).split('#')
    const sep = path.includes('?') ? '&' : '?'
    return `${path}${sep}dark=${dark ? 1 : 0}${hash ? '#' + hash : ''}`
  }

  function initialDark() {
    const query = new URLSearchParams(location.search)
    const fixed = query.get('theme') || { 1: 'dark', 0: 'light' }[query.get('dark')]
    if (fixed) return fixed === 'dark'
    if (root.classList.contains('dark')) return true
    try {
      const saved = localStorage.getItem('moki-theme')
      if (saved) return saved === 'dark'
    } catch (e) {}
    return matchMedia('(prefers-color-scheme: dark)').matches
  }

  function css() {
    return `
      @font-face { font-family: Inter; src: url(/kit/inter.woff2) format('woff2'); font-weight: 100 900; }
      * { box-sizing: border-box; margin: 0; padding: 0; }
      body { padding: 24px 32px 80px; background: #d4d4d8; color: #18181b; font: 14px/22px Inter, system-ui, sans-serif; }
      .dark body { background: #27272a; color: #f4f4f5; }
      a { color: #2563eb; }

      .bar { position: sticky; top: 0; z-index: 5; height: 36px; margin: -24px -32px 0; padding: 0 32px;
             display: flex; gap: 16px; align-items: center; justify-content: flex-end; background: inherit; }
      .bar a { font-size: 13px; color: inherit; opacity: 0.7; text-decoration: none; }
      .bar a:hover { opacity: 1; }

      h1 { font-size: 24px; font-weight: 600; letter-spacing: -0.02em; }
      h2 { margin: 56px 0 4px; padding-top: 16px; border-top: 1px solid rgba(0, 0, 0, 0.15); font-size: 20px; font-weight: 600; }
      h3 { margin: 28px 0 2px; font-size: 16px; font-weight: 600; }
      h3 em { margin-left: 8px; font-size: 13px; font-style: normal; font-weight: 500; color: #2563eb; }
      h4 { margin: 10px 0 6px; font-size: 13px; font-weight: 500; color: #52525b; }
      .intro { max-width: 820px; margin-top: 6px; color: #52525b; }
      .note { max-width: 820px; margin-bottom: 10px; color: #52525b; }
      .dark h2 { border-color: rgba(255, 255, 255, 0.15); }
      .dark h4, .dark .intro, .dark .note { color: #a1a1aa; }
      .flag { display: inline-block; margin-left: 4px; padding: 0 6px; border-radius: 4px;
              background: #fef3c7; color: #92400e; font-size: 12px; font-weight: 500; }

      .row { margin-bottom: 8px; display: flex; flex-wrap: wrap; gap: 24px; align-items: flex-start; }
      iframe { border: 0; border-radius: 6px; background: #fff; box-shadow: 0 4px 20px rgba(0, 0, 0, 0.15);
               transform: scale(0.62); transform-origin: 0 0; }
      .d { width: 1280px; height: 820px; margin: 0 -486px -312px 0; }
      .p { width: 390px; height: 820px; margin: 0 -148px -312px 0; }

      @media (max-width: 640px) {
        body { padding: 16px 16px 60px; }
        .bar { margin: -16px -16px 0; padding: 0 16px; justify-content: flex-start; overflow: auto; white-space: nowrap; }
        .d { display: none; }
        .p { transform: scale(0.88); margin: 0 -47px -98px 0; }
      }`
  }
})()
