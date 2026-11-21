<script lang="ts">
  import ScreenPage from './ScreenPage.svelte'
  import { onMount } from 'svelte'
  import { WelcomeService } from '../bindings/github.com/githugt/flowi/app/internal/bridge'
  import { live } from './lib/live'
  import { LEVEL_CARDS } from './lib/telemetrycards'
  import { radiogroup } from './lib/radiogroup'

  interface TelemetryView {
    explanation: string
    level: string
    problem?: string
    previews: Record<string, string>
  }
  let view = $state<TelemetryView | null>(null)
  onMount(() =>
    live<TelemetryView>('telemetry', () => WelcomeService.Telemetry(), (v) => {
      view = v
    }),
  )
</script>

{#if view}
  <ScreenPage width={760}>
    <h2>Usage reporting</h2>
    <p class="expl">{view.explanation}</p>
    <div class="levels" role="radiogroup" aria-label="Usage reporting level" use:radiogroup>
      {#each LEVEL_CARDS as c (c.id)}
        <div class="card" class:on={view.level === c.id}>
          <button class="pick" role="radio" aria-checked={view.level === c.id} onclick={() => WelcomeService.ChooseTelemetry(c.id)}>
            <strong>{c.label}</strong>
            <span class="sum">{c.summary}</span>
          </button>
          <details>
            <summary>Show exactly what is sent</summary>
            {#if view.previews[c.id]}<pre>{view.previews[c.id]}</pre>{:else}<p class="none">Nothing.</p>{/if}
          </details>
        </div>
      {/each}
    </div>
    {#if view.problem}<p class="err">{view.problem}</p>{/if}
  </ScreenPage>
{/if}

<style>
  h2 { margin: 0 0 8px; }
  .expl { white-space: pre-wrap; line-height: 1.5; }
  .levels { display: flex; flex-direction: column; gap: 10px; margin: 12px 0; }
  .card { border: 1px solid var(--line); border-radius: var(--r-lg); padding: 4px 12px 8px; }
  .card.on { border-color: var(--accent); background: var(--tint-sel); }
  .pick { display: flex; flex-direction: column; align-items: flex-start; gap: 2px; width: 100%; padding: 8px 0 4px; background: none; border: 0; color: var(--text); font: inherit; text-align: left; }
  .sum { color: var(--muted); font-size: var(--fs-sm); }
  summary { cursor: pointer; color: var(--muted); font-size: var(--fs-sm); }
  .none { color: var(--muted); margin: 6px 0 0; }
  .err { color: var(--fail); }
  pre { background: var(--bg); border: 1px solid var(--line); border-radius: var(--r-lg); padding: 10px; font: var(--fs-sm) var(--mono); overflow: auto; -webkit-user-select: text; user-select: text; }
</style>
