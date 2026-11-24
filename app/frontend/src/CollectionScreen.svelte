<script lang="ts">
  // The page of one collection, the one place that shows everything about an API: what it is about, how it stands, what
  // it relies on and how it is set up. It gathers what the library already measures (coverage, saved runs, secrets,
  // environments); the only thing it edits is the collection's variables.
  import { onDestroy, untrack } from 'svelte'
  import Button from './Button.svelte'
  import Select from './Select.svelte'
  import SearchField from './SearchField.svelte'
  import Segmented from './Segmented.svelte'
  import StatusIcon from './StatusIcon.svelte'
  import Verb from './Verb.svelte'
  import EmptyState from './EmptyState.svelte'
  import KVRows from './KVRows.svelte'
  import ScreenPage from './ScreenPage.svelte'
  import Status from './Status.svelte'
  import Tabs, { type TabItem } from './Tabs.svelte'
  import { app } from './lib/app.svelte'
  import { agoText } from './lib/ago'
  import { type Drift, type Endpoint, GROUPINGS, type GroupBy, type Impact, driftText, endpointMatches, impactText, groupCount, groupEndpoints, headline, levelChoices, levelShown, ofTotal, pageCollection, runTone, type SpecRule } from './lib/collectionpage'
  import { debounced } from './lib/debounce'
  import { errorText } from './lib/errors'
  import { latest } from './lib/latest'
  import { nodeById } from './lib/treewalk'
  import type { KV } from './lib/types'
  import { ui } from './lib/ui.svelte'
  import { CollectionService, ManagerService, WorkspaceService } from '../bindings/github.com/githugt/flowi/app/internal/bridge'

  interface Count { label: string; count: number }
  interface Page {
    id: string
    endpoints: Endpoint[]
    about: { name: string; description: string; specs: string[]; counts: Count[]; dependsOn: { name: string; repo: string; mounted: boolean }[]; exports: { id: string; label: string; file: string }[] }
    state: {
      operations: { request: string; node: string; used: string[]; statusesSeen: number; statuses: number; unread: number; impact: Impact; drift: Drift }[]
      operationsUsed: number
      operationsTotal: number
      statusesSeen: number
      statusesTotal: number
      flows: { name: string; node: string; status: string; env: string; ran: string }[]
      neverRan: number
      failing: number
      broken: string[]
      spec: { severity: string; message: string; hint: string; path: string; file: string; line: number }[]
      impactBroken: number
      driftMoved: number
      specTotal: number
      specErrors: number
    }
    uses: { auths: string[]; blueprints: string[]; overlays: string[]; secrets: { name: string; count: number; where: string }[] }
    setup: {
      variables: { name: string; value: string; description: string; perEnv: { env: string; value: string; secret: boolean }[] }[]
      environments: { name: string; overrides: number }[]
      own: { name: string; have: boolean }[]
    }
  }

  let { node }: { node: string } = $props()
  const id = $derived(pageCollection(node))

  let page = $state<Page | null>(null)
  let error = $state('')
  let tab = $state('about')
  let variables = $state<KV[]>([])
  let variableError = $state('')
  let rules = $state<SpecRule[]>([])
  let groupBy = $state<GroupBy>('path')
  let filter = $state('')
  const shown = $derived((page?.endpoints ?? []).filter((e) => endpointMatches(e, filter)))
  const groups = $derived(groupEndpoints(shown, groupBy))

  // An answer still on its way for another collection (or an older read) is dropped.
  const load = latest()
  // The variables are read only when the page opens: after that what is typed is the truth, and reading them back
  // would replace the row being typed in.
  async function reload(withVariables = false) {
    const current = load()
    error = ''
    try {
      const got = (await CollectionService.Page(id)) as unknown as Page
      const vars = withVariables ? ((await ManagerService.Variables(id)) as unknown as KV[]) : null
      const got2 = (await CollectionService.SpecRules(id)) as unknown as SpecRule[]
      if (!current()) return
      page = got
      rules = got2
      if (vars) variables = vars
    } catch (e) {
      if (current()) error = errorText(e)
    }
  }
  $effect(() => {
    if (id) void reload(true)
  })
  // The workspace tree changes when a file does: read the page again so it does not show what is gone.
  $effect(() => {
    void app.tree
    if (untrack(() => page)) void reload()
  })

  const head = $derived(page ? headline(page.state) : null)
  const tabs = $derived<TabItem[]>([
    { id: 'about', label: 'About' },
    { id: 'endpoints', label: 'Endpoints', count: page?.endpoints.length || undefined },
    { id: 'state', label: 'State', count: page ? page.state.failing + page.state.neverRan + page.state.specTotal + (page.state.operationsTotal - page.state.operationsUsed) || undefined : undefined, tone: page && page.state.failing > 0 ? 'fail' : undefined },
    { id: 'uses', label: 'Uses' },
    { id: 'setup', label: 'Setup', count: page?.setup.variables.length || undefined },
  ])

  function open(nodeId: string) {
    const n = nodeById(app.tree?.roots, nodeId)
    if (n) void app.open(n)
  }

  // The variables live in the collection's file: typing is saved after a pause, as in the Collections screen.
  const save = debounced(async (rows: KV[]) => {
    variableError = ''
    try {
      const needsConfirm = await ManagerService.SetVariables(id, rows.filter((r) => r.Key) as never, false)
      if (needsConfirm) {
        ui.confirm('Comments will be lost', 'The collection file has comments; saving rewrites it without them. Save anyway?', async () => {
          try {
            await ManagerService.SetVariables(id, rows.filter((r) => r.Key) as never, true)
            await reload()
          } catch (e) {
            variableError = errorText(e)
          }
        }, 'Save')
        return
      }
      await reload()
    } catch (e) {
      variableError = errorText(e)
    }
  }, 700)
  function typed(rows: KV[]) {
    variables = rows
    save(rows)
  }
  onDestroy(() => save.flush())

  async function copyIn() {
    try {
      const names = (await WorkspaceService.ImportEnvironments(id)) as unknown as string[]
      ui.toast(names.length ? `Copied ${names.join(', ')} into the workspace` : 'Nothing to copy')
      await reload()
    } catch (e) {
      ui.toast(errorText(e), 'error')
    }
  }
  async function setRule(rule: string, level: string) {
    try {
      await CollectionService.SetSpecRule(id, rule, level)
      await reload()
    } catch (e) {
      ui.toast(errorText(e), 'error')
    }
  }
  let exporting = $state('')
  async function saveExport(what: string) {
    exporting = what
    try {
      const file = await CollectionService.SaveExport(id, what)
      if (file) ui.toast(`Saved ${file}`)
    } catch (e) {
      ui.toast(errorText(e), 'error')
    } finally {
      exporting = ''
    }
  }
  // A drift check runs a flow, so it is asked for, one flow at a time or all of them.
  let drifting = $state('')
  async function checkDrift(flow: string) {
    drifting = flow || '*'
    try {
      const run = (await CollectionService.CheckDrift(id, flow)) as unknown as { checked: number; problems: string[] }
      await reload()
      const done = `Checked ${run.checked} ${run.checked === 1 ? 'flow' : 'flows'}`
      if (run.problems.length) ui.toast(`${done}. ${run.problems[0]}${run.problems.length > 1 ? ` (and ${run.problems.length - 1} more)` : ''}`, 'error')
      else ui.toast(done)
    } catch (e) {
      ui.toast(errorText(e), 'error')
    } finally {
      drifting = ''
    }
  }
  const checkAll = () =>
    ui.confirm('Run every flow?', 'Each flow of this collection runs for real with the selected environment, and sends its requests.', () => void checkDrift(''), 'Run and compare')
  const canCopy = $derived(page?.setup.own.some((e) => !e.have) ?? false)
  const now = () => new Date()
