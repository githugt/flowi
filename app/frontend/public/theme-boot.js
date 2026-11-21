// The last theme, before the first paint (src/lib/themecache.ts writes it).
// This is a file of its own, not an inline script in index.html, so that the page's Content-Security-Policy can
// say script-src 'self' without 'unsafe-inline' or a hash that breaks whenever this text changes.
try {
  var t = JSON.parse(localStorage.getItem('flowi.theme'))
  if (t && t.theme) {
    document.documentElement.dataset.theme = t.theme
    document.documentElement.dataset.mode = t.mode
  }
} catch (e) {}
