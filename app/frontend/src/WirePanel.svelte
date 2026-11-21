<script lang="ts">
  import Field from './Field.svelte'
  import Popover from './Popover.svelte'
  import IconButton from './IconButton.svelte'
  import Button from './Button.svelte'
  import Select from './Select.svelte'
  // A value wire was dropped from one step onto another: say which part of what the first makes to take, and where
  // in the second to put it. The result is a template written into that field, as one change that Undo takes back.
  import { FlowService } from '../bindings/github.com/githugt/flowi/app/internal/bridge'
  import { errorText } from './lib/errors'
  import { previewLine, wireRef } from './lib/wirepreview'
  import type { WireChoices, WirePreview } from './lib/types'

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

  // What the path gives in the step's last run, and what is inside it to click. Asked a moment after the last
  // keystroke, and an answer that arrives after a newer question is dropped.
  let preview = $state<WirePreview | null>(null)
  let asked = 0
  $effect(() => {
    const path = take
    if (!from) return
    const mine = ++asked
    const timer = setTimeout(() => {
      FlowService.WirePreview(node, from, path.trim()).then(
        (p) => mine === asked && (preview = p as unknown as WirePreview),
        () => mine === asked && (preview = null),
      )
    }, 200)
    return () => clearTimeout(timer)
  })
  const line = $derived(previewLine(preview))

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

<Popover {x} {y} width={320} height={260} tone="data" label={`Pass a value from ${from} to ${to}`} {onclose}>
  <div class="top"><h4>Pass a value: {from} → {to}</h4><IconButton icon="close" label="Close" onclick={onclose} /></div>
  {#if choices.fields.length === 0}
    <p class="hint">{to} has nothing that takes a value in. Put the value in a variable with a set step first.</p>
  {:else}
    <Field label="Take">
      <input list="wire-outputs" bind:value={take} placeholder="body.token" spellcheck="false" />
      <datalist id="wire-outputs">{#each choices.outputs as o}<option value={o}></option>{/each}</datalist>
    </Field>
    <Field label="Put it in">
      <Select bind:value={field}>{#each choices.fields as f}<option value={f.id}>{f.label}</option>{/each}</Select>
    </Field>
    {#if from && line.kind !== 'none'}
      <div class="preview" aria-live="polite">
        {#if line.kind === 'value'}<code class="ref">{wireRef(from, take)}</code> <span class="arrow">→</span> <code class="val">{line.text}</code>
        {:else}<span class="muted">{line.text}</span>{/if}
        {#if preview?.children.length}
          <div class="tree" role="group" aria-label="Inside this value">
            {#each preview.children as c (c.path)}
              <span class="chip"><Button small variant="ghost" title={c.sample} onclick={() => (take = c.path)}>{c.name}{c.branch ? ' ›' : ''} <span class="sample">{c.sample}</span></Button></span>
            {/each}
          </div>
        {/if}
      </div>
    {/if}
    {#if chosen?.needsName}
      <Field label="Name"><input bind:value={name} placeholder="token" spellcheck="false" /></Field>
    {/if}
    {#if error}<p class="err">{error}</p>{/if}
    <div class="actions"><Button onclick={onclose}>Cancel</Button><Button variant="primary" onclick={connect}>Connect</Button></div>
  {/if}
</Popover>

<style>
  .top { display: flex; align-items: center; justify-content: space-between; gap: 8px; }
  h4 { margin: 0; font-size: var(--fs-md); }
  input:focus { border-color: var(--data); }
  .preview { display: flex; flex-direction: column; gap: 4px; font-size: var(--fs-sm); }
  .ref { color: var(--data); overflow-wrap: anywhere; }
  .val { overflow-wrap: anywhere; }
  .arrow, .muted { color: var(--muted); }
  .tree { display: flex; flex-wrap: wrap; gap: 4px; max-height: 120px; overflow: auto; }
  .sample { color: var(--muted); max-width: 12ch; overflow: hidden; text-overflow: ellipsis; display: inline-block; vertical-align: bottom; }
  .hint { color: var(--muted); font-size: var(--fs-sm); margin: 0; }
  .err { color: var(--fail); font-size: var(--fs-sm); margin: 0; }
  .actions { display: flex; justify-content: flex-end; gap: 8px; }
</style>
