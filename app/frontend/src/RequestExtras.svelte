<script lang="ts">
  import EmptyState from './EmptyState.svelte'
  import Code from './Code.svelte'
  // The request's side tabs: the code snippet, who uses it, and the cookie jar.
  import { Clipboard } from '@wailsio/runtime'
  import { RequestService } from '../bindings/github.com/githugt/flowi/app/internal/bridge'
  import { app } from './lib/app.svelte'
  import { ui } from './lib/ui.svelte'
  import type { CookiesView, RequestView, UsedByView } from './lib/types'

  let { view, which }: { view: RequestView; which: 'snippet' | 'usedby' | 'cookies' | 'settings' | 'schema' } = $props()
  const node = $derived(view.node)

  let used = $state<UsedByView | null>(null)
  let cookies = $state<CookiesView | null>(null)

  interface SettingsView {
    rows: { key: string; raw: string }[]
    available: { key: string; description: string; default: string; choices?: string[]; set: boolean }[]
    problems: string[]
  }
  interface SchemaView {
    title: string
    source: string
    text: string
    kind: string
    status: number
  }
  let settings = $state<SettingsView | null>(null)
  let rows = $state<{ key: string; raw: string }[]>([])
  let problems = $state<string[]>([])
  let schemas = $state<SchemaView[] | null>(null)
  let add = $state('')

  async function loadSettings() {
    settings = (await RequestService.Settings(node)) as unknown as SettingsView
    rows = settings.rows.map((r) => ({ ...r }))
  }
  async function pushRows() {
    problems = (await RequestService.SetSettings(node, $state.snapshot(rows))) as string[]
  }
  async function loadSchemas() {
    schemas = (await RequestService.Schemas(node)) as unknown as SchemaView[]
  }

  async function loadCookies() {
    cookies = (await RequestService.Cookies(node)) as unknown as CookiesView
  }

  $effect(() => {
    if (which === 'usedby') RequestService.UsedBy(node).then((v) => (used = v as unknown as UsedByView))
    if (which === 'cookies') loadCookies()
    if (which === 'settings') loadSettings()
    if (which === 'schema') loadSchemas()
  })

  function copy(text: string) {
    Clipboard.SetText(text)
    ui.toast('Copied')
  }

  function openRow(row?: string) {
    const n = row && app.find(row)
    if (n) app.open(n)
  }
</script>

