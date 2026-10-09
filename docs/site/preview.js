// The frame of a component page takes the height of its example. An example
// with an overlay keeps a minimum height and sits at the top of its frame: a
// dialog, a popover and a fixed region add no height to the document. An
// open popover that ends below the frame makes the frame taller. The preview
// and the page have one origin, so the preview sets the height of its own
// frame.
(() => {
  const frame = window.frameElement && window.frameElement.parentElement
  const stage = document.querySelector('.gx-stage')
  const content = document.querySelector('.gx-stage-content')
  if (!frame || !stage || !content) return
  const overlay = document.querySelector('dialog, [popover], [data-gx-toaster], [role="tooltip"]')
  const rem = parseFloat(getComputedStyle(document.documentElement).fontSize)
  // A tooltip opens on any side of its trigger, so it stays in the centre.
  if (overlay && !overlay.matches('[role="tooltip"]')) stage.classList.add('gx-stage-top')
  let open = 0
  const fit = () => {
    const style = getComputedStyle(stage)
    const pad = parseFloat(style.paddingTop) + parseFloat(style.paddingBottom)
    const border = frame.offsetHeight - frame.clientHeight
    const height = Math.max(content.offsetHeight + pad, overlay ? 22 * rem : 0, open)
    frame.style.height = Math.ceil(height + border) + 'px'
  }
  // measure sets the room an open popover needs. A popover that the frame
  // cuts scrolls inside itself: its full height is the height of its
  // content and its borders.
  const measure = () => {
    open = 0
    for (const el of document.querySelectorAll(':popover-open')) {
      const box = el.getBoundingClientRect()
      open = Math.max(open, box.top + el.scrollHeight + (box.height - el.clientHeight) + rem)
    }
    fit()
  }
  document.addEventListener('toggle', (e) => {
    if (!(e.target instanceof HTMLElement) || !e.target.matches('[popover]')) return
    measure()
    // The popover takes its place and its size after the event: its
    // position follows the trigger, and its enter transition ends.
    requestAnimationFrame(measure)
    window.setTimeout(measure, 300)
  }, true)
  new ResizeObserver(fit).observe(content)
  fit()
})()

// An example stays in its frame. A link to a page of an app and a form that
// posts to an app have no target on this site: the preview stops them and
// says what an app does. A link to a different state of a live example is a
// page of this site, so it opens.
;(() => {
  const note = document.getElementById('gx-preview-note')
  if (!note) return
  let timer = 0
  const say = (text) => {
    note.textContent = text
    note.hidden = false
    window.clearTimeout(timer)
    timer = window.setTimeout(() => {
      note.hidden = true
    }, 5000)
  }
  document.addEventListener('click', (e) => {
    const link = e.target instanceof Element ? e.target.closest('a[href]') : null
    if (!link) return
    const url = new URL(link.getAttribute('href'), window.location.href)
    if (url.origin !== window.location.origin) {
      link.target = '_blank'
      link.rel = 'noopener'
      return
    }
    const here = url.pathname === window.location.pathname
    if (here && url.hash !== '') return
    if (!here && url.pathname.includes('/preview/')) return
    // The listener is first in the capture phase, so the navigation of the
    // runtime does not load the address.
    e.preventDefault()
    e.stopImmediatePropagation()
    say('In an app, this link opens ' + url.pathname + url.search + '. The preview has no such page.')
  }, true)
  // An action of an example is a request to the server of an app. The
  // preview answers it with no content.
  const send = window.fetch.bind(window)
  window.fetch = (input, init) => {
    const method = ((init && init.method) || (input instanceof Request ? input.method : 'GET')).toUpperCase()
    if (method === 'GET') return send(input, init)
    say('An app runs this action on its server. The preview has no server.')
    return Promise.resolve(new Response(null, { status: 204 }))
  }
  document.addEventListener('submit', (e) => {
    const form = e.target
    if (!(form instanceof HTMLFormElement)) return
    // The search form of a docs shell has its own handler.
    if (form.hasAttribute('data-gx-search-form') || form.method === 'dialog') return
    e.preventDefault()
    e.stopImmediatePropagation()
    say('An app sends this form to its server, and the server answers. The preview has no server.')
  }, true)
})()
