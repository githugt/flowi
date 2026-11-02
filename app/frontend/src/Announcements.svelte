<script lang="ts">
  // Notices from the project's signed feed. Quiet by design: with no feed, or
  // announcements off, nothing shows.
  import { onMount } from 'svelte'
  import { Browser } from '@wailsio/runtime'
  import { WelcomeService } from '../bindings/github.com/githugt/flowi/app/internal/bridge'
  import { live } from './lib/live'

  interface Item {
    id: string
    title: string
    body: string
    url: string
    level: string
  }
  let items = $state<Item[]>([])
  onMount(() => live<{ announcements: Item[] }>('welcome', () => WelcomeService.State(), (v) => (items = v.announcements)))
</script>

{#each items.slice(0, 1) as it (it.id)}
  <div class="note {it.level}" role="status">
    <div class="text"><strong>{it.title}</strong>{#if it.body} <span>{it.body}</span>{/if}</div>
    {#if it.url}<button onclick={() => Browser.OpenURL(it.url)}>Read more</button>{/if}
    <button onclick={() => WelcomeService.Dismiss(it.id)}>Got it</button>
    <button class="quiet" onclick={() => WelcomeService.TurnOffAnnouncements()}>Turn off announcements</button>
  </div>
{/each}

<style>
  .note { display: flex; align-items: center; gap: 8px; padding: 6px 12px; background: color-mix(in srgb, var(--accent) 14%, var(--panel)); border-bottom: 1px solid var(--line); font-size: var(--fs-sm); }
  .note.warning { background: color-mix(in srgb, var(--warn) 18%, var(--panel)); }
  .text { flex: 1; }
  button { background: var(--raised); color: var(--text); border: 0; box-shadow: inset 0 0 0 1px var(--line); border-radius: var(--r-md); padding: 0 10px; min-height: var(--ctl-h-sm); font: inherit; font-weight: 500; font-size: var(--fs-sm); }
  button:hover:not(:disabled) { background: var(--raised-3); }
  .quiet { color: var(--muted); border: 0; }
</style>
