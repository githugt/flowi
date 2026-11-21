<script lang="ts">
  import Checkbox from './Checkbox.svelte'
  import Modal from './Modal.svelte'
  import Button from './Button.svelte'
  // Sends the saved request once for every combination of environments, overlays and the values of one of its
  // {{names}}, and shows a grid of passes and failures. The responses are not kept: a matrix is a check.
  import { errorText } from './lib/errors'
  import { matrixNote } from './lib/matrixlimit'
  import { RequestService } from '../bindings/github.com/githugt/flowi/app/internal/bridge'
  import MatrixResult from './MatrixResult.svelte'
  import MatrixRunning from './MatrixRunning.svelte'
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
  const note = $derived(matrixNote(dims, 'send'))

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

<Modal title="Run a matrix" width="min(760px, 94vw)" maxHeight="88vh" scroll {onclose}>
    <p class="note">Sends the saved request once for every combination you pick, one after the other, and shows a grid of passes and failures. The responses are not saved. Pick at least one thing to vary.</p>
    <div class="group"><strong>Environments</strong>
      {#each choices.envs as e}<span class="opt"><Checkbox checked={byEnv.includes(e)} onchange={() => (byEnv = toggle(byEnv, e))}>{e}</Checkbox></span>{/each}
      {#if !choices.envs.length}<span class="muted">none yet</span>{/if}
    </div>
    <div class="group"><strong>Overlays</strong> <span class="muted">laid over the request</span>
      {#each choices.overlays as o}<span class="opt"><Checkbox checked={byOverlay.includes(o)} onchange={() => (byOverlay = toggle(byOverlay, o))}>{o}</Checkbox></span>{/each}
      {#if !choices.overlays.length}<span class="muted">this collection has none</span>{/if}
    </div>
    <div class="group"><strong>A value</strong> <span class="muted">for a name the request uses, such as {'{{id}}'}</span>
      <input list="matrix-names" placeholder="name" bind:value={valueName} spellcheck="false" />
      <datalist id="matrix-names">{#each choices.names as n}<option value={n}></option>{/each}</datalist>
      {#if valueName}<input placeholder="values, separated by commas" bind:value={values} spellcheck="false" />{/if}
    </div>
    <div class="actions">
      {#if busy}
        <MatrixRunning {node} oncancel={() => RequestService.CancelMatrix(node)} />
      {:else}
        <span class={note.over ? 'bad' : 'muted'} role={note.over ? 'alert' : undefined}>{note.text}</span>
      {/if}
      <Button variant="go" loading={busy} disabled={!dims.length || note.over} onclick={run}>{busy ? 'Sending…' : 'Run'}</Button>
      <Button onclick={onclose}>Close</Button>
    </div>
    {#if matrix}<MatrixResult {matrix} />{/if}
    {#if error}<p class="bad">{error}</p>{/if}
</Modal>

<style>
  .note, .muted { color: var(--muted); font-size: var(--fs-sm); margin: 0; }
  .group { display: flex; flex-wrap: wrap; align-items: center; gap: 4px 14px; font-size: var(--fs-sm); }
  .group > strong { min-width: 90px; }
  .opt { display: flex; align-items: center; gap: 4px; font-size: var(--fs-sm); }
  input:not([type]) { padding: 3px 8px; }
  .actions { display: flex; align-items: center; justify-content: flex-end; gap: 8px; margin-top: 4px; }
  .actions .muted { margin-right: auto; }
  .bad { color: var(--fail); margin: 2px 0; }
</style>
