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
