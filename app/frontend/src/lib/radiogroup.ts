// A group of radios (swatches, the View switch, "Which calls") is one tab stop: Tab enters at the chosen one and
// the arrow keys move inside it. Before: every swatch was a tab stop and the arrows did nothing, so a person
// without a mouse tabbed through nine colours to leave the row.

/** The index an arrow, Home or End key moves to inside a group, wrapping round; null for any other key. */
export function radioKeyTarget(key: string, index: number, length: number): number | null {
  if (length <= 0) return null
  switch (key) {
    case 'ArrowRight':
    case 'ArrowDown':
      return (index + 1) % length
    case 'ArrowLeft':
    case 'ArrowUp':
      return (index - 1 + length) % length
    case 'Home':
      return 0
    case 'End':
      return length - 1
    default:
      return null
  }
}

/** The index that is the tab stop: the chosen radio, else the first. */
export function tabStop(checked: boolean[]): number {
  const i = checked.indexOf(true)
  return i < 0 ? 0 : i
}

const radios = (group: HTMLElement) => Array.from(group.querySelectorAll<HTMLElement>('[role="radio"]:not(:disabled)'))

/** Svelte action for an element with role="radiogroup": roving tabindex and arrow keys that choose. */
export function radiogroup(group: HTMLElement) {
  const sync = () => {
    const all = radios(group)
    const stop = tabStop(all.map((r) => r.getAttribute('aria-checked') === 'true'))
    all.forEach((r, i) => r.setAttribute('tabindex', i === stop ? '0' : '-1'))
  }
  const onkeydown = (e: KeyboardEvent) => {
    const all = radios(group)
    const at = all.findIndex((r) => r === document.activeElement)
    if (at < 0) return
    const to = radioKeyTarget(e.key, at, all.length)
    if (to === null) return
    e.preventDefault()
    all[to].focus()
    all[to].click() // a radio chooses when it is reached
  }
  group.addEventListener('keydown', onkeydown)
  const watch = new MutationObserver(sync)
  watch.observe(group, { subtree: true, childList: true, attributes: true, attributeFilter: ['aria-checked', 'disabled'] })
  sync()
  return {
    destroy() {
      group.removeEventListener('keydown', onkeydown)
      watch.disconnect()
    },
  }
}
