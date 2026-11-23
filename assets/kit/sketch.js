// moki sketch: hand-drawn shapes in SVG, for diagrams and art.
//
//   const s = sketch(document.querySelector('svg'), { seed: 7 })
//   s.box(40, 40, 160, 72, { label: 'Agent', fill: '#fde68a' })
//   s.arrow(200, 76, 320, 76, { label: 'upload' })
//
// Shapes: line, arrow, box, circle, poly (points, close), curve (smooth points), text.
// Options: stroke, fill, width, label, size, dash. The same seed draws the same lines,
// so screenshots do not change. clean: true draws straight lines in the page font.
;(() => {
  const NS = 'http://www.w3.org/2000/svg'

  function rng(seed) {
    return () => {
      seed = (seed + 0x6d2b79f5) | 0
      let t = Math.imul(seed ^ (seed >>> 15), 1 | seed)
      t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t
      return ((t ^ (t >>> 14)) >>> 0) / 4294967296
    }
  }

  window.sketch = function sketch(svg, opts = {}) {
    window.moki = window.moki || { kind: 'drawing' } // moki offers PNG and SVG; deck.js and video.js override it
    const rand = rng(opts.seed || 1)
    const clean = !!opts.clean
    const ink = opts.stroke || 'currentColor'
    const font = clean ? 'Inter, system-ui, sans-serif' : "Caveat, 'Comic Sans MS', cursive"
    const j = (n) => (clean ? 0 : (rand() - 0.5) * 2 * n)

    function add(tag, attrs, parent = svg) {
      const el = document.createElementNS(NS, tag)
      for (const k in attrs) if (attrs[k] != null) el.setAttribute(k, attrs[k])
      parent.append(el)
      return el
    }

    // one hand stroke from a to b: the ends wobble and the middle bows a little
    function stroke(x1, y1, x2, y2, bow) {
      const len = Math.hypot(x2 - x1, y2 - y1)
      const b = Math.min(bow ?? len / 60, 3)
      const mx = (x1 + x2) / 2 + j(b * 1.6)
      const my = (y1 + y2) / 2 + j(b * 1.6)
      return `M${x1 + j(b)},${y1 + j(b)} Q${mx},${my} ${x2 + j(b)},${y2 + j(b)}`
    }

    function path(d, o, fill = 'none') {
      return add('path', {
        d,
        fill,
        stroke: fill === 'none' ? o.stroke || ink : 'none',
        'stroke-width': o.width || 2,
        'stroke-linecap': 'round',
        'stroke-linejoin': 'round',
        'stroke-dasharray': o.dash,
      })
    }

    // a shape is drawn twice by hand, once when clean
    function twice(make, o) {
      const g = add('g', {})
      for (let i = 0; i < (clean ? 1 : 2); i++) g.append(path(make(), o))
      return g
    }

    function fillPoly(points, o) {
      if (!o.fill) return
      const d = points.map(([x, y], i) => `${i ? 'L' : 'M'}${x + j(2)},${y + j(2)}`).join(' ') + 'Z'
      path(d, o, o.fill).setAttribute('opacity', o.opacity ?? 1)
    }

    const api = {
      line(x1, y1, x2, y2, o = {}) {
        return twice(() => stroke(x1, y1, x2, y2), o)
      },

      arrow(x1, y1, x2, y2, o = {}) {
        api.line(x1, y1, x2, y2, o)
        const a = Math.atan2(y2 - y1, x2 - x1)
        const h = o.head || 12
        for (const s of [-1, 1]) {
          const t = a + Math.PI + s * 0.45
          api.line(x2, y2, x2 + h * Math.cos(t), y2 + h * Math.sin(t), o)
        }
        if (o.label) api.text((x1 + x2) / 2, (y1 + y2) / 2 - 18, o.label, { size: 20, ...o, label: null })
      },

      box(x, y, w, h, o = {}) {
        const r = o.radius || 0
        fillPoly([[x, y], [x + w, y], [x + w, y + h], [x, y + h]], o)
        if (clean) {
          add('rect', { x, y, width: w, height: h, rx: r || 10, fill: 'none', stroke: o.stroke || ink, 'stroke-width': o.width || 2 })
        } else {
          api.line(x, y, x + w, y, o)
          api.line(x + w, y, x + w, y + h, o)
          api.line(x + w, y + h, x, y + h, o)
          api.line(x, y + h, x, y, o)
        }
        if (o.label) api.text(x + w / 2, y + h / 2, o.label, { size: o.size || 24, stroke: o.stroke })
      },

      circle(cx, cy, rx, ry = rx, o = {}) {
        const pts = (wob) => {
          const n = 24
          const start = rand() * Math.PI * 2
          const out = []
          for (let i = 0; i <= n + (clean ? 0 : 2); i++) {
            const t = start + (i / n) * Math.PI * 2
            const k = 1 + j(wob)
            out.push([cx + Math.cos(t) * rx * k, cy + Math.sin(t) * ry * k])
          }
          return out
        }
        if (o.fill) fillPoly(pts(0.02), o)
        twice(() => smooth(pts(0.03)), o)
        if (o.label) api.text(cx, cy, o.label, { size: o.size || 24, stroke: o.stroke })
      },

      poly(points, o = {}) {
        if (o.close) fillPoly(points, o)
        const list = o.close ? [...points, points[0]] : points
        for (let i = 1; i < list.length; i++) api.line(...list[i - 1], ...list[i], o)
      },

      curve(points, o = {}) {
        if (o.close) fillPoly(points, o)
        return twice(() => smooth(points.map(([x, y]) => [x + j(1.5), y + j(1.5)])), o)
      },

      text(x, y, str, o = {}) {
        const el = add('text', {
          x,
          y,
          fill: o.stroke || ink,
          'font-family': o.font || font,
          'font-size': (o.size || 24) * (clean ? 0.72 : 1),
          'font-weight': clean ? 500 : 600,
          'text-anchor': o.anchor || 'middle',
          'dominant-baseline': 'central',
        })
        el.textContent = str
        return el
      },
    }

    // smooth draws a curve through points with Catmull-Rom segments
    function smooth(p) {
      let d = `M${p[0][0]},${p[0][1]}`
      for (let i = 0; i < p.length - 1; i++) {
        const [a, b, c, e] = [p[i - 1] || p[i], p[i], p[i + 1], p[i + 2] || p[i + 1]]
        d += ` C${b[0] + (c[0] - a[0]) / 6},${b[1] + (c[1] - a[1]) / 6} ${c[0] - (e[0] - b[0]) / 6},${c[1] - (e[1] - b[1]) / 6} ${c[0]},${c[1]}`
      }
      return d
    }

    return api
  }
})()
