// What the Manage environments screen offers beside the list: one card for each thing to do with environments. A new
// environment feature adds an entry here (and its screen to ./screens), and the launchpad shows it; the screen itself
// does not change.
import { app } from './app.svelte'
import { compareChoice } from './envgroup.svelte'
import { promptNewEnvironment } from './menus'

export interface EnvTool {
  id: string
  title: string
  /** One line on what it does. */
  hint: string
  /** An Icon name. */
  icon: string
  run: () => void
}

/** Opens Compare environments on a group ("" is the default group). */
export function openCompare(group: string) {
  compareChoice.group = group
  app.openScreen('screen:envcompare', 'Compare environments')
}

export const envTools: EnvTool[] = [
  { id: 'new', title: 'New environment', hint: 'Start an empty one, then fill in its variables.', icon: 'plus', run: promptNewEnvironment },
  { id: 'compare', title: 'Compare environments', hint: 'Every variable of a group side by side.', icon: 'key', run: () => openCompare(compareChoice.group ?? '') },
  { id: 'secrets', title: 'Secrets', hint: 'Which secrets have a value in the vault.', icon: 'key', run: () => app.openScreen('screen:secrets', 'Secrets') },
]
