<script lang="ts">
  // What to do with text pasted into a request's address that is really something to import (a curl
  // command, an OpenAPI document …): fill the request from it, import it as new requests, or paste it
  // as typed. Importing shows the import screen's preview here, in short.
  import { onDestroy } from 'svelte'
  import { ImportService, RequestService } from '../bindings/github.com/githugt/flowi/internal/bridge'
  import { app } from './lib/app.svelte'
  import { errorText } from './lib/errors'
  import { live } from './lib/live'
  import type { ImportView } from './lib/types'
  import { ui } from './lib/ui.svelte'

  const offer = $derived(app.pasteOffer)
  let preview = $state<ImportView | null>(null)
  let error = $state('')
  let existing = $state('skip')
  let stop: (() => void) | null = null

  const SHOWN_LINES = 8
  const excerpt = $derived.by(() => {
    const lines = (offer?.text ?? '').trim().split('\n')
    return lines.slice(0, SHOWN_LINES).join('\n') + (lines.length > SHOWN_LINES ? `\n… ${lines.length - SHOWN_LINES} more lines` : '')
  })
  const canFill = $derived(!!offer?.kind.fills && offer.http)

  function close() {
    stop?.()
    stop = null
    preview = null
    error = ''
    app.pasteOffer = null
  }
  onDestroy(() => stop?.())

  async function fill() {
    if (!offer) return
    // A focused field keeps the text being typed over Go's: let go of it, so the filled address shows.
    offer.field?.blur()
    try {
      const n = await RequestService.FillFromPaste(offer.node, offer.kind.id, offer.text)
      if (n > 1) ui.toast(`Filled from the first of ${n} requests. Import the text to keep them all.`)
      close()
    } catch (e) {
      error = errorText(e)
    }
  }

  async function importNew() {
    if (!offer) return
    error = ''
    stop = live<ImportView>('import', () => ImportService.State(), (v) => {
      preview = v
      if (v.done) {
        ui.toast(v.done)
        close()
      }
    })
    try {
      await ImportService.PreviewPasted(offer.kind.id, offer.text, offer.node)
    } catch (e) {
      error = errorText(e)
    }
  }

  // The text goes into the field it was pasted into, as if nothing had been offered.
  function pasteAsText() {
    const o = offer
    close()
    if (!o?.field) return
    o.field.focus()
    document.execCommand('insertText', false, o.text)
  }

  function moreOptions() {
    close()
    app.openScreen('screen:import', 'Import')
  }

  function setCollection(id: string) {
    ImportService.SetCollection(id)
    ImportService.Preview()
  }
</script>

<svelte:window onkeydown={(e) => offer && e.key === 'Escape' && close()} />

