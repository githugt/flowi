<script lang="ts">
  // The flowi overlays of this request's collection: list them, open one to read or edit its file, add
  // one (type it, or paste one brought from elsewhere), delete one. An overlay is a named set of changes
  // laid over a request version when it is sent; this is not the spec overlay of an imported API.
  import { RequestService } from '../bindings/github.com/githugt/flowi/internal/bridge'
  import { errorText } from './lib/errors'
  import { ui } from './lib/ui.svelte'
  import type { FlowiOverlayInfo, SpecExport } from './lib/types'

  let { node, overlays, current, onclose }: { node: string; overlays: FlowiOverlayInfo[]; current: string; onclose: () => void } = $props()

  let name = $state('')
  let text = $state('')
  let isNew = $state(false)
  let problem = $state('')
  let loaded = $state('')

  async function open(n: string) {
    exported = null
    problem = ''
    try {
      text = await RequestService.FlowiOverlayText(node, n)
      name = n
      loaded = n
      isNew = false
    } catch (e) {
      problem = errorText(e)
    }
  }
  async function create() {
    problem = ''
    text = await RequestService.FlowiOverlayText(node, '')
    name = ''
    loaded = ''
    isNew = true
  }
  async function save() {
    problem = (await RequestService.SaveFlowiOverlayText(node, name, text)) as string
    if (!problem) {
      loaded = name
      isNew = false
      ui.toast(`Saved the flowi overlay ${name}.`)
    }
  }
  function remove() {
    ui.confirm('Delete flowi overlay', `Delete ${name}? Responses kept for it stay in the data folder.`, async () => {
      try {
        await RequestService.DeleteFlowiOverlay(node, name)
        name = ''
        text = ''
        loaded = ''
      } catch (e) {
        problem = errorText(e)
      }
    })
  }
  // The overlay as OpenAPI Overlays for the specs this collection keeps, with what they leave out.
  let exported = $state<SpecExport | null>(null)
  async function exportSpec() {
    problem = ''
    try {
      exported = (await RequestService.ExportFlowiOverlay(node, loaded)) as unknown as SpecExport
    } catch (e) {
      problem = errorText(e)
    }
  }
  async function saveExport(d: SpecExport['documents'][number]) {
    try {
      const file = await RequestService.SaveSpecExport(d.file, d.yaml)
      if (file) ui.toast(`Saved ${file}.`)
    } catch (e) {
      problem = errorText(e)
    }
  }
  const editing = $derived(isNew || loaded !== '')
  const nameProblem = $derived(isNew && name && !/^[A-Za-z0-9_-][A-Za-z0-9._-]*$/.test(name) ? 'Use letters, digits, “.”, “_” and “-”.' : '')

  $effect(() => {
    if (current && !loaded && !isNew) open(current)
  })
</script>

