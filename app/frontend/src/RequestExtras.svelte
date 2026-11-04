<script lang="ts">
  import Toggle from './Toggle.svelte'
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
    shallow: boolean
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
    <Toggle checked={view.snippet.exclude} onchange={(on) => RequestService.SetSnippetExclude(node, on)}>Leave secrets out</Toggle>
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
  <div class="bar">
    <select bind:value={add}>
      <option value="">Add a setting…</option>
      {#each settings?.available.filter((a) => !rows.some((r) => r.key === a.key)) ?? [] as a}<option value={a.key}>{a.key}</option>{/each}
    </select>
    <button disabled={!add} onclick={async () => { await RequestService.AddSetting(node, add); add = ''; await loadSettings() }}>Add</button>
  </div>
  {#if rows.length}
    <p class="muted">Settings this version sets for itself; everything else comes from the layers above (collection, workspace, you).</p>
  {:else}
    <EmptyState icon="settings" title="No settings of its own" hint="Everything comes from the collection, the workspace and your own settings. Add one above to change it for this version only." compact />
  {/if}
  {#each rows as r, i}
    {@const spec = settings?.available.find((a) => a.key === r.key)}
    <div class="scard">
      <div class="shead">
        <code>{r.key}</code>
        <button class="x" aria-label="Remove setting" title="Remove this setting" onclick={() => { rows.splice(i, 1); pushRows() }}>×</button>
      </div>
      {#if spec}<p class="muted">{spec.description}</p>{/if}
      {#if spec?.choices?.length}
        <select bind:value={r.raw} onchange={pushRows} aria-label={r.key}>{#each spec.choices as c}<option value={c}>{c}</option>{/each}</select>
      {:else}
        <input bind:value={r.raw} oninput={pushRows} spellcheck="false" aria-label={r.key} />
      {/if}
    </div>
  {/each}
  {#each problems as p}<p class="err">{p}</p>{/each}
{:else if which === 'schema'}
  {#if schemas && !schemas.length}
    <EmptyState icon="schema" title="No schema yet" hint="A send learns one from the request and response bodies while the setting schemas.infer is on, and checks later sends against it." compact />
  {:else}
    <p class="muted">What a send learned about this version's request and responses, or what a spec said. Contracts are used to check what flows read from them.</p>
  {/if}
  {#if schemas?.some((s) => s.source === 'inferred')}
    <div class="bar">
      <span class="muted">Inferred schemas widen as later sends differ. Keep them as they are to check sends, and <code>flowi drift</code>, against them.</span><span class="spacer"></span>
      <button onclick={async () => { await RequestService.PromoteSchemas(node); loadSchemas() }}>Promote to drift baseline</button>
    </div>
  {/if}
  {#each schemas ?? [] as sc}
    <div class="schema">
      <div class="bar"><strong>{sc.title}</strong>
        <span class="muted">{sc.source}</span>
        {#if sc.shallow}<span class="chip" title="Inferred from the single answer a discovery got: empty arrays and optional fields are unseen. A send that reaches it confirms it.">inferred: shallow</span>{/if}
        <span class="spacer"></span>
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
  .bar { display: flex; align-items: center; flex-wrap: wrap; gap: 6px 10px; }
  .bar button { white-space: nowrap; }
  .spacer { flex: 1; }
  .chip { border: 1px solid var(--warn, var(--line)); border-radius: var(--r-pill); padding: 0 8px; font-size: var(--fs-xs); color: var(--warn, var(--muted)); }
  select, button { background: var(--raised); color: var(--text); border: 0; box-shadow: inset 0 0 0 1px var(--line); border-radius: var(--r-md); padding: 0 10px; min-height: var(--ctl-h); font: inherit; font-weight: 500; }
  select { appearance: none; -webkit-appearance: none; padding-right: 28px; background: var(--panel) url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 24 24' fill='none' stroke='rgb(140 146 158)' stroke-width='2' stroke-linecap='round' stroke-linejoin='round'%3E%3Cpath d='M6 9l6 6 6-6'/%3E%3C/svg%3E") no-repeat right 8px center / 14px; }
  button:disabled { opacity: 0.4; }
  .muted { color: var(--muted); font-size: var(--fs-sm); margin: 0; }
  .err { color: var(--fail); margin: 0; }
  h5 { margin: 8px 0 2px; color: var(--muted); font-size: var(--fs-xs); text-transform: uppercase; letter-spacing: 0.05em; }
  .row { display: flex; flex-direction: column; align-items: flex-start; gap: 2px; width: 100%; text-align: left; background: none; border: 0; padding: 3px 0; }
  .row:enabled:hover { background: var(--tint-hover); }
  .cookie { border: 1px solid var(--line); border-radius: var(--r-lg); padding: 6px 10px; margin: 6px 0; display: flex; flex-direction: column; gap: 2px; }
  code { font: var(--fs-sm) var(--mono); word-break: break-all; -webkit-user-select: text; user-select: text; }
  .x { border: 0; float: right; color: var(--muted); padding: 0 4px; }
  .scard { margin-top: 10px; padding: 12px 14px; display: flex; flex-direction: column; gap: 8px; background: var(--panel); border: 1px solid var(--line); border-radius: var(--r-lg); }
  .shead { display: flex; align-items: center; gap: 8px; }
  .shead code { flex: 1; font-size: var(--fs-md); font-weight: 600; }
  .shead .x { float: none; box-shadow: none; background: none; min-height: var(--ctl-h-sm); padding: 0 8px; }
  .shead .x:hover { background: var(--tint-hover); color: var(--text); }
  .scard input, .scard select { width: 100%; box-sizing: border-box; min-height: var(--ctl-h); background-color: var(--bg); color: var(--text); border: 1px solid var(--line); border-radius: var(--r-md); padding: 0 10px; font: inherit; }
  .scard select { padding-right: 28px; }
  .schema { margin: 8px 0; }
</style>
