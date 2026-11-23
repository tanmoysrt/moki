// moki video: a short video made with HTML. Put the scene in <div class="stage">, and
// set window.VIDEO before this script:
//
//   VIDEO = { size: '1280x720', duration: 8, draw(t) { ...set styles for time t... } }
//
// video.js scales the stage to fit, plays it in a loop, and adds play and a scrubber.
// draw(t) must depend only on t, so any frame can be shown at any time.
// ?t=2.5 shows the frame at 2.5 s with no controls: use it for screenshots.
// Keys: space plays and pauses, ← → step 0.5 s.
//
// Helpers: video.span(t, from, to) is 0 to 1 between two times, eased.
// video.lerp(a, b, p) mixes two numbers.
;(() => {
  const V = window.VIDEO
  const [W, H] = (V.size || '1280x720').split('x').map(Number)
  const stage = document.querySelector('.stage')
  const fixed = new URLSearchParams(location.search).get('t')

  const ease = (x) => (x < 0.5 ? 4 * x * x * x : 1 - Math.pow(-2 * x + 2, 3) / 2)
  window.video = {
    span: (t, from, to) => ease(Math.max(0, Math.min(1, (t - from) / (to - from)))),
    lerp: (a, b, p) => a + (b - a) * p,
  }

  const css = document.createElement('style')
  css.textContent = `
    html, body { margin: 0; height: 100%; overflow: hidden; background: #e4e4e7; }
    html.dark, html.dark body { background: #09090b; }
    .stage { position: absolute; left: 50%; top: 50%; width: ${W}px; height: ${H}px; overflow: hidden; }
    .vctl { position: fixed; left: 12px; right: 12px; bottom: 10px; height: 32px; padding: 0 10px 0 4px;
      display: flex; align-items: center; gap: 10px; border-radius: 10px;
      background: rgb(24 24 27 / .72); color: #fff; font: 500 12px/1 Inter, system-ui, sans-serif;
      font-variant-numeric: tabular-nums; backdrop-filter: blur(8px); }
    .vctl button { width: 28px; height: 28px; display: grid; place-items: center; border: 0; border-radius: 7px;
      background: none; color: inherit; cursor: pointer; }
    .vctl input { flex: 1; accent-color: #fff; }`
  document.head.append(css)

  let t = +fixed || 0
  let playing = fixed == null
  let last = 0
  let bar

  function fit() {
    const room = innerHeight - (bar ? 52 : 0)
    const scale = Math.min(innerWidth / W, room / H)
    stage.style.transform = `translate(-50%, calc(-50% - ${bar ? 21 : 0}px)) scale(${scale})`
  }

  function render() {
    V.draw(t)
    if (!bar) return
    bar.querySelector('input').value = t
    bar.querySelector('span').textContent = `${t.toFixed(1)} / ${V.duration.toFixed(1)}`
    bar.querySelector('button').innerHTML = playing
      ? '<svg width="14" height="14" viewBox="0 0 14 14"><rect x="3" y="2" width="3" height="10" rx="1" fill="currentColor"/><rect x="8" y="2" width="3" height="10" rx="1" fill="currentColor"/></svg>'
      : '<svg width="14" height="14" viewBox="0 0 14 14"><path d="M4 2.5v9l7.5-4.5z" fill="currentColor"/></svg>'
  }

  function tick(now) {
    if (playing) t = (t + Math.min(0.1, (now - last) / 1000)) % V.duration
    last = now
    render()
    requestAnimationFrame(tick)
  }

  function seek(to) {
    t = Math.max(0, Math.min(V.duration - 0.001, to))
    render()
  }

  if (fixed == null) {
    bar = document.createElement('div')
    bar.className = 'vctl'
    bar.innerHTML = `<button aria-label="Play or pause"></button>
      <input type="range" min="0" max="${V.duration}" step="0.01" aria-label="Time"><span></span>`
    document.body.append(bar)
    bar.querySelector('button').onclick = () => ((playing = !playing), render())
    bar.querySelector('input').oninput = (e) => ((playing = false), seek(+e.target.value))
    addEventListener('keydown', (e) => {
      if (e.key === ' ') playing = !playing
      else if (e.key === 'ArrowRight') seek(t + 0.5)
      else if (e.key === 'ArrowLeft') seek(t - 0.5)
      else return
      e.preventDefault()
      render()
    })
  }

  addEventListener('resize', fit)
  fit()
  render()
  // moki reads this to offer the MP4 export, and steps the frames with seek
  window.moki = { kind: 'video', size: `${W}x${H}`, duration: V.duration, fps: V.fps || 30, seek, now: () => t }
  if (fixed == null) requestAnimationFrame((now) => ((last = now), tick(now)))
})()
