<script lang="ts">
  let menuH = $state(0)
  let menuW = $state(0)
  let innerWidth = $state(0)
  let innerHeight = $state(0)
  import BlueprintDialog from './BlueprintDialog.svelte'
  import MoveDialog from './MoveDialog.svelte'
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
        <button role="menuitem" disabled={item.disabled} onclick={() => { ui.closeMenu(); item.run() }}>{item.label}</button>
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
            <span>Protocol</span>
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

<div class="toasts" aria-live="polite">
  {#each ui.toasts as t (t.id)}<div class="toast {t.level}">{t.text}</div>{/each}
</div>

<style>
  .menu {
    position: fixed;
    z-index: 50;
    min-width: 190px;
    max-height: calc(100vh - 24px);
    overflow: auto;
    display: flex;
    flex-direction: column;
    padding: 4px;
    background: var(--panel);
    border: 1px solid var(--line);
    border-radius: var(--r-lg);
    box-shadow: 0 8px 24px rgb(0 0 0 / 0.35);
  }
  .menu button { background: none; border: 0; color: var(--text); font: inherit; text-align: left; padding: 5px 10px; border-radius: var(--r-sm); }
  .menu button:hover:not(:disabled) { background: color-mix(in srgb, var(--accent) 20%, transparent); }
  .menu button:disabled { opacity: 0.4; }
  hr { width: 100%; border: 0; border-top: 1px solid var(--line); margin: 4px 0; }
  .scrim { position: fixed; inset: 0; background: rgb(0 0 0 / 0.4); display: grid; place-items: center; z-index: 60; }
  .dialog { width: 380px; display: flex; flex-direction: column; gap: 10px; padding: 16px; background: var(--panel); border: 1px solid var(--line); border-radius: var(--r-lg); }
  .dialog h3 { margin: 0; }
  .dialog p { margin: 0; line-height: 1.45; }
  .dialog label { display: flex; flex-direction: column; gap: 3px; color: var(--muted); font-size: var(--fs-xs); }
  .dialog input, .dialog select {
    background: var(--bg); color: var(--text); border: 1px solid var(--line); border-radius: var(--r-md); padding: 5px 8px; font: inherit;
    -webkit-user-select: text; user-select: text;
  }
  .actions { display: flex; justify-content: flex-end; gap: 8px; }
  .actions button { background: transparent; color: var(--text); border: 1px solid var(--line); border-radius: var(--r-md); padding: 4px 14px; font: inherit; }
  .primary { background: var(--accent) !important; color: var(--on-accent) !important; border-color: var(--accent) !important; }
  .toasts { position: fixed; right: 16px; bottom: 16px; display: flex; flex-direction: column; gap: 8px; z-index: 70; }
  .toast { max-width: 380px; padding: 8px 12px; border-radius: var(--r-lg); background: var(--panel); border: 1px solid var(--line); box-shadow: 0 8px 24px rgb(0 0 0 / 0.35); }
  .toast.error { border-color: var(--fail); color: var(--fail); }
</style>