{#if which === 'snippet'}
  <div class="bar">
    <select value={view.snippet.format} onchange={(e) => RequestService.SetSnippetFormat(node, e.currentTarget.value)} aria-label="Format">
      {#each view.snippet.formats as f}<option value={f}>{f}</option>{/each}
    </select>
    <label class="check"><input type="checkbox" checked={view.snippet.exclude} onchange={(e) => RequestService.SetSnippetExclude(node, e.currentTarget.checked)} /> Leave secrets out</label>
    <span class="spacer"></span>
    <button disabled={!view.snippet.text} onclick={() => copy(view.snippet.text)}>Copy</button>
  </div>
  {#if view.snippet.error}<p class="err">{view.snippet.error}</p>{/if}
  <Code text={view.snippet.text} />
{:else if which === 'usedby'}
  {#if !used}<p class="muted">Looking…</p>
  {:else}
    {#if used.error}<p class="err">{used.error}</p>{/if}
    {#if !used.rows.length}<p class="muted">Nothing uses this request yet.</p>{/if}
    {#each used.rows as r}
      {#if r.header}<h5>{r.header}</h5>
      {:else}
        <button class="row" disabled={!r.opens} onclick={() => openRow(r.row)}>
          <span>{r.text}</span>{#if r.detail}<span class="muted">{r.detail}</span>{/if}
        </button>
      {/if}
    {/each}
  {/if}
{:else if which === 'settings'}
  {#if rows.length}
    <p class="muted">Settings this version sets for itself; everything else comes from the layers above (collection, workspace, you).</p>
  {:else}
    <EmptyState icon="settings" title="No settings of its own" hint="Everything comes from the collection, the workspace and your own settings. Add one below to change it for this version only." compact />
  {/if}
  {#each rows as r, i}
    {@const spec = settings?.available.find((a) => a.key === r.key)}
    <div class="srow">
      <code>{r.key}</code>
      {#if spec?.choices?.length}
        <select bind:value={r.raw} onchange={pushRows}>{#each spec.choices as c}<option value={c}>{c}</option>{/each}</select>
      {:else}
        <input bind:value={r.raw} oninput={pushRows} spellcheck="false" />
      {/if}
      <button class="x" aria-label="Remove setting" onclick={() => { rows.splice(i, 1); pushRows() }}>×</button>
    </div>
    {#if spec}<p class="muted">{spec.description}</p>{/if}
  {/each}
  {#each problems as p}<p class="err">{p}</p>{/each}
  <div class="bar">
    <select bind:value={add}>
      <option value="">Add a setting…</option>
      {#each settings?.available.filter((a) => !rows.some((r) => r.key === a.key)) ?? [] as a}<option value={a.key}>{a.key}</option>{/each}
    </select>
    <button disabled={!add} onclick={async () => { await RequestService.AddSetting(node, add); add = ''; await loadSettings() }}>Add</button>
  </div>
{:else if which === 'schema'}
  {#if schemas && !schemas.length}
    <EmptyState icon="schema" title="No schema yet" hint="A send learns one from the request and response bodies while the setting schemas.infer is on, and checks later sends against it." compact />
  {:else}
    <p class="muted">What a send learned about this version's request and responses, or what a spec said. Contracts are used to check what flows read from them.</p>
  {/if}
  {#each schemas ?? [] as sc}
    <div class="schema">
      <div class="bar"><strong>{sc.title}</strong><span class="muted">{sc.source}</span><span class="spacer"></span>
        <button onclick={async () => { await RequestService.DeleteSchema(node, sc.kind, sc.status); loadSchemas() }}>Delete</button></div>
      <Code text={sc.text} lang="json" />
    </div>
  {/each}
{:else}
  <div class="bar">
    <span class="muted">Saved cookies of this collection</span><span class="spacer"></span>
    <button disabled={!cookies?.cookies.length} onclick={async () => { await RequestService.ClearCookies(node); loadCookies() }}>Clear all</button>
  </div>
  {#if cookies?.error}<p class="err">{cookies.error}</p>{/if}
  {#if cookies && !cookies.cookies.length}<p class="muted">The jar is empty.</p>{/if}
  {#each cookies?.cookies ?? [] as c (c.key)}
    <div class="cookie">
      <div><strong>{c.name}</strong> <span class="muted">{c.domain}{c.path}</span>
        <button class="x" aria-label="Delete cookie" onclick={async () => { await RequestService.DeleteCookie(node, c.key); loadCookies() }}>×</button></div>
      <code>{c.value}</code>
      <div class="muted">expires {c.expires}{c.secure ? ' · secure' : ''}{c.httpOnly ? ' · http-only' : ''}</div>
    </div>
  {/each}
{/if}

<style>
  .bar { display: flex; align-items: center; gap: 10px; }
  .spacer { flex: 1; }
  .check { display: flex; align-items: center; gap: 4px; font-size: var(--fs-sm); }
  select, button { background: transparent; color: var(--text); border: 1px solid var(--line); border-radius: var(--r-md); padding: 3px 10px; font: inherit; }
  select { background: var(--bg); }
  button:disabled { opacity: 0.4; }
  .muted { color: var(--muted); font-size: var(--fs-sm); margin: 0; }
  .err { color: var(--fail); margin: 0; }
  h5 { margin: 8px 0 2px; color: var(--muted); font-size: var(--fs-xs); text-transform: uppercase; letter-spacing: 0.05em; }
  .row { display: flex; flex-direction: column; align-items: flex-start; gap: 2px; width: 100%; text-align: left; background: none; border: 0; padding: 3px 0; }
  .row:enabled:hover { background: color-mix(in srgb, var(--accent) 10%, transparent); }
  .cookie { border: 1px solid var(--line); border-radius: var(--r-lg); padding: 6px 10px; margin: 6px 0; display: flex; flex-direction: column; gap: 2px; }
  code { font: var(--fs-sm) var(--mono); word-break: break-all; -webkit-user-select: text; user-select: text; }
  .x { border: 0; float: right; color: var(--muted); padding: 0 4px; }
  .srow { display: flex; gap: 6px; align-items: center; margin-top: 6px; }
  .srow code { flex: 0 0 38%; font-size: var(--fs-sm); }
  .srow input, .srow select { flex: 1; min-width: 0; background: var(--bg); color: var(--text); border: 1px solid var(--line); border-radius: var(--r-md); padding: 4px 8px; font: inherit; }
  .schema { margin: 8px 0; }
</style>
