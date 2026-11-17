<script lang="ts">
  // Sends the saved request once for every combination of environments, overlays and the values of one of its
  // {{names}}, and shows a grid of passes and failures. The responses are not kept: a matrix is a check.
  import { errorText } from './lib/errors'
  import { RequestService } from '../bindings/github.com/githugt/flowi/app/internal/bridge'
  import MatrixResult from './MatrixResult.svelte'
  import type { MatrixView, RequestMatrixChoices } from './lib/types'

  let { node, onclose }: { node: string; onclose: () => void } = $props()

  let choices = $state<RequestMatrixChoices>({ envs: [], overlays: [], names: [] })
  let byEnv = $state<string[]>([])
  let byOverlay = $state<string[]>([])
  let valueName = $state('')
  let values = $state('')
  let matrix = $state<MatrixView | null>(null)
  let busy = $state(false)
  let error = $state('')

  $effect(() => {
    RequestService.MatrixChoices(node).then((c) => (choices = c as unknown as RequestMatrixChoices), (e) => (error = errorText(e)))
  })

  const toggle = (list: string[], v: string): string[] => (list.includes(v) ? list.filter((x) => x !== v) : [...list, v])
  const dims = $derived.by(() => {
    const out: { name: string; values: string[] }[] = []
    if (byEnv.length) out.push({ name: 'env', values: byEnv })
    if (byOverlay.length) out.push({ name: 'overlay', values: byOverlay })
    const vals = values.split(',').map((v) => v.trim()).filter(Boolean)
    if (valueName.trim() && vals.length) out.push({ name: `with.${valueName.trim()}`, values: vals })
    return out
  })
  const cellCount = $derived(dims.reduce((n, d) => n * d.values.length, 1))

  async function run() {
    busy = true
    error = ''
    matrix = null
    try {
      matrix = (await RequestService.RunMatrix(node, dims)) as unknown as MatrixView
    } catch (e) {
      error = errorText(e)
    } finally {
      busy = false
    }
  }
</script>

<div class="scrim" role="presentation" onpointerdown={onclose}>
  <div class="dialog" role="dialog" aria-label="Run a matrix" tabindex="-1" onpointerdown={(e) => e.stopPropagation()}>
    <h3>Run a matrix</h3>
    <p class="note">Sends the saved request once for every combination you pick, one after the other, and shows a grid of passes and failures. The responses are not saved. Pick at least one thing to vary.</p>
    <div class="group"><strong>Environments</strong>
      {#each choices.envs as e}<label class="opt"><input type="checkbox" checked={byEnv.includes(e)} onchange={() => (byEnv = toggle(byEnv, e))} /> {e}</label>{/each}
      {#if !choices.envs.length}<span class="muted">none yet</span>{/if}
    </div>
    <div class="group"><strong>Overlays</strong> <span class="muted">laid over the request</span>
      {#each choices.overlays as o}<label class="opt"><input type="checkbox" checked={byOverlay.includes(o)} onchange={() => (byOverlay = toggle(byOverlay, o))} /> {o}</label>{/each}
      {#if !choices.overlays.length}<span class="muted">this collection has none</span>{/if}
    </div>
    <div class="group"><strong>A value</strong> <span class="muted">for a name the request uses, such as {'{{id}}'}</span>
      <input list="matrix-names" placeholder="name" bind:value={valueName} spellcheck="false" />
      <datalist id="matrix-names">{#each choices.names as n}<option value={n}></option>{/each}</datalist>
      {#if valueName}<input placeholder="values, separated by commas" bind:value={values} spellcheck="false" />{/if}
    </div>
    <div class="actions">
      <span class="muted">{dims.length ? `${cellCount} send${cellCount > 1 ? 's' : ''}` : ''}</span>
      <button class="primary" disabled={busy || !dims.length || cellCount > 100} onclick={run}>{busy ? 'Sending…' : 'Run'}</button>
      <button onclick={onclose}>Close</button>
    </div>
    {#if matrix}<MatrixResult {matrix} />{/if}
    {#if error}<p class="bad">{error}</p>{/if}
  </div>
</div>

<style>
  button { background: var(--raised); color: var(--text); border: 0; box-shadow: inset 0 0 0 1px var(--line); border-radius: var(--r-md); padding: 0 12px; min-height: var(--ctl-h); font: inherit; font-weight: 500; }
  button:hover:not(:disabled) { background: var(--raised-3); }
  button:disabled { opacity: 0.4; }
  .primary { background: var(--send-fill); color: var(--on-send); border: 0; font-weight: 600; box-shadow: var(--send-edge), 0 4px 14px var(--send-glow); }
  .scrim { position: fixed; inset: 0; z-index: 40; background: rgba(0, 0, 0, 0.45); display: flex; align-items: center; justify-content: center; }
  .dialog { width: min(760px, 94vw); max-height: 88vh; overflow: auto; background: var(--panel); border: 1px solid var(--line); border-radius: 14px; padding: 16px 20px; display: flex; flex-direction: column; gap: 8px; box-shadow: var(--shadow); }
  h3 { margin: 0; }
  .note, .muted { color: var(--muted); font-size: var(--fs-sm); margin: 0; }
  .group { display: flex; flex-wrap: wrap; align-items: center; gap: 4px 14px; font-size: var(--fs-sm); }
  .group > strong { min-width: 90px; }
  .opt { display: flex; align-items: center; gap: 4px; font-size: var(--fs-sm); }
  input:not([type]) { padding: 3px 8px; }
  .actions { display: flex; align-items: center; justify-content: flex-end; gap: 8px; margin-top: 4px; }
  .actions .muted { margin-right: auto; }
  .bad { color: var(--fail); margin: 2px 0; }
</style>
