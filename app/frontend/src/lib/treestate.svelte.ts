import { loadFolded, saveFolded, toggleFolded } from './folded'

/**
 * The groups of the tree the person folded. Before: each row kept its own open flag, so every rebuild of the tree
 * (a save, a rename, a new file) opened all groups again.
 */
class TreeState {
  folded = $state<string[]>(loadFolded())

  isOpen(id: string): boolean {
    return !this.folded.includes(id)
  }

  toggle(id: string) {
    this.folded = toggleFolded(this.folded, id)
    saveFolded(this.folded)
  }
}

export const treeState = new TreeState()
