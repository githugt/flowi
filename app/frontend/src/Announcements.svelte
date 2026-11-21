<script lang="ts">
  import Banner from './Banner.svelte'
  import Button from './Button.svelte'
  // Notices from the project's signed feed. Quiet by design: with no feed, or
  // announcements off, nothing shows.
  import { onMount } from 'svelte'
  import { openExternal } from './lib/openurl'
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
  <Banner tone={it.level === 'warning' ? 'warn' : 'info'}>
    <strong>{it.title}</strong>{#if it.body} <span>{it.body}</span>{/if}
    {#snippet actions()}
      {#if it.url}<Button small onclick={() => openExternal(it.url)}>Read more</Button>{/if}
      <Button small onclick={() => WelcomeService.Dismiss(it.id)}>Got it</Button>
      <Button small variant="ghost" onclick={() => WelcomeService.TurnOffAnnouncements()}>Turn off announcements</Button>
    {/snippet}
  </Banner>
{/each}
