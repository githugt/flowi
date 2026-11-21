<script lang="ts">
  // A titled section of a panel that folds away, and stays as the person left it. `name` keys the choice, so the same
  // section is open or shut for every step; `open` is how it starts the first time.
  // Before: the step inspector hid its most-used fields (Run when, Timeout, On failure) in a section that opened shut.
  import type { Snippet } from 'svelte'
  import { readFold, writeFold, type FoldStorage } from './lib/foldstate'

  let { name, title, open = false, children }: { name: string; title: string; open?: boolean; children: Snippet } = $props()

  // Reaching for the store can itself throw (blocked site data), so it is looked up inside the guard.
  function store(): FoldStorage | undefined {
    try {
      return localStorage
    } catch {
      return undefined
    }
  }
  // svelte-ignore state_referenced_locally
  let shown = $state(readFold(store(), name, open))

  function toggled(e: Event) {
    const now = (e.currentTarget as HTMLDetailsElement).open
    if (now === shown) return
    shown = now
    writeFold(store(), name, now)
  }
</script>

<details open={shown} ontoggle={toggled}>
  <summary>{title}</summary>
  {@render children()}
</details>

<style>
  details { display: flex; flex-direction: column; gap: 8px; }
  summary { cursor: default; font-weight: 600; font-size: var(--fs-sm); margin-bottom: 4px; }
</style>
