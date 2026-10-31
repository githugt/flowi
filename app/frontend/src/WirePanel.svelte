<script lang="ts">
  // A value wire was dropped from one step onto another: say which part of what the first makes to take, and where
  // in the second to put it. The result is a template written into that field, as one change that Undo takes back.
  import { FlowService } from '../bindings/github.com/githugt/flowi/app/internal/bridge'
  import { errorText } from './lib/errors'
  import type { WireChoices } from './lib/types'

  let { node, from, to, x, y, choices, onclose }: { node: string; from: string; to: string; x: number; y: number; choices: WireChoices; onclose: () => void } = $props()

  let take = $state('')
  let field = $state('')
  let name = $state('')
  let error = $state('')
  $effect(() => {
    if (!take) take = choices.outputs[0] ?? ''
    if (!field) field = choices.fields[0]?.id ?? ''
  })
  const chosen = $derived(choices.fields.find((f) => f.id === field))

  async function connect() {
    error = ''
    const place = chosen?.needsName ? field + name.trim() : field
    if (chosen?.needsName && !name.trim()) {
      error = 'Give it a name.'
      return
    }
    try {
      await FlowService.ConnectData(node, from, take.trim(), to, place)
      onclose()
    } catch (e) {
      error = errorText(e)
    }
  }
</script>

<svelte:window onkeydown={(e) => e.key === 'Escape' && onclose()} />

<div class="wire" style:left={`${Math.min(x, window.innerWidth - 340)}px`} style:top={`${Math.min(y + 8, window.innerHeight - 260)}px`} role="dialog" aria-label={`Pass a value from ${from} to ${to}`}>
  <div class="top"><h4>Pass a value: {from} → {to}</h4><button class="close" aria-label="Close" onclick={onclose}>×</button></div>
  {#if choices.fields.length === 0}
    <p class="hint">{to} has nothing that takes a value in. Put the value in a variable with a set step first.</p>
  {:else}
    <label class="field">
      <span class="label">Take</span>
      <input list="wire-outputs" bind:value={take} placeholder="body.token" spellcheck="false" />
      <datalist id="wire-outputs">{#each choices.outputs as o}<option value={o}></option>{/each}</datalist>
    </label>
    <label class="field">
      <span class="label">Put it in</span>
      <select bind:value={field}>{#each choices.fields as f}<option value={f.id}>{f.label}</option>{/each}</select>
    </label>
    {#if chosen?.needsName}
      <label class="field"><span class="label">Name</span><input bind:value={name} placeholder="token" spellcheck="false" /></label>
    {/if}
    {#if error}<p class="err">{error}</p>{/if}
    <div class="actions"><button onclick={onclose}>Cancel</button><button class="primary" onclick={connect}>Connect</button></div>
  {/if}
</div>

<style>
  .wire { position: fixed; z-index: 30; width: 320px; box-sizing: border-box; display: flex; flex-direction: column; gap: 8px; padding: 12px; background: var(--panel); border: 1px solid var(--data); border-radius: 14px; box-shadow: var(--shadow); }
  .top { display: flex; align-items: center; justify-content: space-between; gap: 8px; }
  h4 { margin: 0; font-size: var(--fs-md); }
  .close { background: none; border: 0; color: var(--muted); font: inherit; font-size: var(--fs-lg); line-height: 1; }
  .field { display: flex; flex-direction: column; gap: 3px; }
  .label { color: var(--muted); font-size: var(--fs-xs); }
  input, select { background: var(--bg); color: var(--text); border: 1px solid var(--line); border-radius: var(--r-md); padding: 5px 6px; font: inherit; }
  input:focus, select:focus { border-color: var(--data); }
  .hint { color: var(--muted); font-size: var(--fs-sm); margin: 0; }
  .err { color: var(--fail); font-size: var(--fs-sm); margin: 0; }
  .actions { display: flex; justify-content: flex-end; gap: 8px; }
  .actions button { background: transparent; color: var(--text); border: 1px solid var(--line); border-radius: var(--r-md); padding: 3px 12px; font: inherit; }
  .actions .primary { background: var(--act-send); color: var(--on-act); border-color: var(--act-send); font-weight: 600; }
</style>
