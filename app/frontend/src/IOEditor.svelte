<script lang="ts">
  import Field from './Field.svelte'
  import Popover from './Popover.svelte'
  import IconButton from './IconButton.svelte'
  import Button from './Button.svelte'
  import Select from './Select.svelte'
  // The editor behind a Start or End row: the input's or output's name, type and default or value, and Remove. It
  // opens where the row was clicked and each field commits when it is left, so Undo takes back one field at a time.
  import { FlowService } from '../bindings/github.com/githugt/flowi/app/internal/bridge'
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
<Popover {x} {y} label={isInput ? 'Input' : 'Output'} width={320} height={300} {onclose}>
  <div class="top"><h4>{isInput ? 'Input' : 'Output'}</h4><IconButton icon="close" label="Close" onclick={onclose} /></div>
  <Field label="Name"><input bind:value={name} onchange={commit} spellcheck="false" /></Field>
  <Field label="Type">
    <Select bind:value={type} onchange={commit}>{#each IO_TYPES as t}<option value={t}>{t || 'any'}</option>{/each}</Select>
  </Field>
  <Field label={`${isInput ? 'Default' : 'Value'}`}>
    <input bind:value onchange={commit} placeholder={isInput ? '' : '{{vars.token}}'} spellcheck="false" />
  </Field>
  {#if error}<p class="err">{error}</p>{/if}
  <div class="actions"><Button variant="danger" onclick={remove}>Remove</Button><Button variant="primary" onclick={onclose}>Done</Button></div>
</Popover>

<style>
  .top { display: flex; align-items: center; justify-content: space-between; }
  h4 { margin: 0; font-size: var(--fs-md); }
  .err { color: var(--fail); font-size: var(--fs-sm); margin: 0; }
  input:focus { border-color: var(--accent); outline: none; }
  .actions { display: flex; justify-content: space-between; gap: 8px; }
</style>
