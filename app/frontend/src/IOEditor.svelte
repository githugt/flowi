<script lang="ts">
  // The editor behind a Start or End row: the input's or output's name, type and default or value, and Remove. It
  // opens where the row was clicked and each field commits when it is left, so Undo takes back one field at a time.
  import { FlowService } from '../bindings/github.com/githugt/flowi/internal/bridge'
  import { errorText } from './lib/errors'
  import { IO_TYPES } from './lib/edgecards'
  import type { IOItem } from './lib/types'

  let {
    node,
    role,
    index,
    item,
    x,
    y,
    onclose,
  }: { node: string; role: 'start' | 'end'; index: number; item: IOItem; x: number; y: number; onclose: () => void } = $props()

  const isInput = $derived(role === 'start')
  let name = $state('')
  let type = $state('')
  let value = $state('')
  let error = $state('')
  $effect(() => {
    name = item.name
    type = item.type
    value = item.value
  })

  async function commit() {
    error = ''
    try {
      if (isInput) await FlowService.SetInput(node, index, name.trim(), type, value)
      else await FlowService.SetOutput(node, index, name.trim(), type, value)
    } catch (e) {
      error = errorText(e)
    }
  }

  async function remove() {
    try {
      if (isInput) await FlowService.RemoveInput(node, index)
      else await FlowService.RemoveOutput(node, index)
      onclose()
    } catch (e) {
      error = errorText(e)
    }
  }
</script>

<svelte:window onkeydown={(e) => e.key === 'Escape' && onclose()} />
<div class="scrim" role="presentation" onpointerdown={onclose}></div>
<div class="io" style:left={`${Math.min(x, window.innerWidth - 340)}px`} style:top={`${Math.min(y + 8, window.innerHeight - 300)}px`} role="dialog" aria-label={isInput ? 'Input' : 'Output'}>
  <div class="top"><h4>{isInput ? 'Input' : 'Output'}</h4><button class="close" aria-label="Close" onclick={onclose}>×</button></div>
  <label class="field"><span class="label">Name</span><input bind:value={name} onchange={commit} spellcheck="false" /></label>
  <label class="field">
    <span class="label">Type</span>
    <select bind:value={type} onchange={commit}>{#each IO_TYPES as t}<option value={t}>{t || 'any'}</option>{/each}</select>
  </label>
  <label class="field">
    <span class="label">{isInput ? 'Default' : 'Value'}</span>
    <input bind:value onchange={commit} placeholder={isInput ? '' : '{{vars.token}}'} spellcheck="false" />
  </label>
  {#if error}<p class="err">{error}</p>{/if}
  <div class="actions"><button class="remove" onclick={remove}>Remove</button><button class="primary" onclick={onclose}>Done</button></div>
</div>

<style>
  .scrim { position: fixed; inset: 0; z-index: 29; }
  .io { position: fixed; z-index: 30; width: 320px; box-sizing: border-box; display: flex; flex-direction: column; gap: 8px; padding: 12px; background: var(--panel); border: 1px solid var(--line); border-radius: 14px; box-shadow: var(--shadow); }
  .top { display: flex; align-items: center; justify-content: space-between; }
  h4 { margin: 0; font-size: var(--fs-md); }
  .close { background: none; border: 0; color: var(--muted); font: inherit; font-size: var(--fs-lg); line-height: 1; }
  .field { display: flex; flex-direction: column; gap: 3px; }
  .label { color: var(--muted); font-size: var(--fs-xs); }
  input, select { background: var(--bg); color: var(--text); border: 1px solid var(--line); border-radius: var(--r-md); padding: 5px 6px; font: inherit; }
  input:focus, select:focus { border-color: var(--accent); outline: none; }
  .err { color: var(--fail); font-size: var(--fs-sm); margin: 0; }
  .actions { display: flex; justify-content: space-between; gap: 8px; }
  .actions button { background: transparent; color: var(--text); border: 1px solid var(--line); border-radius: var(--r-md); padding: 3px 12px; font: inherit; }
  .actions .remove:hover { color: var(--fail); border-color: var(--fail); }
  .actions .primary { background: var(--act-send); color: var(--on-act); border-color: var(--act-send); font-weight: 600; }
</style>
