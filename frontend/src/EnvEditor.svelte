<script lang="ts">
  import HiddenChars from './HiddenChars.svelte'
  import { EnvironmentService } from '../bindings/github.com/githugt/flowi/internal/bridge'
  import type { EnvRow, EnvView, MissingVar } from './lib/types'
  import FileBanner from './FileBanner.svelte'

  let { view }: { view: EnvView } = $props()
  const node = $derived(view.node)

  let rows = $state<EnvRow[]>([])
  let within = $state(false)
  $effect(() => {
    if (!within) rows = view.rows.map((r) => ({ ...r }))
  })

  const push = () => EnvironmentService.SetRows(node, $state.snapshot(rows))

  // Variables the requests and flows use that this environment does not set yet, offered as rows.
  let unset = $state<MissingVar[]>([])
  $effect(() => {
    const n = node
    void view.rows
    EnvironmentService.Unset(n).then((u) => {
      if (n === node) unset = u as unknown as MissingVar[]
    })
  })
  function addRows(vars: MissingVar[]) {
    for (const v of vars) rows.push({ name: v.name, value: '', secret: v.secret, secretSet: false, override: false })
    push()
  }

  function onkeydown(e: KeyboardEvent) {
    if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 's') {
      e.preventDefault()
      EnvironmentService.Save(node)
    }
  }
</script>

<svelte:window {onkeydown} />

<div class="env">
  <header>
    <strong>{view.name}</strong>
    {#if view.dirty}<span class="dirty">unsaved</span>{/if}
    <span class="spacer"></span>
    <button class="primary" disabled={!view.dirty} onclick={() => EnvironmentService.Save(node)}>Save</button>
  </header>
  <FileBanner
    onDisk={view.onDisk}
    needsDrop={view.needsDrop}
    revert={() => EnvironmentService.Revert(node)}
    saveDropping={() => EnvironmentService.SaveDroppingComments(node)}
    declineDrop={() => EnvironmentService.DeclineDrop(node)}
  />
  {#if view.error}<p class="err">{view.error}</p>{/if}
  <div class="body" onfocusin={() => (within = true)} onfocusout={() => (within = false)}>
    {#if unset.length}
      <div class="unset">
        <span>Used by your requests and flows but not set here:</span>
        {#each unset as v}<button class="chip" title="Add a row for it" onclick={() => addRows([v])}>+ {v.name}{v.secret ? ' 🔒' : ''}</button>{/each}
        {#if unset.length > 1}<button class="all" onclick={() => addRows(unset)}>Add all</button>{/if}
      </div>
    {/if}
    <p class="muted">Secrets (🔒) are kept in the encrypted workspace vault, never in the file. Leave a secret empty to keep its stored value.</p>
    <div class="head"><span>Name</span><span>Value</span><span>Secret</span><span></span></div>
    {#each rows as r, i}
      <div class="row">
        <input placeholder="name" bind:value={r.name} oninput={push} spellcheck="false" />
        <span class="cell">
          <input
            type={r.secret ? 'password' : 'text'}
            placeholder={r.secret ? (r.secretSet ? '•••••• (stored; type to replace)' : 'value (stored in the vault)') : 'value'}
            bind:value={r.value}
            oninput={push}
            spellcheck="false"
          />
          <HiddenChars value={r.value} onclean={(t) => { r.value = t; push() }} />
        </span>
        <label class="secret" title={r.override ? 'Overridden by an environment variable' : ''}>
          <input type="checkbox" bind:checked={r.secret} onchange={push} />{r.override ? 'override' : ''}
        </label>
        <button class="x" aria-label="Remove variable" onclick={() => { rows.splice(i, 1); push() }}>×</button>
      </div>
    {/each}
    <button class="add" onclick={() => rows.push({ name: '', value: '', secret: false, secretSet: false, override: false })}>+ Add variable</button>
  </div>
</div>

<style>
  .env { position: absolute; inset: 0; display: flex; flex-direction: column; }
  header { height: 40px; flex: none; display: flex; align-items: center; gap: 8px; padding: 0 12px; border-bottom: 1px solid var(--line); }
  .dirty { color: var(--accent); font-size: var(--fs-xs); }
  .spacer { flex: 1; }
  .body { padding: 12px 16px; overflow: auto; display: flex; flex-direction: column; gap: 6px; max-width: 900px; }
  .muted { color: var(--muted); font-size: var(--fs-sm); margin: 0 0 6px; }
  .err { margin: 0; padding: 6px 12px; color: var(--fail); }
  .head, .row { display: grid; grid-template-columns: 1fr 1.6fr 70px 24px; gap: 6px; align-items: center; }
  .head { color: var(--muted); font-size: var(--fs-xs); }
  input[type='text'], input[type='password'], input:not([type]) {
    min-width: 0; background: var(--bg); color: var(--text); border: 1px solid var(--line); border-radius: var(--r-md); padding: 5px 8px;
    font: var(--fs-sm) var(--mono); -webkit-user-select: text; user-select: text;
  }
  input:focus { border-color: var(--accent); }
  .cell { position: relative; display: flex; min-width: 0; }
  .cell input { flex: 1; min-width: 0; }
  .secret { display: flex; align-items: center; gap: 4px; font-size: var(--fs-xs); color: var(--muted); }
  button { background: transparent; color: var(--text); border: 1px solid var(--line); border-radius: var(--r-md); padding: 3px 12px; font: inherit; }
  button:disabled { opacity: 0.4; }
  .primary:not(:disabled) { background: var(--accent); color: var(--on-accent); border-color: var(--accent); }
  .x { border: 0; color: var(--muted); padding: 0; }
  .unset { display: flex; flex-wrap: wrap; align-items: center; gap: var(--sp-1) var(--sp-2); padding: var(--sp-2) var(--sp-3); margin-bottom: var(--sp-2); border: 1px solid color-mix(in srgb, var(--warn) 45%, var(--line)); border-radius: var(--r-lg); background: color-mix(in srgb, var(--warn) 8%, transparent); font-size: var(--fs-sm); }
  .chip { font-family: var(--mono); font-size: var(--fs-xs); border-radius: var(--r-pill); padding: 1px 10px; }
  .chip:hover, .all:hover { border-color: var(--accent); }
  .all { border: 0; color: var(--accent); padding: 1px 4px; }
  .add { align-self: flex-start; border: 0; color: var(--accent); padding: 4px 0; }
</style>
