// The toast preview has no action to push its toast. A press of the button
// copies the toast of the template into the toaster; the behaviour runtime
// of the toast item then runs it like a pushed toast.
document.addEventListener('click', (e) => {
  if (!e.target.closest('[data-docs-toast]')) return
  const source = document.querySelector('template[data-docs-toast-source]')
  const toaster = document.querySelector('[data-gx-toaster]')
  if (source && toaster) toaster.append(source.content.cloneNode(true))
})
