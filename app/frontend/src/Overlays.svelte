<script lang="ts">
  import Modal from './Modal.svelte'
  import Button from './Button.svelte'
  import Field from './Field.svelte'
  import Select from './Select.svelte'
  import TextInput from './TextInput.svelte'
  // The shared Menu takes a separator as a rule above the next item.
  const menuItems = $derived.by((): MenuItem[] => {
    const out: MenuItem[] = []
    let rule = false
    for (const i of ui.menu?.items ?? []) {
      if (i.separator) rule = true
      else {
        out.push({ label: i.label, onselect: i.run, disabled: i.disabled, checked: i.checked, danger: isDangerLabel(i.label), separated: rule && out.length > 0 })
        rule = false
      }
    }
    return out
  })
  import Menu, { type MenuItem } from './Menu.svelte'
  import { isDangerLabel } from './lib/menudanger'
  import Toasts from './Toasts.svelte'
  import CommandPalette from './CommandPalette.svelte'
  import BlueprintDialog from './BlueprintDialog.svelte'
  import MoveDialog from './MoveDialog.svelte'
  import PlaceDialog from './PlaceDialog.svelte'
  import SignInDialog from './SignInDialog.svelte'
  import Dropdown from './Dropdown.svelte'
  import { protocolVerb } from './lib/verbs'
  import { ui } from './lib/ui.svelte'

  let text = $state('')
  let protocol = $state('')
  let chosen = $state('')
  let input = $state<HTMLInputElement>()

  // A new prompt starts from its initial text and takes focus.
  $effect(() => {
    const d = ui.dialog
    if (d?.kind === 'prompt') {
      text = d.initial
      protocol = d.protocols?.[0]?.id ?? ''
      chosen = d.select?.options[0]?.id ?? ''
      queueMicrotask(() => input?.select())
    }
  })

  // A confirm whose action destroys something (Delete, Remove, Discard …) is drawn in the danger colour and starts
  // with the focus on Cancel, so a stray Enter does not delete. Before: it looked like every other confirm, with the
  // focus on nothing and Enter running the action.
  let cancelButton = $state<HTMLButtonElement>()
  const destructive = $derived(ui.dialog?.kind === 'confirm' && isDangerLabel(ui.dialog.action))
  $effect(() => {
    if (destructive) queueMicrotask(() => cancelButton?.focus())
  })

  function accept() {
    const d = ui.take()
    if (d?.kind === 'prompt') ui.answer(d, text, () => d.done(text, protocol, chosen))
    else if (d?.kind === 'confirm') ui.answer(d, '', () => d.done())
    else if (d?.kind === 'unsaved') d.save()
  }
</script>

{#if ui.menu}
  <Menu at={{ x: ui.menu.x, y: ui.menu.y }} minWidth={190} items={menuItems} onclose={() => ui.closeMenu()} />
{/if}

{#if ui.dialog}
  <Modal title={ui.dialog.title} width={400} onclose={() => ui.dismiss()} onsubmit={() => accept()}>
    {#if ui.dialog.kind === 'prompt'}
      <Field label={ui.dialog.label}>
        <TextInput bind:el={input} bind:value={text} type={ui.dialog.secret ? 'password' : 'text'} />
      </Field>
      {#if ui.dialog.warning}<p class="warning" role="alert">{ui.dialog.warning}</p>{/if}
      {#if ui.dialog.error}<p class="failed" role="alert">{ui.dialog.error}</p>{/if}
      {#if ui.dialog.select}
        <Field label={ui.dialog.select.label}>
          <Select bind:value={chosen}>
            {#each ui.dialog.select.options as o}<option value={o.id}>{o.label}</option>{/each}
          </Select>
        </Field>
      {/if}
      {#if ui.dialog.protocols}
        <Field label={ui.dialog.protocolsLabel ?? 'Protocol'}>
          <Dropdown pill options={ui.dialog.protocols.map((p) => ({ value: p.id, label: p.label, tone: protocolVerb(p.id) }))} value={protocol} onpick={(p) => (protocol = p)} />
        </Field>
      {/if}
    {:else}
      <p class="dtext">{ui.dialog.text}</p>
      {#if ui.dialog.kind === 'confirm' && ui.dialog.error}<p class="failed" role="alert">{ui.dialog.error}</p>{/if}
    {/if}
    {#snippet actions()}
      <Button type="button" bind:el={cancelButton} onclick={() => ui.dismiss()}>Cancel</Button>
      {#if ui.dialog?.kind === 'unsaved'}
        <Button type="button" onclick={() => { const d = ui.take(); if (d?.kind === 'unsaved') d.discard() }}>{ui.dialog.discardLabel ?? "Don't save"}</Button>
        <Button type="button" variant="primary" onclick={() => { const d = ui.take(); if (d?.kind === 'unsaved') d.save() }}>{ui.dialog.saveLabel ?? 'Save'}</Button>
      {:else if ui.dialog}
        <Button type="submit" variant={destructive ? 'danger' : 'primary'}>{ui.dialog.kind === 'confirm' ? ui.dialog.action : 'OK'}</Button>
      {/if}
    {/snippet}
  </Modal>
{/if}

<BlueprintDialog />
<MoveDialog />
<PlaceDialog />
<SignInDialog />

<CommandPalette />
<Toasts />

<style>
  /* Positioned by the code above so the menu stays inside the window; Menu draws it. */
  .failed { margin: 0; padding: 6px 8px; border: 1px solid color-mix(in srgb, var(--fail) 60%, transparent); border-radius: var(--r-md); background: color-mix(in srgb, var(--fail) 10%, transparent); font-size: var(--fs-sm); line-height: 1.45; }
  .warning { margin: 0; padding: 6px 8px; border: 1px solid color-mix(in srgb, var(--warn) 60%, transparent); border-radius: var(--r-md); background: color-mix(in srgb, var(--warn) 12%, transparent); font-size: var(--fs-sm); line-height: 1.45; }
  .dtext { margin: 0; line-height: 1.45; white-space: pre-line; }
</style>
