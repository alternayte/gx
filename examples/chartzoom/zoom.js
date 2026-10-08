// The behaviour of the chartzoom:zoom directive. The compiler writes the
// level into data-chartzoom; this module makes the element larger by it.
const apply = () => {
  for (const el of document.querySelectorAll('[data-chartzoom]:not([data-chartzoom-live])')) {
    el.setAttribute('data-chartzoom-live', '')
    el.style.transformOrigin = 'top left'
    el.style.transform = `scale(${parseFloat(el.getAttribute('data-chartzoom')) || 1})`
  }
}
apply()
new MutationObserver(apply).observe(document.documentElement, { childList: true, subtree: true })
