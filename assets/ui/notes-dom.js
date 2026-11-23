// Notes: find what the user clicked, and find it again. A page can hold frames, such
// as the options of a board, so a note may point into a frame of the page.

function findEl(doc, selector) {
  let el = null
  for (const part of (selector || '').split(' >>> ')) {
    try {
      el = (el ? frameDoc(el) : doc)?.querySelector(part)
    } catch (e) {
      return null // a bad selector
    }
    if (!el) return null
  }
  return el
}

// rectIn is the box of el in the coordinates of doc, through frames that are scaled down.
// It is null when el is scrolled out of its frame.
function rectIn(el, doc) {
  let { left, top, width, height } = el.getBoundingClientRect()
  for (let d = el.ownerDocument; d !== doc; ) {
    const frame = d.defaultView?.frameElement
    if (!frame) return null
    const f = frame.getBoundingClientRect()
    const scale = frame.clientWidth ? f.width / frame.clientWidth : 1
    if (left + width < 0 || top + height < 0 || left > frame.clientWidth || top > frame.clientHeight) return null
    left = f.left + (frame.clientLeft + left) * scale
    top = f.top + (frame.clientTop + top) * scale
    width *= scale
    height *= scale
    d = frame.ownerDocument
  }
  return { left, top, width, height }
}

function frameDoc(frame) {
  try {
    return frame.contentDocument
  } catch (e) {
    return null
  }
}

const NAMED = new Set(['a', 'button', 'input', 'select', 'textarea', 'img', 'label', 'h1', 'h2', 'h3', 'h4', 'h5', 'h6'])

// describe names an element for the agent: button "Start trial", or .price "₹1,999".
function describe(el) {
  const tag = el.tagName.toLowerCase()
  const name = NAMED.has(tag) || !el.classList.length ? tag : '.' + el.classList[0]
  const text = (el.getAttribute('aria-label') || el.alt || el.placeholder || el.innerText || '').trim().replace(/\s+/g, ' ')
  return text ? `${name} "${text.length > 40 ? text.slice(0, 39) + '…' : text}"` : name
}

// cssPath is a selector that finds el again: from the nearest id, or from body.
function cssPath(el) {
  const parts = []
  for (let x = el; x && x.nodeType === 1 && x.tagName !== 'BODY' && x.tagName !== 'HTML'; x = x.parentElement) {
    if (x.id && /^[A-Za-z][\w-]*$/.test(x.id)) {
      parts.unshift('#' + x.id)
      break
    }
    const same = [...x.parentElement.children].filter((c) => c.tagName === x.tagName)
    const tag = x.tagName.toLowerCase()
    parts.unshift(same.length > 1 ? `${tag}:nth-of-type(${same.indexOf(x) + 1})` : tag)
  }
  return parts.join(' > ') || 'body'
}

// framePath is cssPath through the frames of the page: "iframe:nth-of-type(2) >>> button".
function framePath(el) {
  const parts = [cssPath(el)]
  for (let f = el.ownerDocument.defaultView.frameElement; f && f.ownerDocument !== document; f = f.ownerDocument.defaultView.frameElement) {
    parts.unshift(cssPath(f))
  }
  return parts.join(' >>> ')
}

// inFrame names the frame page of el, so the agent knows which option the user means.
function inFrame(el) {
  const frame = el.ownerDocument.defaultView.frameElement
  return frame && frame.ownerDocument !== document ? ` in ${frame.getAttribute('src') || 'a frame'}` : ''
}

const round = (v) => Math.round(Math.min(1, Math.max(0, v)) * 1000) / 1000
