// The three levels of usage reporting, each with what it sends in one line (the wording follows the catalogue in
// lib/telemetry: keep them in step). The screen shows one card per level, with the exact payload under it.
// Before: a separate tab strip changed only a preview, so a click on "Full" there looked like a choice.

export interface LevelCard {
  id: string
  label: string
  summary: string
}

export const LEVEL_CARDS: LevelCard[] = [
  { id: 'off', label: 'Off', summary: 'Nothing is sent. This is the default.' },
  { id: 'minimal', label: 'Minimal', summary: 'Once per start: the app, its version, your operating system and CPU architecture, and the day.' },
  { id: 'full', label: 'Full', summary: 'Minimal, plus which command ran and how it ended, and for each run its kind, outcome, duration and step count as ranges.' },
]