<div class="scrim" role="presentation" onpointerdown={onclose}>
  <div class="dialog" role="dialog" aria-label="Flowi overlays" tabindex="-1" onpointerdown={(e) => e.stopPropagation()}>
    <h3>Flowi overlays</h3>
    <p class="muted">A flowi overlay lays changes over a request version when it is sent: the same request as a VIP customer, a slow one, one with a bad token. Choose one beside the version picker; <code>flowi send --flowi-overlay</code> does the same.</p>
    <div class="cols">
      <ul class="list" aria-label="Overlays">
        {#each overlays as o}
          <li>
            <button class:on={loaded === o.name && !isNew} onclick={() => open(o.name)} title={o.description}>
              <strong>{o.name}</strong>
              <span class="muted">{o.shared} shared{o.used ? `, ${o.own} for this request` : ''}</span>
            </button>
          </li>
        {/each}
        {#if !overlays.length}<li class="muted none">No overlays yet. Edit this request, then Save ▾ → Save as flowi overlay, or add one here.</li>{/if}
        <li><button class="add" class:on={isNew} onclick={create}>+ New overlay</button></li>
      </ul>
      <div class="edit">
        {#if editing}
          {#if isNew}
            <label class="field"><span>Name</span><input bind:value={name} placeholder="vip" aria-label="Overlay name" /></label>
            {#if nameProblem}<p class="err">{nameProblem}</p>{/if}
          {:else}
            <p class="muted"><code>overlays/{loaded}.flowi-overlay.yaml</code></p>
          {/if}
          <textarea spellcheck="false" bind:value={text} aria-label="Overlay file"></textarea>
          <details class="help">
            <summary>What can an overlay change?</summary>
            <p>Each item is <code>set</code>, <code>remove</code>, <code>add</code> or <code>merge</code> with a path: <code>url</code>, <code>method</code>, <code>name</code>, <code>docs</code>, <code>headers.X-Tier</code>, <code>query.page</code>, <code>variables.who</code>, <code>settings.timeouts.total</code>, <code>body.json</code> (<code>merge</code> patches a JSON object; <code>null</code> removes a key), <code>body.text</code>, <code>assert</code> (<code>add</code> or <code>remove</code> a check), <code>extract.name</code> and <code>hooks.pre</code>, <code>hooks.post</code> or <code>hooks.message</code> (<code>add</code> or <code>remove</code> a reusable flow, named by its id). Overlays stack: <code>--flowi-overlay vip+slow</code>, or “+ stack” beside the picker, applies them in order and a later one wins. <code>shared</code> applies to every request that uses the overlay; <code>requests.&lt;id&gt;.ops</code> to one request. <code>was</code> records what the saved version held, to report drift.</p>
          </details>
          {#if problem}<p class="err">{problem}</p>{/if}
          <div class="actions">
            {#if !isNew}<button class="danger" onclick={remove}>Delete</button>{/if}
            {#if !isNew}<button title="Write what a spec can hold as an OpenAPI Overlay" onclick={exportSpec}>Export as OpenAPI Overlay…</button>{/if}
            <span class="grow"></span>
            <button class="primary" disabled={!name || !!nameProblem} onclick={save}>Save</button>
          </div>
          {#if exported}
            <div class="export" role="region" aria-label="OpenAPI Overlay export">
              {#if exported.documents.length === 0}
                <p class="muted">Nothing in this overlay can go into a spec, so no OpenAPI Overlay was made.</p>
              {/if}
              {#each exported.documents as d}
                <p class="muted"><strong>{d.file}</strong> for {d.spec} · {d.requests.length} {d.requests.length === 1 ? 'request' : 'requests'}
                  <button onclick={() => saveExport(d)}>Save…</button></p>
                <textarea readonly spellcheck="false" aria-label={`OpenAPI Overlay for ${d.spec}`}>{d.yaml}</textarea>
              {/each}
              {#if exported.notCarried.length}
                <details><summary>{exported.notCarried.length} not carried: the spec has no place for it</summary>
                  <ul>{#each exported.notCarried as n}<li>{n}</li>{/each}</ul>
                </details>
              {/if}
            </div>
          {/if}
        {:else}
          <p class="muted">Pick an overlay to read or edit it.</p>
        {/if}
      </div>
    </div>
    <div class="actions"><span class="grow"></span><button onclick={onclose}>Close</button></div>
  </div>
</div>

<style>
  .scrim { position: fixed; inset: 0; background: rgb(0 0 0 / 0.4); display: grid; place-items: center; z-index: 60; }
  .dialog { width: 760px; max-width: calc(100vw - 32px); max-height: 86vh; display: flex; flex-direction: column; gap: 10px; padding: 16px; background: var(--panel); border: 1px solid var(--line); border-radius: var(--r-lg); }
  h3, p { margin: 0; }
  .muted { color: var(--muted); font-size: var(--fs-sm); }
  .cols { display: flex; gap: 12px; min-height: 280px; flex: 1; min-height: 0; }
  .list { list-style: none; margin: 0; padding: 0; width: 210px; flex: none; display: flex; flex-direction: column; gap: 2px; overflow: auto; }
  .list button { width: 100%; display: flex; flex-direction: column; align-items: flex-start; gap: 1px; text-align: left; border: 1px solid transparent; background: transparent; color: var(--text); border-radius: var(--r-md); padding: 5px 8px; font: inherit; }
  .list button:hover { background: var(--hover); }
  .list button.on { background: color-mix(in srgb, var(--accent) 14%, transparent); border-color: var(--accent); }
  .list .add { color: var(--accent); }
  .none { padding: 6px 8px; }
  .edit { flex: 1; min-width: 0; display: flex; flex-direction: column; gap: 8px; }
  textarea { flex: 1; min-height: 180px; resize: none; background: var(--bg); color: var(--text); border: 1px solid var(--line); border-radius: var(--r-md); padding: 8px; font: 12px/1.5 ui-monospace, SFMono-Regular, Menlo, monospace; }
  .field { display: flex; align-items: center; gap: 8px; }
  .field span { color: var(--muted); font-size: var(--fs-sm); width: 44px; }
  .field input { flex: 1; background: var(--bg); color: var(--text); border: 1px solid var(--line); border-radius: var(--r-md); padding: 4px 8px; font: inherit; }
  .export { border-top: 1px solid var(--line); padding-top: 8px; display: flex; flex-direction: column; gap: 6px; }
  .export textarea { min-height: 90px; flex: none; }
  .export ul { margin: 4px 0 0; padding-left: 18px; font-size: var(--fs-xs); color: var(--muted); }
  .help { font-size: var(--fs-xs); color: var(--muted); }
  .help p { margin-top: 4px; line-height: 1.5; }
  code { font-size: var(--fs-xs); }
  .err { color: var(--fail); font-size: var(--fs-sm); }
  .actions { display: flex; gap: 8px; align-items: center; }
  .grow { flex: 1; }
  button { background: transparent; color: var(--text); border: 1px solid var(--line); border-radius: var(--r-md); padding: 3px 12px; font: inherit; }
  button:disabled { opacity: 0.4; }
  .primary { background: var(--accent); color: var(--on-accent); border-color: var(--accent); }
  .danger { color: var(--fail); }
</style>
