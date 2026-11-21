<script lang="ts">
  import Checkbox from './Checkbox.svelte'
  import IconButton from './IconButton.svelte'
  // The sections only a dataset request has: the query, and the values it is sent with.
  import { RequestService } from '../bindings/github.com/githugt/flowi/app/internal/bridge'
  import type { DatasetBlock, RequestView } from './lib/types'
  import Button from './Button.svelte'
  import { parseCount } from './lib/count'
  import CodeArea from './CodeArea.svelte'
  import DataTableChips from './DataTableChips.svelte'
  import TextInput from './TextInput.svelte'
  import Field from './Field.svelte'
  import EmptyState from './EmptyState.svelte'
  import { credentialsBlocked, isFileDriver, moveTemplatesToValues, valuesNote, serverCredentials, withServerCredentials } from './lib/dataset'

  let { view, section }: { view: RequestView; section: string } = $props()
  const node = $derived(view.node)
  const block = $derived(view.dataset!.block)
  const isFile = $derived(isFileDriver(block.Driver))
  const params = $derived(block.Params ?? [])
  // Placeholders are $1, $2 … for PostgreSQL; MySQL and MariaDB use ?; SQLite takes either.
  const placeholder = $derived(block.Driver === 'postgres' ? '$1, $2 …' : block.Driver === 'sqlite' || block.Driver === 'csv' || block.Driver === 'xlsx' || block.Driver === 'parquet' ? '? or $1, $2 …' : '?')

  // Writes: the block comes from the latest view, with one change.
  function put(change: (b: DatasetBlock) => void) {
    const b = structuredClone($state.snapshot(block)) as DatasetBlock
    change(b)
    RequestService.SetDataset(node, b as never)
  }
  const setParam = (i: number, text: string) => put((b) => ((b.Params ??= []), (b.Params[i] = text)))
  const addParam = () => put((b) => ((b.Params ??= []), b.Params.push('')))
  const removeParam = (i: number) => put((b) => b.Params?.splice(i, 1))
  const creds = $derived(serverCredentials(block.Server) ?? { user: '', password: '' })
  const setCreds = (user: string, password: string) => put((b) => (b.Server = withServerCredentials(b.Server, user, password)))
  // A template in the statement is refused at send time: the value goes in the Values tab, and the fix does the move.
  const movable = $derived(moveTemplatesToValues(block.Query, block.Params ?? [], block.Driver))
  const moveToValues = () => movable && put((b) => ((b.Query = movable.query), (b.Params = movable.params)))
  // The statement and the Values list must agree on how many placeholders there are.
  const mismatch = $derived(valuesNote(block.Query, block.Driver, params.length))
  const addMissing = () => put((b) => ((b.Params ??= []), b.Params.push(...Array.from({ length: mismatch?.missing ?? 0 }, () => ''))))
  let limitError = $state('')
  function setLimit(text: string) {
    const n = parseCount(text)
    limitError = n.error
    if (!n.error) put((b) => (b.Limit = n.value))
  }
</script>

