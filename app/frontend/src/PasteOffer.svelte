<script lang="ts">
  import Field from './Field.svelte'
  import Modal from './Modal.svelte'
  import Button from './Button.svelte'
  // What to do with text pasted into a request's address that is really something to import (a curl
  // command, an OpenAPI document …): fill the request from it, import it as new requests, or paste it
  // as typed. Importing shows the import screen's preview here, in short.
  import { onDestroy } from 'svelte'
  import { ImportService, RequestService } from '../bindings/github.com/githugt/flowi/app/internal/bridge'
  import { app } from './lib/app.svelte'
  import { DEFAULT_EXISTING, EXISTING_CHOICES } from './lib/importkinds'
  import { fillNote } from './lib/fillnote'
  import { errorText } from './lib/errors'
  import { live } from './lib/live'
  import type { ImportView } from './lib/types'
  import { ui } from './lib/ui.svelte'
  import Code from './Code.svelte'
  import Select from './Select.svelte'
  import Icon from './Icon.svelte'
  import type { CodeLang } from './lib/highlight'

  const offer = $derived(app.pasteOffer)
  let preview = $state<ImportView | null>(null)
  let error = $state('')
  let existing = $state(DEFAULT_EXISTING)
  let stop: (() => void) | null = null

  const SHOWN_LINES = 8
  const excerpt = $derived.by(() => {
    const lines = (offer?.text ?? '').trim().split('\n')
    return lines.slice(0, SHOWN_LINES).join('\n') + (lines.length > SHOWN_LINES ? `\n… ${lines.length - SHOWN_LINES} more lines` : '')
  })
  // What colours the excerpt: curl and .http have readers of their own; a document is JSON or YAML by how it starts.
  const excerptLang = $derived.by((): CodeLang => {
    const id = offer?.kind.id
    if (id === 'curl' || id === 'http') return id
    if (id === 'flowi') return 'yaml'
    return (offer?.text.trimStart()[0] ?? '') === '{' ? 'json' : 'yaml'
  })
  // A request file fills a request of any kind; a curl command or a .http request only an HTTP one.
  const isRequestFile = $derived(offer?.kind.id === 'flowi')
  const note = $derived(offer ? fillNote(app.requests[offer.node]?.current, isRequestFile) : '')
  const canFill = $derived(!!offer?.kind.fills && (offer.http || isRequestFile))
  // The secrets a credential in the text becomes. The value is not kept, so the user is told where to set it.
  let secrets = $state<string[]>([])
  $effect(() => {
    const o = offer
    secrets = []
    if (!o || !o.kind.fills || isRequestFile) return
    RequestService.PastedSecrets(o.kind.id, o.text).then((s) => {
      if (offer === o) secrets = s ?? []
    })
  })

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

  // The import screen opens on what was recognised, with the text as its source: nothing to choose again.
  async function moreOptions() {
    const o = offer
    if (!o) return
    try {
      await ImportService.PreparePasted(o.kind.id, o.text, o.node)
    } catch (e) {
      error = errorText(e)
      return
    }
    close()
    app.openScreen('screen:import', 'Import')
  }

  // The chooser the new-request and Save dialogs use; an import fills a whole collection, so no folder.
  function chooseCollection(current: string) {
    ui.place = {
      title: 'Import into',
      action: 'Choose',
      tree: 'requests',
      collectionsOnly: true,
      row: `c:${current}`,
      done: (row) => setCollection(row.replace(/^c:/, '')),
    }
  }

  function setCollection(id: string) {
    ImportService.SetCollection(id)
    ImportService.Preview()
  }
</script>

<svelte:window onkeydown={(e) => offer && e.key === 'Escape' && close()} />

