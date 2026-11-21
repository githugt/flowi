<script lang="ts">
  import Banner from './Banner.svelte'
  // A thin bar while Go has not answered for a moment, and a notice when it has not for five seconds. A long request
  // is also a call that waits, so the notice says "still waiting", not "stuck".
  import { busy } from './lib/busy.svelte'
  import { copy } from './lib/clip'
  import { SystemService } from '../bindings/github.com/githugt/flowi/app/internal/bridge'
  import Button from './Button.svelte'

  async function copyDiagnostics() {
    // Go may be the thing that is stuck: ask it, but not for long, and copy what the window knows either way.
    const fromGo = await Promise.race([Promise.resolve(SystemService.Diagnostics()).catch(() => ''), new Promise<string>((r) => setTimeout(() => r(''), 2000))])
    await copy(busy.diagnostics(fromGo), 'the diagnostics')
  }
</script>

{#if busy.state.level !== 'idle'}
  <div class="bar" aria-hidden="true"></div>
{/if}
{#if busy.state.level === 'stuck'}
  <Banner tone="warn">
    Still waiting for Flowi: a call has run for {busy.state.seconds} s. It may be a long request, or Flowi may be stuck.
    {#snippet actions()}
      <Button small variant="ghost" onclick={copyDiagnostics}>Copy diagnostics</Button>
    {/snippet}
  </Banner>
{/if}

<style>
  .bar { position: fixed; top: 0; left: 0; right: 0; height: 2px; z-index: var(--z-top); overflow: hidden; pointer-events: none; }
  .bar::after { content: ''; position: absolute; inset: 0 auto 0 0; width: 30%; background: var(--accent); animation: slide 1.1s ease-in-out infinite; }
  @keyframes slide { from { transform: translateX(-100%); } to { transform: translateX(340%); } }
  @media (prefers-reduced-motion: reduce) { .bar::after { animation: none; width: 100%; opacity: 0.6; } }
</style>