{#if section === 'Query'}
  {#if isFile}{#key block.Driver + '|' + block.Server}<DataTableChips {node} query={block.Query} setQuery={(t) => put((b) => (b.Query = t))} />{/key}{/if}
  <CodeArea
    value={block.Query}
    lang="sql"
    oninput={(t) => put((b) => (b.Query = t))}
    empty={{ icon: 'body', title: 'No query', hint: 'Click to write the query: one SELECT, WITH, VALUES, SHOW, EXPLAIN or DESCRIBE.' }}
  />
  {#if movable}
    <p class="writes" role="alert">⚠ A template in the query is refused when you send. Values go in the <strong>Values</strong> tab: write <code>{placeholder}</code> here.
      <Button type="button" small onclick={moveToValues}>Move to Values</Button></p>
  {/if}
  {#if mismatch}
    <p class="writes" role="status">{mismatch.text}
      {#if mismatch.missing}<Button type="button" small onclick={addMissing}>Add {mismatch.missing === 1 ? 'the missing value' : 'the missing values'}</Button>{/if}</p>
  {/if}
  {#if block.Writes}
    <p class="writes" role="alert">{isFile
      ? '⚠ This query may change the files: the tables it changes are written back (a CSV or Parquet file is rewritten, an Excel workbook has its changed cells updated). Keep a copy of data you cannot recreate.'
      : '⚠ This query may change data. The read-only guards are off for it: connect as a user that can only do what you mean it to.'}</p>
  {/if}
  <span class="limit"><Checkbox checked={!!block.Writes} onchange={(v) => put((b) => (b.Writes = v))}>Allow this query to change {isFile ? 'the files' : 'data'} (INSERT, UPDATE, DELETE …)</Checkbox></span>
  <p class="muted note">
    {isFile ? (block.Writes ? 'Writes are allowed for this request: the changed tables are written back to their files.' : 'Read-only: a statement that changes data is refused, and the files are never changed.') : block.Writes ? 'Writes are allowed for this request.' : 'Read-only: the query is sent in a read-only transaction, and a statement that changes data is refused.'} Put values in <strong>Values</strong> and write
    them as <code>{placeholder}</code>: they travel apart from the statement, so a value can never change what it means.
    {#if !isFile}For the strongest guarantee, connect as a database user that can only read.{/if}
  </p>
  {#if isFile}
    <details class="muted note filenote">
      <summary>What writing to files can and cannot do</summary>
      <ul>
        <li>Only changes to the rows and cells of the tables in the files are written back. Creating or dropping a table is not.</li>
        <li><strong>CSV:</strong> rewritten whole. Cells the query did not touch keep their text (<code>007</code> stays <code>007</code>); line endings become <code>\n</code>.</li>
        <li><strong>Excel:</strong> only the changed cells are updated, so other sheets and formatting stay. A formula cell the query overwrites loses its formula.</li>
        <li><strong>Parquet:</strong> rewritten whole with its own column types. Number, true/false and text columns only: a value a column cannot hold, or a date or nested column, is refused and nothing is written.</li>
        <li>The setting <code>datasets.read_only</code> forbids writing for every query.</li>
      </ul>
    </details>
  {/if}
  <label class="limit">Keep at most
    <input class="mini" inputmode="numeric" placeholder="1000" value={block.Limit || ''} onchange={(e) => setLimit(e.currentTarget.value)} aria-label="Most rows kept" />
    rows
  </label>
  {#if limitError}<p class="writes" role="alert">{limitError}</p>{/if}
{:else if section === 'Credentials'}
  {#if credentialsBlocked(block.Server)}
    <p class="muted note" role="note">{credentialsBlocked(block.Server)}</p>
  {:else}
    <Field label="User"><TextInput mono value={creds.user} placeholder="reader" ontext={(t) => setCreds(t, creds.password)} /></Field>
    <Field label="Password"><TextInput mono value={creds.password} placeholder={'{{secret.db_password}}'} ontext={(t) => setCreds(creds.user, t)} /></Field>
  {/if}
  <p class="muted note">
    The login is part of the connection URL in the server field: these two boxes write it there. Keep the password out of the file: write
    <code>{'{{secret.db_password}}'}</code> and give the secret its value under Secrets. Saving a request with a written password offers to do this for you.
  </p>
{:else if section === 'Values'}
  {#if mismatch}
    <p class="writes" role="status">{mismatch.text}
      {#if mismatch.missing}<Button type="button" small onclick={addMissing}>Add {mismatch.missing === 1 ? 'the missing value' : 'the missing values'}</Button>{/if}</p>
  {/if}
  {#if params.length === 0}
    <EmptyState icon="params" title="No values" hint={`Each value is sent for a placeholder of the query (${placeholder}), in order. Templates such as {{id}} work here.`} compact>
      <Button onclick={addParam}>Add value</Button>
    </EmptyState>
  {:else}
    {#each params as value, i}
      <div class="row">
        <span class="n" title="The placeholder this value is sent for">{block.Driver === 'postgres' ? '$' + (i + 1) : '#' + (i + 1)}</span>
        <TextInput mono {value} placeholder={'{{id}}'} ontext={(t) => setParam(i, t)} />
        <IconButton icon="close" size={12} danger label={`Remove value ${i + 1}`} hint="Remove" onclick={() => removeParam(i)} />
      </div>
    {/each}
    <Button small onclick={addParam}>Add value</Button>
  {/if}
{/if}

<style>
  .note { margin: 8px 0 0; font-size: var(--fs-xs); }
  .filenote summary { cursor: pointer; }
  .filenote ul { margin: 6px 0 0; padding-left: 18px; }
  .writes { color: var(--warn); font-size: var(--fs-sm); font-weight: 600; margin: 8px 0 0; }
  .limit { display: inline-flex; gap: 6px; align-items: center; margin-top: 8px; color: var(--muted); font-size: var(--fs-xs); }
  .mini { width: 5em; box-sizing: border-box; background: var(--panel); color: var(--text); border: 1px solid var(--line); border-radius: var(--r-md); padding: 3px 6px; font: inherit; }
  .row { display: grid; grid-template-columns: 2.4em 1fr auto; gap: 6px; align-items: center; margin-bottom: 6px; }
  .n { color: var(--muted); font-family: var(--mono, monospace); font-size: var(--fs-xs); text-align: right; }
</style>
