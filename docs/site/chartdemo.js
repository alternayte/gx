// The button of the chart demo puts the next set of numbers into the props
// attribute of the island. The island loader then calls the update export
// of the island, as it does after a patch from a server.
(() => {
  for (const demo of document.querySelectorAll('[data-chart-demo]:not([data-chart-ready])')) {
    demo.setAttribute('data-chart-ready', '')
    const sets = JSON.parse(demo.getAttribute('data-sets'))
    let at = 0
    demo.querySelector('[data-chart-next]').addEventListener('click', () => {
      at = (at + 1) % sets.length
      demo.querySelector('gx-island').setAttribute('props', JSON.stringify(sets[at]))
    })
  }
})()
