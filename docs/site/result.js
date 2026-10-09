// A result page is a capture of a page of a sample app. It has the look of
// the page and no server. The script gives the frame the height of the
// page, follows the theme of the docs site, and stops each link, form and
// action with a note.
;(() => {
  try {
    const mode = localStorage.getItem('gx-theme')
    if (mode === 'dark' || mode === 'light') document.documentElement.classList.add(mode)
  } catch {
    // Private mode has no storage; the page follows the system theme.
  }
  const frame = window.frameElement
  if (frame) {
    const rem = parseFloat(getComputedStyle(document.documentElement).fontSize)
    const fit = () => {
      const height = Math.min(Math.max(document.body.scrollHeight, 6 * rem), 40 * rem)
      frame.style.height = Math.ceil(height) + 'px'
    }
    new ResizeObserver(fit).observe(document.body)
    fit()
  }
  const note = document.getElementById('gx-result-note')
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
  const server = 'This page is a capture. In the app, the server answers this.'
  document.addEventListener('click', (e) => {
    const at = e.target instanceof Element ? e.target : null
    const link = at && at.closest('a[href]')
    if (link) {
      e.preventDefault()
      const url = new URL(link.getAttribute('href'), window.location.href)
      const path = url.origin === window.location.origin ? url.pathname + url.search : url.href
      say('This page is a capture. In the app, this link opens ' + path + '.')
      return
    }
    // A button outside a form runs an action of the app.
    if (at && at.closest('button') && !at.closest('form')) say(server)
  }, true)
  document.addEventListener('submit', (e) => {
    e.preventDefault()
    say(server)
  }, true)
})()