{#if offer}
  <Modal title={`This looks like ${offer.kind.label}`} width="min(560px, calc(100vw - 32px))" scroll onclose={close}>
      {#if !preview}
        <p class="muted">
          {isRequestFile && offer.node ? 'Fill this request from it: it takes the request file’s first version.' : canFill ? 'Fill this request from it, or import it as new requests.' : 'Import it as new requests into this collection.'}
          {#if offer.kind.fills && !offer.http && !isRequestFile}A message request cannot take an HTTP request: import it instead.{/if}
        </p>
        {#if canFill && note}<p class="muted" data-testid="fill-note">{note}</p>{/if}
        {#if canFill && secrets.length}
          <p class="muted" data-testid="fill-secrets">
            {secrets.length === 1 ? 'A credential in the text' : `${secrets.length} credentials in the text`} will be replaced by <code>{secrets.join(', ')}</code>. The value is not kept: set it in
            <Button type="button" variant="ghost" small onclick={() => { close(); app.openScreen('screen:secrets', 'Secrets') }}>Secrets</Button> after the fill.
          </p>
        {/if}
        <div class="text"><Code text={excerpt} lang={excerptLang} /></div>
      {:else}
        <div class="f"><span>Into collection</span>
          <button type="button" class="choose" onclick={() => chooseCollection(preview!.collection)} title="Choose a collection, or make a new one">
            {preview.collections.find((c) => c.id === preview!.collection)?.label ?? 'Choose a collection…'}<span class="chev"><Icon name="chevd" size={12} /></span>
          </button>
        </div>
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
            <Field label={`${preview.existing} ${preview.existing === 1 ? 'file already exists' : 'files already exist'}`}>
              <Select bind:value={existing}>
                {#each EXISTING_CHOICES.filter((c) => c.id !== 'refuse') as c (c.id)}<option value={c.id}>{c.label}</option>{/each}
              </Select>
            </Field>
          {/if}
        {:else if !preview.error}
          <p class="muted">Reading it…</p>
        {/if}
        {#if preview.error}<p class="err">{preview.error}</p>{/if}
      {/if}
      {#if error}<p class="err">{error}</p>{/if}
      <div class="actions">
        <Button type="button" variant="ghost" onclick={moreOptions}>More options…</Button>
        <span class="grow"></span>
        <Button type="button" onclick={pasteAsText}>Paste as text</Button>
        {#if !preview}
          {#if !isRequestFile || !offer.node}<Button type="button" variant={canFill ? 'secondary' : 'primary'} onclick={importNew}>Import as new…</Button>{/if}
          {#if canFill}<Button type="button" variant="primary" onclick={fill}>Fill this request</Button>{/if}
        {:else}
          <Button type="button" variant="primary" disabled={!preview.canImport} onclick={() => ImportService.Import(existing, preview!.skipped > 0)}>
            {preview.skipped > 0 ? 'Import the rest' : 'Import'}
          </Button>
        {/if}
      </div>
  </Modal>
{/if}

<style>
  p { margin: 0; line-height: 1.45; }
  .muted { color: var(--muted); font-size: var(--fs-sm); }
  ul.muted { margin: 0; padding-left: 18px; }
  .text { max-height: 180px; overflow: auto; border-radius: var(--r-md); }
  .files { margin: 0; padding: 8px 10px; max-height: 180px; overflow: auto; background: var(--bg); border: 1px solid var(--line); border-radius: var(--r-md); font: var(--fs-sm) var(--mono); white-space: pre-wrap; word-break: break-all; -webkit-user-select: text; user-select: text; }
  .sum { font-weight: 600; }
  .f { display: flex; flex-direction: column; gap: 3px; color: var(--muted); font-size: var(--fs-xs); }
  .choose { display: flex; justify-content: space-between; align-items: center; gap: 8px; text-align: left; width: 100%; box-sizing: border-box; background: var(--raised); color: var(--text); border: 0; box-shadow: inset 0 0 0 1px var(--line); border-radius: var(--r-md); padding: 0 12px; min-height: var(--ctl-h); font: inherit; font-weight: 500; }
  .choose:hover { background: var(--raised-3); }
  .chev { display: inline-flex; color: var(--muted); }
  .item { border-left: 2px solid var(--line); padding: 2px 10px; font-size: var(--fs-sm); }
  .item.skipped { border-color: var(--fail); }
  .tag { margin-left: 8px; color: var(--fail); font-size: var(--fs-xs); }
  .prob.error { color: var(--fail); }
  .prob.warning { color: var(--warn); }
  .exists { color: var(--warn); }
  .err { color: var(--fail); font-size: var(--fs-sm); }
  .actions { display: flex; align-items: center; gap: 8px; }
  .grow { flex: 1; }
</style>
