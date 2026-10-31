<script lang="ts">
  import HiddenChars from './HiddenChars.svelte'
  import { EnvironmentService } from '../bindings/github.com/githugt/flowi/app/internal/bridge'
  import type { EnvRow, EnvView, MissingVar } from './lib/types'
  import FileBanner from './FileBanner.svelte'
  import Hint from './Hint.svelte'
  import { envSwatch } from './lib/envcolours'

  let { view }: { view: EnvView } = $props()
  const node = $derived(view.node)

  let rows = $state<EnvRow[]>([])
  let within = $state(false)
  $effect(() => {
    if (!within) rows = view.rows.map((r) => ({ ...r }))
  })

  // Focus moving between the form's own controls is not leaving it: on Linux a clicked button takes the focus,
  // and the form would reset from the saved rows and drop a blank row before a click on it landed.
  function leave(e: FocusEvent & { currentTarget: HTMLElement }) {
    if (!e.currentTarget.contains(e.relatedTarget as Node | null)) within = false
  }

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
    for (const v of vars) rows.push({ name: v.name, value: '', secret: v.secret, secretSet: false, override: false, kind: '' })
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
  <header style:border-left={envSwatch(view.colour) ? `4px solid ${envSwatch(view.colour)}` : undefined}>
    <strong>{view.name}</strong>
    {#if view.dirty}<span class="dirty">unsaved</span>{/if}
    <span class="spacer"></span>
    <label class="base" title="Variables of the base apply unless this environment sets the same name. Secrets are not inherited.">
      <span>Builds on</span>
      <select value={view.extends} onchange={(e) => EnvironmentService.SetExtends(node, e.currentTarget.value)}>
        <option value="">nothing</option>
        {#each view.bases as b}<option value={b}>{b}</option>{/each}
      </select>
    </label>
    <div class="swatches" role="radiogroup" aria-label="Colour">
      <button class="sw none" class:on={!view.colour} role="radio" aria-checked={!view.colour} title="Default colour" aria-label="Default colour" onclick={() => EnvironmentService.SetColour(node, '')}></button>
      {#each view.colours as c}
        <button class="sw" class:on={view.colour === c} role="radio" aria-checked={view.colour === c} title={c} aria-label={c} style:background={envSwatch(c)} onclick={() => EnvironmentService.SetColour(node, c)}></button>
      {/each}
    </div>
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
  <div class="body" onfocusin={() => (within = true)} onfocusout={leave}>
    {#if unset.length}
      <div class="unset">
        <span>Used by your requests and flows but not set here:</span>
        {#each unset as v}<button class="chip" title="Add a row for it" onclick={() => addRows([v])}>+ {v.name}{v.secret ? ' 🔒' : ''}</button>{/each}
        {#if unset.length > 1}<button class="all" onclick={() => addRows(unset)}>Add all</button>{/if}
      </div>
    {/if}
    <p class="muted">Secrets (🔒) are kept in the encrypted workspace vault, never in the file, and are not shown again once saved. To change one, type a new value; leave the box blank to keep the saved one.<Hint>To see a stored secret again, open <b>Workspace → Secrets…</b> and choose <b>Show</b> on that secret, or run <code>flowi secret get &lt;env&gt; &lt;name&gt; --reveal</code> in a terminal. It shows one secret for a few seconds.</Hint></p>
    <div class="head"><span>Name</span><span>Value</span><span>Type</span><span>Secret</span><span></span></div>
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
        <select class="kind" aria-label="Type of {r.name || 'variable'}" bind:value={r.kind} onchange={push} disabled={r.secret} title="A number, true/false or an object keeps its type when used on its own, so a JSON body gets 5 and not &quot;5&quot;.">
          <option value="">text</option>
          <option value="number">number</option>
          <option value="boolean">true/false</option>
          <option value="json">object</option>
        </select>
        <label class="secret" title={r.override ? 'Overridden by an environment variable' : ''}>
          <input type="checkbox" bind:checked={r.secret} onchange={push} />{r.override ? 'override' : ''}
        </label>
        <button class="x" aria-label="Remove variable" onclick={() => { rows.splice(i, 1); push() }}>×</button>
      </div>
    {/each}
    <button class="add" onclick={() => rows.push({ name: '', value: '', secret: false, secretSet: false, override: false, kind: '' })}>+ Add variable</button>
  </div>
</div>

<style>
  .env { position: absolute; inset: 0; display: flex; flex-direction: column; }
  .base { display: flex; align-items: center; gap: 6px; margin-right: 12px; font-size: 12px; color: var(--muted); }
  .base select { height: 28px; }
  .swatches { display: flex; align-items: center; gap: 6px; margin-right: 8px; }
  .sw { width: 16px; height: 16px; padding: 0; border-radius: 50%; border: 2px solid transparent; box-sizing: border-box; background: var(--bg); }
  .sw.none { border: 1px dashed var(--muted); }
  .sw.on { outline: 2px solid var(--text); outline-offset: 1px; }
  header { height: 40px; flex: none; display: flex; align-items: center; gap: 8px; padding: 0 12px; border-bottom: 1px solid var(--line); }
  .dirty { color: var(--accent); font-size: var(--fs-xs); }
  .spacer { flex: 1; }
  .body { padding: 12px 16px; overflow: auto; display: flex; flex-direction: column; gap: 6px; max-width: 900px; }
  .muted { color: var(--muted); font-size: var(--fs-sm); margin: 0 0 6px; }
  .err { margin: 0; padding: 6px 12px; color: var(--fail); }
  .head, .row { display: grid; grid-template-columns: 1fr 1.6fr 96px 70px 24px; gap: 6px; align-items: center; }
  .head { color: var(--muted); font-size: var(--fs-xs); }
  input[type='text'], input[type='password'], input:not([type]) {
    min-width: 0; background: var(--bg); color: var(--text); border: 1px solid var(--line); border-radius: var(--r-md); padding: 5px 8px;
    font: var(--fs-sm) var(--mono); -webkit-user-select: text; user-select: text;
  }
  input:focus { border-color: var(--accent); }
  .cell { position: relative; display: flex; min-width: 0; }
  .cell input { flex: 1; min-width: 0; }
  .kind { height: 28px; min-width: 0; }
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
