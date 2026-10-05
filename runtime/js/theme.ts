// The theme script. The app links it in the head as a classic script, with
// no defer, on a page that has a theme control. It sets the stored theme
// class on the html element before the first paint. gx.ts owns the theme
// select buttons.
try {
  const mode = localStorage.getItem('gx-theme')
  if (mode === 'dark' || mode === 'light') document.documentElement.classList.add(mode)
} catch {
  // Private mode has no storage; the page follows the system theme.
}