</script>

<ScreenPage width={900}>
  {#if error}
    <p class="err" role="alert">{error}</p>
  {:else if !page}
    <p class="muted" role="status" aria-busy="true">Reading the collection…</p>
  {:else}
    <h2>{page.about.name}</h2>
    {#if head}<p class="headline"><Status tone={head.tone}>{head.text}</Status></p>{/if}
    <Tabs items={tabs} active={tab} onpick={(t) => (tab = t)} label="Collection" />

    {#if tab === 'about'}
      <section>
        {#if page.about.description}<p>{page.about.description}</p>{:else}<p class="muted">This collection has no description. Say what it is for in its file, <code>flowi.yaml</code>.</p>{/if}
        <dl class="counts">
          {#each page.about.counts as c (c.label)}<div><dt>{c.label}</dt><dd>{c.count}</dd></div>{/each}
        </dl>
        <h3>API descriptions</h3>
        {#if page.about.specs.length}
          <p class="muted">Kept from the import, so an update can be compared with them.</p>
          <ul>{#each page.about.specs as s (s)}<li><code>{s}</code></li>{/each}</ul>
        {:else}
          <p class="muted">The collection kept no description of the API, so its requests and flows are its operations.</p>
        {/if}
        {#if page.about.exports.length}
          <h3>Export</h3>
          <p class="muted">Save a description as a file for someone who does not use Flowi. The requests as Flowi sends them is made from the requests and learned schemas and is always current; a kept description is saved as it stands, with its overlays and Flowi's own edits applied.</p>
          <ul class="rows">
            {#each page.about.exports as e (e.id)}
              <li>
                <span>{e.label}</span>
                <Button small loading={exporting === e.id} disabled={!!exporting} onclick={() => saveExport(e.id)}>Save…</Button>
              </li>
            {/each}
          </ul>
        {/if}
        <h3>Depends on</h3>
        {#if page.about.dependsOn.length}
          <ul>
            {#each page.about.dependsOn as d (d.name)}
              <li>{d.name} <Status tone={d.mounted ? 'ok' : 'warn'} plain>{d.mounted ? 'open' : 'not open'}</Status>{#if !d.mounted && d.repo} <span class="muted">clone <code>{d.repo}</code></span>{/if}</li>
            {/each}
          </ul>
        {:else}
          <p class="muted">This collection builds on no other. It can name some in <code>flowi.yaml</code>.</p>
        {/if}
      </section>
    {:else if tab === 'endpoints'}
      <section>
        <div class="bar">
          <span class="muted">Group by</span>
          <Segmented label="Group endpoints by" items={GROUPINGS} value={groupBy} onpick={(g) => (groupBy = g as GroupBy)} />
          <SearchField bind:value={filter} placeholder="Filter endpoints…" label="Filter endpoints" small style="margin-left: auto; width: 240px" />
        </div>
        {#if page.endpoints.length === 0}
          <EmptyState icon="list" title="No requests yet" hint="Import an API description or make a request, and it is listed here." compact />
        {:else if groups.length === 0}
          <EmptyState kind="filtered" query={filter} onclear={() => (filter = '')} compact />
        {:else}
          <p class="muted">{shown.length} of {page.endpoints.length} endpoints</p>
          {#each groups as g (g.key)}
            <h3 class="group">{g.key} <span class="muted">{groupCount(g.endpoints.length, groupBy)}</span></h3>
            {#each g.endpoints as e (e.node)}
              <details class="ep">
                <summary>
                  <Verb verb={e.verb || '—'} short />
                  <span class="where">{groupBy === 'path' ? e.name || e.path : e.where}</span>
                  {#if e.status}<StatusIcon mark={e.status} size={13} />{/if}
                  <span class="sum">{e.summary}</span>
                </summary>
                <div class="body">
                  <dl>
                    <dt>Request</dt><dd><Button variant="link" onclick={() => open(e.node)}>{e.path}</Button></dd>
                    {#if e.where !== e.path}<dt>Address</dt><dd><code>{e.where}</code></dd>{/if}
                    {#if e.operation}<dt>Operation</dt><dd><code>{e.operation}</code></dd>{/if}
                    {#if e.versions.length > 1}<dt>Versions</dt><dd>{e.versions.join(', ')}</dd>{/if}
                    {#if e.tags.length}<dt>Tags</dt><dd>{e.tags.join(', ')}</dd>{/if}
                    <dt>Flows</dt><dd>{#if e.used.length}{e.used.join(', ')}{:else}<span class="muted">no flow calls it</span>{/if}</dd>
                    {@render reading(e.impact, e.drift)}
                    {#if e.statuses.length}
                      <dt>Responses</dt>
                      <dd class="codes">{#each e.statuses as s (s.status)}<span class="chip" class:seen={s.seen} title={s.seen ? 'A saved run got this status' : 'No saved run got this status'}>{s.status}</span>{/each}{#if e.unread} <span class="muted">{e.unread} fields unread</span>{/if}</dd>
                    {/if}
                  </dl>
                </div>
              </details>
            {/each}
          {/each}
        {/if}
      </section>
    {:else if tab === 'state'}
      <section>
        <h3>Operations</h3>
        {#if page.state.operations.length === 0}
          <EmptyState icon="checks" title="No operation to measure" hint="This collection has no request imported from an API description with response schemas." compact />
        {:else}
          <p class="muted">{ofTotal(page.state.operationsUsed, page.state.operationsTotal)} operations are called by a flow; {ofTotal(page.state.statusesSeen, page.state.statusesTotal)} documented statuses were ever seen in a saved run. Those no flow calls come first.</p>
          <ul class="rows">
            {#each page.state.operations as o (o.request)}
              <li>
                {#if o.node}<Button variant="link" onclick={() => open(o.node)}>{o.request}</Button>{:else}<span>{o.request}</span>{/if}
                {#if o.used.length}<span class="muted">called by {o.used.join(', ')}</span>{:else}<Status tone="warn" plain>no flow calls it</Status>{/if}
                <span class="muted">{ofTotal(o.statusesSeen, o.statuses)} statuses{#if o.unread} · {o.unread} fields unread{/if}</span>
                {#if o.impact.consumers > 0}{@const w = impactText(o.impact)}<span title={o.impact.notes.join('\n')}><Status tone={w.tone} plain>{w.text}</Status></span>{/if}
                {#if o.drift.checked}{@const w = driftText(o.drift)}<span title={o.drift.notes.join('\n')}><Status tone={w.tone} plain>drift: {w.text}</Status></span>{/if}
              </li>
            {/each}
          </ul>
        {/if}
        <h3>Flows</h3>
        {#if page.state.flows.length > 0}
          <p class="muted">Drift is what a response has that its stored schema does not, or lacks that it has. Checking runs the flow for real, with the selected environment, and shows the result per operation; nothing is stored. The result is kept until the app closes.</p>
          <Button small loading={drifting === '*'} disabled={!!drifting} onclick={checkAll}>Check drift on every flow</Button>
        {/if}
        {#if page.state.flows.length === 0}
          <EmptyState icon="flow" title="No flows yet" hint="A flow runs requests in order; its saved runs show here." compact />
        {:else}
          <ul class="rows">
            {#each page.state.flows as f (f.node)}
              <li>
                <Button variant="link" onclick={() => open(f.node)}>{f.name}</Button>
                {#if f.status}<Status tone={runTone(f.status)}>{f.status}</Status><span class="muted">{[agoText(f.ran, now()), f.env].filter(Boolean).join(' · ')}</span>{:else}<span class="muted">never ran</span>{/if}
                <Button variant="ghost" small loading={drifting === f.node} disabled={!!drifting} title="Run this flow and compare each response with its stored schema" onclick={() => checkDrift(f.node)}>Check drift</Button>
              </li>
            {/each}
          </ul>
        {/if}
        <h3>API description{#if page.state.specTotal} <span class="muted">({page.state.specTotal})</span>{/if}</h3>
        {#if page.state.specTotal === 0}
          {#if page.about.specs.length}
            <p class="muted">The kept description passes its rules.</p>
          {:else}
            <p class="muted">This collection kept no description to check.</p>
          {/if}
        {:else}
          <ul class="rows">
            {#each page.state.spec as f (f.path + f.message)}
              <li>
                <Status tone={f.severity === 'error' ? 'bad' : 'warn'}>{f.severity}</Status>
                <span>{f.message}</span>
                {#if f.hint}<span class="muted">{f.hint}</span>{/if}
                {#if f.file}<code class="at" title={f.path ? `${f.file}, at ${f.path}` : f.file}>{f.file}{f.line ? `:${f.line}` : ''}</code>{/if}
              </li>
            {/each}
          </ul>
          {#if page.state.specTotal > page.state.spec.length}<p class="muted">{page.state.specTotal - page.state.spec.length} more are in the Problems panel.</p>{/if}
        {/if}
        <details class="rules">
          <summary>Rules ({rules.filter((r) => r.chosen).length} chosen)</summary>
          <p class="muted">How seriously each rule is taken for this collection. A choice is saved in <code>descriptions/lint.yaml</code>, which can be shared through git.</p>
          {#each rules as r (r.name)}
            <div class="rule">
              <span class="what"><code>{r.name}</code><span class="muted">{r.about}</span></span>
              <Select label={`Level of ${r.name}`} size="sm" value={levelShown(r)} onchange={(e) => setRule(r.name, e.currentTarget.value)}>
                {#each levelChoices(r) as c (c.value)}<option value={c.value}>{c.label}</option>{/each}
              </Select>
            </div>
          {/each}
        </details>
        {#if page.state.broken.length}
          <h3>Files that cannot be read</h3>
          <ul>{#each page.state.broken as b (b)}<li><code>{b}</code></li>{/each}</ul>
        {/if}
      </section>
    {:else if tab === 'uses'}
      <section>
        {@render named('Authentications', page.uses.auths, 'This collection has no named authentication.')}
        {@render named('Blueprints', page.uses.blueprints, 'This collection has no blueprint.')}
        <p class="muted">A blueprint is a flow template or a set of hooks. Make a flow one from its ⋯ menu with "Save as blueprint…", or start a hook blueprint on the Blueprints screen.</p>
        <Button small onclick={() => app.openScreen('screen:blueprints', 'Blueprints')}>Open Blueprints</Button>
        {@render named('Overlays', page.uses.overlays, 'This collection has no overlay.')}
        <h3>Secrets</h3>
        {#if page.uses.secrets.length}
          <p class="muted">Written as <code>{'{{secret.name}}'}</code> in this collection. Their values are in the vault, not here.</p>
          <ul class="rows">{#each page.uses.secrets as s (s.name)}<li><code>{s.name}</code><span class="muted">{s.where}</span></li>{/each}</ul>
        {:else}
          <p class="muted">This collection reads no secret. One is written as <code>{'{{secret.name}}'}</code>.</p>
        {/if}
      </section>
    {:else}
      <section>
        <h3>Variables</h3>
        <p class="muted">Values every request and flow here can read as <code>{'{{collection_var.name}}'}</code>. An environment's variable of the same name overrides one.</p>
        <KVRows rows={variables} keyLabel="name" onchange={typed} describe
          empty={{ icon: 'list', title: 'No collection variables', hint: 'A base URL or a region that belongs to this collection, not to an environment.', add: 'Add variable' }} />
        {#if variableError}<p class="err">{variableError}</p>{/if}
        {#each page.setup.variables.filter((v) => v.perEnv.length) as v (v.name)}
          <p class="over"><code>{v.name}</code> is overridden in {v.perEnv.map((e) => `${e.env}${e.secret ? ' (secret)' : ` = ${e.value}`}`).join(', ')}</p>
        {/each}
        <h3>Environments</h3>
        {#if page.setup.environments.length}
          <ul class="rows">{#each page.setup.environments as e (e.name)}<li>{e.name}<span class="muted">{e.overrides ? `overrides ${e.overrides} of the variables` : 'overrides none'}</span></li>{/each}</ul>
        {:else}
          <p class="muted">The workspace has no environment yet.</p>
        {/if}
        {#if page.setup.own.length}
          <h3>Shipped with the collection</h3>
          <p class="muted">Environment files in its own <code>environments/</code> folder. The workspace's environments are shared by every collection, so these are only copied in.</p>
          <ul class="rows">{#each page.setup.own as e (e.name)}<li>{e.name}<span class="muted">{e.have ? 'the workspace has one of that name' : 'not in the workspace'}</span></li>{/each}</ul>
          {#if canCopy}<Button small onclick={copyIn}>Copy into the workspace</Button>{/if}
        {/if}
      </section>
    {/if}
  {/if}
</ScreenPage>

{#snippet reading(impact: Impact, drift: Drift)}
  {@const read = impactText(impact)}
  {@const moved = driftText(drift)}
  <dt>Read by</dt>
  <dd><Status tone={read.tone} plain>{read.text}</Status>{#if impact.notes.length}<ul class="notes">{#each impact.notes as n (n)}<li>{n}</li>{/each}</ul>{/if}</dd>
  <dt>Drift</dt>
  <dd>
    <Status tone={moved.tone} plain>{moved.text}</Status>
    {#if drift.checked}<span class="muted">via {drift.via.join(', ')}{drift.at ? `, ${drift.at}` : ''}</span>{/if}
    {#if drift.notes.length}<ul class="notes">{#each drift.notes as n (n)}<li>{n}</li>{/each}</ul>{/if}
  </dd>
{/snippet}

{#snippet named(title: string, names: string[], none: string)}
  <h3>{title}</h3>
  {#if names.length}<ul>{#each names as n (n)}<li>{n}</li>{/each}</ul>{:else}<p class="muted">{none}</p>{/if}
{/snippet}

<style>
  h2 { margin: 0 0 6px; }
  h3 { margin: 22px 0 4px; }
  .headline { margin: 0 0 12px; }
  section { margin-top: 12px; }
  .muted { color: var(--muted); font-size: var(--fs-sm); margin: 0 0 6px; }
  .err { color: var(--fail); }
  .counts { display: flex; flex-wrap: wrap; gap: 10px; margin: 14px 0; }
  .counts div { border: 1px solid var(--line); border-radius: var(--r-lg); padding: 8px 14px; min-width: 96px; }
  .counts dt { color: var(--muted); font-size: var(--fs-xs); }
  .counts dd { margin: 0; font-size: var(--fs-lg, 1.25em); font-weight: 600; }
  .rows { list-style: none; padding: 0; margin: 6px 0; }
  .rows li { display: flex; align-items: center; flex-wrap: wrap; gap: 10px; padding: 6px 2px; border-bottom: 1px solid var(--line); font-size: var(--fs-sm); }
  .rows li .muted { margin: 0; }
  .bar { display: flex; align-items: center; gap: 10px; margin: 6px 0 12px; }
  .group { margin: 18px 0 6px; font-size: var(--fs-md, 1em); }
  .group .muted { margin-left: 6px; font-weight: 400; }
  .ep { border: 1px solid var(--line); border-radius: var(--r-lg); margin: 4px 0; }
  .ep summary { display: flex; align-items: center; gap: 10px; padding: 7px 12px; cursor: pointer; list-style: none; font-size: var(--fs-sm); }
  .ep summary::-webkit-details-marker { display: none; }
  .ep summary::before { content: '›'; color: var(--muted); width: 8px; transition: transform 0.12s; }
  .ep[open] summary::before { transform: rotate(90deg); }
  .where { font-weight: 600; }
  .sum { margin-left: auto; color: var(--muted); text-align: right; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .body { padding: 4px 14px 10px 30px; border-top: 1px solid var(--line); font-size: var(--fs-sm); }
  .body dl { display: grid; grid-template-columns: 90px 1fr; gap: 4px 12px; margin: 8px 0 0; }
  .body dt { color: var(--muted); }
  .body dd { margin: 0; }
  .codes { display: flex; gap: 6px; flex-wrap: wrap; align-items: center; }
  .chip { border: 1px solid var(--line); border-radius: var(--r-pill); padding: 0 8px; font-size: var(--fs-xs); color: var(--muted); }
  .chip.seen { color: var(--text); border-color: var(--accent); }
  .rules { margin: 8px 0; }
  .rules summary { cursor: pointer; }
  .rule { display: flex; align-items: center; justify-content: space-between; gap: 12px; padding: 6px 0; border-bottom: 1px solid var(--line); font-size: var(--fs-sm); }
  .what { display: flex; flex-direction: column; gap: 2px; }
  .over { font-size: var(--fs-sm); color: var(--muted); margin: 4px 0; }
  code { font: var(--fs-xs) var(--mono); }
  .at { margin-left: auto; color: var(--muted); }
  .notes { margin: 2px 0 0; padding-left: 16px; color: var(--muted); font-size: var(--fs-xs); }
</style>