{#if offer}
  <div class="scrim" role="presentation" onpointerdown={close}>
    <div class="dialog" role="dialog" aria-modal="true" aria-labelledby="paste-title" tabindex="-1" onpointerdown={(e) => e.stopPropagation()}>
      <h3 id="paste-title">This looks like {offer.kind.label}</h3>
      {#if !preview}
        <p class="muted">
          {canFill ? 'Fill this request from it, or import it as new requests.' : 'Import it as new requests into this collection.'}
          {#if offer.kind.fills && !offer.http}A message request cannot take an HTTP request: import it instead.{/if}
        </p>
        <pre class="text">{excerpt}</pre>
      {:else}
        <label class="f"><span>Into collection</span>
          <select value={preview.collection} onchange={(e) => setCollection(e.currentTarget.value)}>
            {#each preview.collections as c}<option value={c.id}>{c.label}</option>{/each}
          </select>
        </label>
        {#if preview.previewed}
          <p class="sum">{preview.summary}</p>
          {#if preview.lossy.length}<ul class="muted">{#each preview.lossy as l}<li>{l}</li>{/each}</ul>{/if}
          {#each preview.items.filter((it) => it.skipped || it.problems.length) as it}
            <div class="item" class:skipped={it.skipped}>
              <strong>{it.name}</strong>{#if it.skipped}<span class="tag">cannot be imported</span>{/if}
              {#each it.problems as p}<div class="prob {p.severity}">{p.text}</div>{/each}
            </div>
          {/each}
          <div class="files">{#each preview.files as f}<div class:exists={f.exists}>+ {f.rel}{f.exists ? ' (exists)' : ''}</div>{/each}</div>
          {#if preview.existing}
            <label class="f"><span>{preview.existing} {preview.existing === 1 ? 'file already exists' : 'files already exist'}</span>
              <select bind:value={existing}>
                <option value="skip">Keep them, add only new files</option>
                <option value="overwrite">Overwrite them</option>
              </select>
            </label>
          {/if}
        {:else if !preview.error}
          <p class="muted">Reading it…</p>
        {/if}
        {#if preview.error}<p class="err">{preview.error}</p>{/if}
      {/if}
      {#if error}<p class="err">{error}</p>{/if}
      <div class="actions">
        <button type="button" class="link" onclick={moreOptions}>More options…</button>
        <span class="grow"></span>
        <button type="button" onclick={pasteAsText}>Paste as text</button>
        {#if !preview}
          <button type="button" class:primary={!canFill} onclick={importNew}>Import as new…</button>
          {#if canFill}<button type="button" class="primary" onclick={fill}>Fill this request</button>{/if}
        {:else}
          <button type="button" class="primary" disabled={!preview.canImport} onclick={() => ImportService.Import(existing, preview!.skipped > 0)}>
            {preview.skipped > 0 ? 'Import the rest' : 'Import'}
          </button>
        {/if}
      </div>
    </div>
  </div>
{/if}

<style>
  .scrim { position: fixed; inset: 0; background: rgb(0 0 0 / 0.4); display: grid; place-items: center; z-index: 60; }
  .dialog { width: min(560px, calc(100vw - 32px)); max-height: 80vh; overflow: auto; box-sizing: border-box; display: flex; flex-direction: column; gap: 10px; padding: 16px; background: var(--panel); border: 1px solid var(--line); border-radius: var(--r-lg); box-shadow: var(--shadow); }
  h3 { margin: 0; }
  p { margin: 0; line-height: 1.45; }
  .muted { color: var(--muted); font-size: var(--fs-sm); }
  ul.muted { margin: 0; padding-left: 18px; }
  .text, .files { margin: 0; padding: 8px 10px; max-height: 180px; overflow: auto; background: var(--bg); border: 1px solid var(--line); border-radius: var(--r-md); font: var(--fs-sm) var(--mono); white-space: pre-wrap; word-break: break-all; -webkit-user-select: text; user-select: text; }
  .sum { font-weight: 600; }
  .f { display: flex; flex-direction: column; gap: 3px; color: var(--muted); font-size: var(--fs-xs); }
  select { background: var(--bg); color: var(--text); border: 1px solid var(--line); border-radius: var(--r-md); padding: 5px 8px; font: inherit; }
  .item { border-left: 2px solid var(--line); padding: 2px 10px; font-size: var(--fs-sm); }
  .item.skipped { border-color: var(--fail); }
  .tag { margin-left: 8px; color: var(--fail); font-size: var(--fs-xs); }
  .prob.error { color: var(--fail); }
  .prob.warning { color: var(--warn); }
  .exists { color: var(--warn); }
  .err { color: var(--fail); font-size: var(--fs-sm); }
  .actions { display: flex; align-items: center; gap: 8px; }
  .grow { flex: 1; }
  button { background: transparent; color: var(--text); border: 1px solid var(--line); border-radius: var(--r-md); padding: 5px 12px; font: inherit; white-space: nowrap; }
  button:hover:not(:disabled) { border-color: var(--accent); }
  button:disabled { opacity: 0.4; }
  .primary { background: var(--accent); color: var(--on-accent); border-color: var(--accent); }
  .link { border: 0; padding: 5px 0; color: var(--muted); }
  .link:hover:not(:disabled) { color: var(--text); }
</style>
