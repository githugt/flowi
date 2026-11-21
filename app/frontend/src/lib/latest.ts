// Tells a load whether it is still the newest one, so an answer that comes late does not replace a newer one.
// Before: on the Checks screen, results measured for one collection stayed (or came back) after another was chosen.
//
//   const start = latest()
//   async function load() { const current = start(); const data = await fetch(); if (!current()) return; show(data) }

/** Each call of the returned function starts a load and gives back a check that is true until the next load starts. */
export function latest(): () => () => boolean {
  let n = 0
  return () => {
    const mine = ++n
    return () => mine === n
  }
}
