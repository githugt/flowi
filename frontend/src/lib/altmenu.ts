// A lone press of Alt, as Windows programs use it to show their menu bar: Alt goes down and comes back up
// with no other key or click in between. Alt+Tab, Alt+Left and Alt-clicks are not taps.

export interface AltTap {
  keydown(e: Pick<KeyboardEvent, 'key' | 'repeat'>): void
  keyup(e: Pick<KeyboardEvent, 'key'>): void
  /** Anything else that happens while Alt is down: a click, or the window losing focus. */
  cancel(): void
}

export function createAltTap(ontap: () => void): AltTap {
  let armed = false
  return {
    keydown(e) {
      // Holding Alt repeats the event; the repeats neither start nor spoil a tap.
      if (e.key === 'Alt') armed = armed || !e.repeat
      else armed = false
    },
    keyup(e) {
      const tapped = armed && e.key === 'Alt'
      armed = false
      if (tapped) ontap()
    },
    cancel() {
      armed = false
    },
  }
}

export function isWindows(userAgent: string): boolean {
  return /Windows/i.test(userAgent)
}
