// Svelte action: moves an element to the end of <body> while it is shown, and takes it away with the component.
// For what floats over the whole window (a popover, a tip): placed where the component is, it can be cut off by a panel
// that scrolls or clips around it, and it stacks under whatever sits above that panel. Fixed positioning alone did not
// prevent either one in the web view, where a hint opened inside a card's panel was cut off at the panel's edge.
// Before: the hint of a field in a card's panel was clipped at that panel's right edge.
export function portal(node: HTMLElement) {
  document.body.appendChild(node)
  return {
    destroy() {
      node.remove()
    },
  }
}
