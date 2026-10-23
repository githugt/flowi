<script lang="ts">
  import { onMount } from 'svelte'
  import { WelcomeService } from '../bindings/github.com/githugt/flowi/internal/bridge'
  import { live } from './lib/live'

  interface TelemetryView {
    explanation: string
    level: string
    problem?: string
    previews: Record<string, string>
  }
  let view = $state<TelemetryView | null>(null)
  let looking = $state('off')
  onMount(() =>
    live<TelemetryView>('telemetry', () => WelcomeService.Telemetry(), (v) => {
      if (!view) looking = v.level
      view = v
    }),
  )
  const levels: [string, string][] = [['off', 'Off'], ['minimal', 'Minimal'], ['full', 'Full']]
</script>

{#if view}
  <div class="page">
    <h2>Usage reporting</h2>
    <p class="expl">{view.explanation}</p>
    <div class="levels">
      {#each levels as [id, label]}
        <label class="lv" class:on={view.level === id}>
          <input type="radio" name="level" checked={view.level === id} onchange={() => WelcomeService.ChooseTelemetry(id)} onfocus={() => (looking = id)} />
          {label}
        </label>
      {/each}
    </div>
    {#if view.problem}<p class="err">{view.problem}</p>{/if}
    <h4>What “{looking}” would send</h4>
    <div class="tabs">{#each levels as [id, label]}<button class:on={looking === id} onclick={() => (looking = id)}>{label}</button>{/each}</div>
    <pre>{view.previews[looking]}</pre>
  </div>
{/if}

<style>
  .page { position: absolute; inset: 0; overflow: auto; padding: 20px 28px 40px; max-width: 760px; }
  h2 { margin: 0 0 8px; }
  .expl { white-space: pre-wrap; line-height: 1.5; }
  .levels { display: flex; gap: 10px; margin: 12px 0; }
  .lv { display: flex; gap: 6px; align-items: center; padding: 6px 14px; border: 1px solid var(--line); border-radius: var(--r-lg); }
  .lv.on { border-color: var(--accent); background: color-mix(in srgb, var(--accent) 14%, transparent); }
  .err { color: var(--fail); }
  h4 { margin: 18px 0 6px; }
  .tabs { display: flex; gap: 4px; margin-bottom: 6px; }
  .tabs button { background: transparent; color: var(--muted); border: 1px solid var(--line); border-radius: var(--r-md); padding: 2px 12px; font: inherit; }
  .tabs .on { color: var(--text); border-color: var(--accent); }
  pre { background: var(--bg); border: 1px solid var(--line); border-radius: var(--r-lg); padding: 10px; font: var(--fs-sm) var(--mono); overflow: auto; -webkit-user-select: text; user-select: text; }
</style>
