<script lang="ts">
  let menuH = $state(0)
  // A menu with a ticked choice keeps room for the tick on every row, so the labels line up.
  const hasChecks = $derived(!!ui.menu?.items.some((i) => i.checked !== undefined))
  let menuW = $state(0)
  let innerWidth = $state(0)
  let innerHeight = $state(0)
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
  <div class="menu" role="menu" tabindex="-1" bind:clientHeight={menuH} bind:clientWidth={menuW}
    style:left={`${Math.max(8, Math.min(ui.menu.x, innerWidth - menuW - 8))}px`} style:top={`${Math.max(8, Math.min(ui.menu.y, innerHeight - menuH - 8))}px`} onpointerdown={(e) => e.stopPropagation()}>
    {#each ui.menu.items as item}
      {#if item.separator}<hr />{:else}
        <button role={item.checked === undefined ? 'menuitem' : 'menuitemradio'} class:checkable={hasChecks} aria-checked={item.checked} disabled={item.disabled} onclick={() => { ui.closeMenu(); item.run() }}>{#if hasChecks}<span class="tick" aria-hidden="true">{item.checked ? '✓' : ''}</span>{/if}{item.label}</button>
      {/if}
    {/each}
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
<MoveDialog />
<PlaceDialog />
<SignInDialog />

<div class="toasts" aria-live="polite">
  {#each ui.toasts as t (t.id)}<div class="toast {t.level}">{t.text}</div>{/each}
</div>

<style>
  .menu {
    position: fixed;
    z-index: 50;
    min-width: 190px;
    width: max-content;
    max-width: calc(100vw - 16px);
    max-height: calc(100vh - 24px);
    overflow: auto;
    display: flex;
    flex-direction: column;
    padding: 4px;
    background: var(--raised);
    border: 1px solid var(--line);
    border-radius: var(--r-lg);
    box-shadow: var(--shadow);
  }
  .menu button { background: none; border: 0; color: var(--text); font: inherit; text-align: left; padding: 5px 10px; border-radius: var(--r-sm); white-space: nowrap; }
  .menu button:hover:not(:disabled) { background: var(--tint-hover); }
  .menu button:disabled { opacity: 0.4; }
  .menu button.checkable { display: flex; gap: 6px; }
  .tick { flex: none; width: 1em; color: var(--accent); }
  hr { width: 100%; border: 0; border-top: 1px solid var(--line); margin: 4px 0; }
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
  .toasts { position: fixed; right: 16px; bottom: 16px; display: flex; flex-direction: column; gap: 8px; z-index: 70; }
  .toast { max-width: 380px; padding: 9px 14px 9px 12px; border-radius: var(--r-lg); background: var(--raised); border: 1px solid var(--line); border-left: 3px solid var(--accent); box-shadow: var(--shadow); animation: rise var(--t-slow) var(--ease-out); }
  .toast.error { border-left-color: var(--fail); }
  @media (prefers-reduced-motion: reduce) { .toast { animation: none; } }
</style>
