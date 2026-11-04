<script lang="ts">
  let menuH = $state(0)
  // The shared Menu takes a separator as a rule above the next item.
  const menuItems = $derived.by((): MenuItem[] => {
    const out: MenuItem[] = []
    let rule = false
    for (const i of ui.menu?.items ?? []) {
      if (i.separator) rule = true
      else {
        out.push({ label: i.label, onselect: i.run, disabled: i.disabled, checked: i.checked, separated: rule && out.length > 0 })
        rule = false
      }
    }
    return out
  })
  let menuW = $state(0)
  let innerWidth = $state(0)
  let innerHeight = $state(0)
  import Menu, { type MenuItem } from './Menu.svelte'
  import Toasts from './Toasts.svelte'
  import CommandPalette from './CommandPalette.svelte'
  import BlueprintDialog from './BlueprintDialog.svelte'
  import SuggestDialog from './SuggestDialog.svelte'
  import NegativesDialog from './NegativesDialog.svelte'
  import EnvCompareDialog from './EnvCompareDialog.svelte'
  import DiscoverDialog from './DiscoverDialog.svelte'
  import ForkDialog from './ForkDialog.svelte'
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

  function accept() {
    const d = ui.take()
    if (d?.kind === 'prompt') d.done(text, protocol, chosen)
    else if (d?.kind === 'confirm') d.done()
    else if (d?.kind === 'unsaved') d.save()
  }
</script>

<svelte:window bind:innerWidth bind:innerHeight onpointerdown={() => ui.closeMenu()} onkeydown={(e) => e.key === 'Escape' && (ui.dismiss(), ui.closeMenu())} />

{#if ui.menu}
  <!-- kept inside the window: moved up when it would run off the bottom, left when off the right -->
  <div class="menupos" bind:clientHeight={menuH} bind:clientWidth={menuW}
    style:left={`${Math.max(8, Math.min(ui.menu.x, innerWidth - menuW - 8))}px`} style:top={`${Math.max(8, Math.min(ui.menu.y, innerHeight - menuH - 8))}px`}>
    <Menu style="position: static; min-width: 190px" items={menuItems} onclose={() => ui.closeMenu()} />
  </div>
{/if}

{#if ui.dialog}
  <div class="scrim" role="presentation" onpointerdown={() => ui.dismiss()}>
    <form class="dialog" onpointerdown={(e) => e.stopPropagation()} onsubmit={(e) => { e.preventDefault(); accept() }}>
      <h3>{ui.dialog.title}</h3>
      {#if ui.dialog.kind === 'prompt'}
        <label>
          <span>{ui.dialog.label}</span>
          <input bind:this={input} bind:value={text} spellcheck="false" type={ui.dialog.secret ? 'password' : 'text'} autocomplete="off" />
        </label>
        {#if ui.dialog.warning}<p class="warning" role="alert">{ui.dialog.warning}</p>{/if}
        {#if ui.dialog.select}
          <label>
            <span>{ui.dialog.select.label}</span>
            <select bind:value={chosen}>
              {#each ui.dialog.select.options as o}<option value={o.id}>{o.label}</option>{/each}
            </select>
          </label>
        {/if}
        {#if ui.dialog.protocols}
          <label>
            <span>{ui.dialog.protocolsLabel ?? 'Protocol'}</span>
            <Dropdown pill options={ui.dialog.protocols.map((p) => ({ value: p.id, label: p.label, tone: protocolVerb(p.id) }))} value={protocol} onpick={(p) => (protocol = p)} />
          </label>
        {/if}
      {:else}
        <p>{ui.dialog.text}</p>
      {/if}
      <div class="actions">
        <button type="button" onclick={() => ui.dismiss()}>Cancel</button>
        {#if ui.dialog.kind === 'unsaved'}
          <button type="button" onclick={() => { const d = ui.take(); if (d?.kind === 'unsaved') d.discard() }}>{ui.dialog.discardLabel ?? "Don't save"}</button>
          <button class="primary" type="button" onclick={() => { const d = ui.take(); if (d?.kind === 'unsaved') d.save() }}>{ui.dialog.saveLabel ?? 'Save'}</button>
        {:else}
          <button class="primary" type="submit">{ui.dialog.kind === 'confirm' ? ui.dialog.action : 'OK'}</button>
        {/if}
      </div>
    </form>
  </div>
{/if}

<BlueprintDialog />
<SuggestDialog />
<NegativesDialog />
<EnvCompareDialog />
<DiscoverDialog />
<ForkDialog />
<MoveDialog />
<PlaceDialog />
<SignInDialog />

<CommandPalette />
<Toasts />

<style>
  /* Positioned by the code above so the menu stays inside the window; Menu draws it. */
  .menupos { position: fixed; z-index: 50; }
  .scrim { position: fixed; inset: 0; background: rgb(0 0 0 / 0.5); backdrop-filter: blur(2px); display: grid; place-items: center; z-index: 60; animation: fade var(--t-base) var(--ease-out); }
  .dialog { width: 400px; display: flex; flex-direction: column; gap: 12px; padding: 20px; background: var(--raised); border: 1px solid var(--line); border-radius: var(--r-xl); box-shadow: var(--shadow); animation: rise var(--t-slow) var(--ease-out); }
  .dialog h3 { margin: 0; font-size: var(--fs-lg); font-weight: 600; }
  @keyframes fade { from { opacity: 0; } }
  @keyframes rise { from { opacity: 0; transform: translateY(6px) scale(.985); } }
  @media (prefers-reduced-motion: reduce) { .scrim, .dialog { animation: none; } }
  .dialog p { margin: 0; line-height: 1.45; }
  .dialog .warning { padding: 6px 8px; border: 1px solid color-mix(in srgb, var(--warn) 60%, transparent); border-radius: var(--r-md); background: color-mix(in srgb, var(--warn) 12%, transparent); font-size: var(--fs-sm); }
  .dialog label { display: flex; flex-direction: column; gap: 3px; color: var(--muted); font-size: var(--fs-xs); }
  .dialog input, .dialog select {
    background: var(--bg); color: var(--text); border: 1px solid var(--line); border-radius: var(--r-md); padding: 5px 8px; min-height: var(--ctl-h); box-sizing: border-box; font: inherit;
    -webkit-user-select: text; user-select: text;
  }
  .actions { display: flex; justify-content: flex-end; gap: 8px; }
  .actions button { background: var(--raised-3); color: var(--text); border: 0; border-radius: var(--r-md); padding: 0 14px; min-height: var(--ctl-h); font: inherit; font-weight: 500; }
  .primary { background: var(--accent) !important; color: var(--on-accent) !important; border-color: var(--accent) !important; }
</style>
