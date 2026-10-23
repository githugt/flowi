<script lang="ts">
  import { errorText } from './lib/errors'
  import { FlowService } from '../bindings/github.com/githugt/flowi/internal/bridge'
  import type { FlowView } from './lib/types'

  let { view }: { view: FlowView } = $props()
  const node = $derived(view.node)
  let open = $state(false)
  let asking = $state<null | { scope: string }>(null)
  let values = $state<Record<string, string>>({})
  let error = $state('')

  const inputs = $derived(view.form.flow.inputs)
  const needsInputs = $derived(inputs.some((i) => i.value === ''))

  async function launch(scope: string, force: boolean) {
    open = false
    error = ''
    if (inputs.length && (force || needsInputs)) {
      values = {}
      asking = { scope }
      return
    }
    await go(scope, {})
  }

  async function go(scope: string, given: Record<string, string>) {
    const filled = Object.fromEntries(Object.entries(given).filter(([, v]) => v !== ''))
    try {
      if (scope === 'all') await FlowService.Run(node, filled)
      else await FlowService.RunPart(node, scope, filled)
    } catch (e) {
      error = errorText(e)
    }
  }

  async function submit() {
    const scope = asking!.scope
    asking = null
    await go(scope, values)
  }

  const choices: [string, string, boolean][] = [
    ['Run', 'all', false],
    ['Run with inputs…', 'all', true],
    ['Run only the selected step', 'only', false],
    ['Run until the selected step', 'until', false],
    ['Run from the selected step', 'from', false],
  ]
</script>

<svelte:window onpointerdown={() => (open = false)} />

<div class="run">
  {#if view.run.running}
    <button class="primary" onclick={() => FlowService.Cancel(node)}>Cancel</button>
  {:else}
    <button class="primary" onclick={() => launch('all', false)}>▶ Run</button>
    <button class="more" aria-label="More ways to run" onpointerdown={(e) => e.stopPropagation()} onclick={() => (open = !open)}>▾</button>
  {/if}
  {#if open}
    <div class="menu" role="menu" tabindex="-1" onpointerdown={(e) => e.stopPropagation()}>
      {#each choices as [label, scope, force]}
        <button role="menuitem" disabled={scope !== 'all' && !view.selected} onclick={() => launch(scope, force)}>{label}</button>
      {/each}
      <hr />
      <button role="menuitem" onclick={() => { open = false; FlowService.ClearResults(node) }}>Clear results</button>
    </div>
  {/if}
</div>
{#if error}<span class="err">{error}</span>{/if}

{#if asking}
  <div class="scrim" role="presentation" onpointerdown={() => (asking = null)}>
    <form class="dialog" onpointerdown={(e) => e.stopPropagation()} onsubmit={(e) => { e.preventDefault(); submit() }}>
      <h3>Run flow</h3>
      {#each inputs as i}
        <label>
          <span>{i.key}</span>
          <input placeholder={i.value} bind:value={values[i.key]} spellcheck="false" />
        </label>
      {/each}
      <div class="actions">
        <button type="button" onclick={() => (asking = null)}>Cancel</button>
        <button class="primary" type="submit">Run</button>
      </div>
    </form>
  </div>
{/if}

<style>
  .run { position: relative; display: flex; gap: 2px; }
  button {
    background: transparent;
    color: var(--text);
    border: 1px solid var(--line);
    border-radius: var(--r-md);
    padding: 3px 12px;
    font: inherit;
  }
  button:hover:not(:disabled) { border-color: var(--accent); }
  button:disabled { opacity: 0.4; }
  /* the same strong blue as a request's Send, not the paler accent */
  .primary { background: var(--act-send); color: var(--on-act); border-color: var(--act-send); font-weight: 600; }
  .primary:hover:not(:disabled) { border-color: var(--act-send); background: color-mix(in srgb, var(--act-send) 88%, white); }
  .more { padding: 3px 8px; }
  .menu {
    position: absolute;
    top: 100%;
    right: 0;
    margin-top: 4px;
    min-width: 230px;
    display: flex;
    flex-direction: column;
    padding: 4px;
    background: var(--panel);
    border: 1px solid var(--line);
    border-radius: var(--r-lg);
    box-shadow: var(--shadow);
    z-index: 20;
  }
  .menu button { border: 0; text-align: left; }
  .menu button:hover:not(:disabled) { background: color-mix(in srgb, var(--accent) 18%, transparent); }
  hr { width: 100%; border: 0; border-top: 1px solid var(--line); margin: 4px 0; }
  .err { color: var(--fail); font-size: var(--fs-sm); }
  .scrim { position: fixed; inset: 0; background: rgb(0 0 0 / 0.4); display: grid; place-items: center; z-index: 30; }
  .dialog {
    width: 360px;
    display: flex;
    flex-direction: column;
    gap: 10px;
    padding: 16px;
    background: var(--panel);
    border: 1px solid var(--line);
    border-radius: var(--r-lg);
  }
  .dialog h3 { margin: 0; }
  .dialog label { display: flex; flex-direction: column; gap: 3px; color: var(--muted); font-size: var(--fs-xs); }
  .dialog input {
    background: var(--bg);
    color: var(--text);
    border: 1px solid var(--line);
    border-radius: var(--r-md);
    padding: 5px 8px;
    font: inherit;
    -webkit-user-select: text;
    user-select: text;
  }
  .actions { display: flex; justify-content: flex-end; gap: 8px; }
</style>
