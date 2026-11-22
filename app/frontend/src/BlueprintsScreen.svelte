<script lang="ts">
  import ScreenPage from './ScreenPage.svelte'
  import Button from './Button.svelte'
  import EmptyState from './EmptyState.svelte'
  import { errorText } from './lib/errors'
  import { onMount } from 'svelte'
  import { live } from './lib/live'
  import { BlueprintsService } from '../bindings/github.com/githugt/flowi/app/internal/bridge'
  import { ui } from './lib/ui.svelte'
  import type { BlueprintsView } from './lib/types'
  import { typeIcon, typeLabel } from './lib/card'
  import Icon from './Icon.svelte'
  import CodeEditor from './CodeEditor.svelte'
  import Select from './Select.svelte'

  type Blueprint = BlueprintsView['blueprints'][number]

  let view = $state<BlueprintsView>({ blueprints: [], collections: [] })

  onMount(() => live<BlueprintsView>('blueprints', () => BlueprintsService.State(), (v) => (view = v)))

  // The blueprint being edited as text, one at a time: its key, the text and the problems of the last save.
  let editing = $state<{ key: string; text: string; problems: string } | null>(null)

  async function edit(key: string) {
    try {
      editing = { key, text: await BlueprintsService.Source(key), problems: '' }
    } catch (e) {
      ui.fail(e)
    }
  }

  async function save() {
    if (!editing) return
    try {
      const problems = await BlueprintsService.SaveSource(editing.key, editing.text)
      if (problems) editing.problems = problems
      else editing = null
    } catch (e) {
      if (editing) editing.problems = errorText(e)
    }
  }

  // A new blueprint starts as hooks with one log step; its text is opened at once to make it what is wanted.
  function create() {
    ui.prompt({
      title: 'New blueprint',
      label: 'Name',
      initial: '',
      protocols: [{ id: 'post', label: 'Post-request hooks' }, { id: 'pre', label: 'Pre-request hooks' }],
      protocolsLabel: 'Kind',
      done: async (name, phase) => {
        try {
          await edit(await BlueprintsService.New(collection, name, phase || 'post'))
        } catch (e) {
          ui.fail(e)
        }
      },
    })
  }

  // One section per kind of request, for the hooks its new requests start with; then the hooks and flow
  // templates that belong to no kind.
  const sections: { group: Blueprint['group']; tone: string; icon: string; title: string; note: string }[] = [
    { group: 'http', tone: '--v-GET', icon: 'send', title: 'HTTP requests', note: 'What a new HTTP request starts with.' },
    { group: 'async', tone: '--v-MQTT', icon: 'stream', title: 'Message requests', note: 'What a new message request (Kafka, MQTT, WebSocket …) starts with.' },
    { group: 'grpc', tone: '--v-GRPC', icon: 'send', title: 'gRPC requests', note: 'What a new gRPC request starts with.' },
    { group: 'dataset', tone: '--v-SQL', icon: 'list', title: 'Dataset queries', note: 'What a new dataset query starts with.' },
    { group: 'hooks', tone: '--info', icon: 'hooks', title: 'Other hooks', note: 'Reusable hooks a request of any kind can add.' },
    { group: 'flow', tone: '--v-FLOW', icon: 'blueprint', title: 'Flow templates', note: 'Using one makes an ordinary flow; nothing links back.' },
  ]
  // Where New and a built-in's Edit put their file: a blueprint lives in a collection.
  let target = $state('')
  const collection = $derived(view.collections.some((c) => c.id === target) ? target : (view.collections[0]?.id ?? ''))

  // A built-in has no file: editing it gives the collection its own copy, which replaces it there.
  async function customize(key: string) {
    try {
      await edit(await BlueprintsService.Customize(key, collection))
    } catch (e) {
      ui.fail(e)
    }
  }

  // A step type's pill colour: what calls out is the accent, what checks is green, what branches is amber.
  const stepTone = (type: string) =>
    ({ request: '--accent', subflow: '--accent', fetch: '--accent', webhook: '--accent', assert: '--ok', log: '--info', set: '--data', save: '--data', digest: '--data',
      condition: '--warn', batch: '--warn', run_till: '--warn', parallel: '--warn', delay: '--pending', stop: '--fail', loop_control: '--warn' })[type] ?? '--muted'
  // A built-in steps aside for the chosen collection's own version of it, so each default is listed once.
  const replaced = (b: Blueprint) => b.builtIn && view.blueprints.some((o) => o.replacesDefault && o.collection === collection && o.group === b.group && o.kind === b.kind)
  const of = (group: Blueprint['group']) => view.blueprints.filter((b) => b.group === group && !replaced(b))
</script>

<ScreenPage>
  <div class="head">
    <p class="note">Blueprints are flow templates and reusable hooks kept in a collection's blueprints folder.</p>
    {#if view.collections.length > 1}
      <Select label="Collection that new and edited blueprints go to" value={collection} onchange={(e) => (target = e.currentTarget.value)}>
        {#each view.collections as c}<option value={c.id}>{c.label}</option>{/each}
      </Select>
    {/if}
    <Button variant="primary" disabled={!view.collections.length} title={view.collections.length ? undefined : 'Open a collection first'} onclick={create}>New blueprint…</Button>
  </div>
  <p class="note">New requests start with the built-in hooks of their kind. Edit one and the collection keeps its own version, which its new requests use from then on; Reset brings the built-in back. The setting <code>requests.seed_blueprint</code> can name other blueprints, or turn this off.</p>
  {#each sections as s (s.group)}
    {@const list = of(s.group)}
    <h2 style:--tone={`var(${s.tone})`}><span class="sicon"><Icon name={s.icon} size={15} /></span>{s.title}<span class="count">{list.length}</span></h2>
    <p class="note">{s.note}</p>
    {#if !list.length}<EmptyState icon={s.icon} title="None yet" hint="Edit a built-in one above, or make one from a flow's menu, and it shows here." compact />{/if}
    {#each list as b (b.key)}
      <div class="card" style:--tone={`var(${s.tone})`}>
        <div class="top"><strong>{b.name}</strong>
          {#if b.kind !== 'flow'}<span class="tag phase" class:pre={b.kind === 'pre-hooks'}>{b.kind === 'pre-hooks' ? 'Before sending' : 'On the response'}</span>{/if}
          {#if b.builtIn}<span class="tag builtin">Built in</span>{:else}{#if b.replacesDefault}<span class="tag edited">Replaces the built-in</span>{/if}<span class="muted">{b.collectionName} · <code>{b.path}</code></span>{/if}
          <span class="spacer"></span>
          {#if b.builtIn}
            <!-- Shipped with Flowi, so it has no file: Edit makes the collection's own copy. -->
            <Button disabled={!collection} title={collection ? undefined : 'Open a collection first'} onclick={() => customize(b.key)}>Edit…</Button>
          {:else if b.replacesDefault}
            <!-- It is already what new requests of its kind start with. -->
          {:else if b.kind === 'flow'}<Button onclick={() => (ui.blueprint = { collection: b.collection, folder: '', blueprint: b.key })}>New flow from this…</Button>
          {:else}<Button onclick={() => BlueprintsService.UseAsDefault(b.key)}>Use for new requests</Button>{/if}
          {#if !b.builtIn}<Button onclick={() => edit(b.key)}>Edit…</Button>{/if}
          {#if b.replacesDefault}<Button onclick={() => ui.confirm('Reset to the built-in', `Delete ${b.collectionName}'s own version of this default? Its new requests start with the built-in again; requests made already keep their hooks.`, () => BlueprintsService.Delete(b.key), 'Reset')}>Reset…</Button>
          {:else if !b.builtIn}<Button onclick={() => ui.confirm('Delete blueprint', `Delete the blueprint ${b.name}? Flows made from it are not affected.`, () => BlueprintsService.Delete(b.key))}>Delete…</Button>{/if}
        </div>
        {#if b.description}<p class="muted">{b.description}</p>{/if}
        {#if b.params.length}
          <div class="params">{#each b.params as p}<span class="tag" title={p.description || undefined}>{p.name}<span class="muted">: {p.type}</span></span>{/each}</div>
        {/if}
        {#if editing?.key === b.key}
          <div class="editor"><CodeEditor value={editing.text} onchange={(t) => editing && (editing.text = t)} /></div>
          {#if editing.problems}<pre class="problems">{editing.problems}</pre>{/if}
          <div class="actions">
            <span class="muted">Give it <code>params</code> and it becomes a flow template; <code>phase: pre</code> makes its steps pre-request hooks.</span>
            <span class="spacer"></span>
            <Button onclick={() => (editing = null)}>Cancel</Button><Button variant="primary" onclick={save}>Save</Button>
          </div>
        {/if}
        <details>
          <summary>{b.nodes} {b.nodes === 1 ? 'step' : 'steps'}</summary>
          <ol>
            {#each b.steps as step}
              <li><span class="type" style:--tone={`var(${stepTone(step.type)})`}><Icon name={typeIcon(step.type)} size={12} />{typeLabel(step.type)}</span><span class="name">{step.name}</span>{#if step.detail}<code title={step.detail}>{step.detail}</code>{/if}</li>
            {/each}
          </ol>
        </details>
      </div>
    {/each}
  {/each}
</ScreenPage>

<style>
  h2 { display: flex; align-items: center; gap: var(--sp-2); margin: 24px 0 2px; font-size: var(--fs-lg); font-weight: 600; }
  h2 .count { margin-left: 0; }
  .sicon { display: grid; place-items: center; width: 24px; height: 24px; border-radius: var(--r-md); background: color-mix(in srgb, var(--tone) 16%, transparent); color: var(--tone); }
  .note, .muted { color: var(--muted); font-size: var(--fs-sm); margin: 0 0 4px; }
  /* A card wears its section's colour as a bar on its leading edge. */
  .card { border: 1px solid var(--line); border-left: 3px solid color-mix(in srgb, var(--tone) 70%, transparent); border-radius: var(--r-xl); background: var(--panel); padding: 8px 12px; margin: 8px 0; }
  .top { display: flex; align-items: center; gap: 10px; min-height: var(--ctl-h); }
  .top strong { white-space: nowrap; }
  .top .muted { min-width: 0; margin: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .spacer { flex: 1; }
  .head { display: flex; align-items: flex-start; gap: var(--sp-3); }
  .head .note { flex: 1; }
  .editor { height: 320px; margin: var(--sp-2) 0; border: 1px solid var(--line); border-radius: var(--r-md); overflow: hidden; background: var(--sunk); -webkit-user-select: text; user-select: text; }
  .editor :global(.cm-editor) { height: 100%; }
  .problems { margin: 0 0 var(--sp-2); padding: var(--sp-2); border-radius: var(--r-md); background: var(--fail-t); color: var(--fail); font: var(--fs-sm) var(--mono); white-space: pre-wrap; }
  .actions { display: flex; align-items: center; gap: var(--sp-2); margin-bottom: var(--sp-2); }
  .tag { flex: none; white-space: nowrap; font-size: var(--fs-xs); border: 1px solid var(--line); border-radius: var(--r-pill); padding: 0 8px; color: var(--text); }
  .builtin { color: var(--accent); border-color: color-mix(in srgb, var(--accent) 45%, transparent); }
  .phase { color: var(--ok); border-color: transparent; background: var(--ok-t); }
  .phase.pre { color: var(--info); background: var(--info-t); }
  .edited { color: var(--dirty); border-color: color-mix(in srgb, var(--dirty) 45%, transparent); }
  .params { display: flex; flex-wrap: wrap; gap: var(--sp-1); margin: var(--sp-1) 0; }
  code { font: var(--fs-xs) var(--mono); }
  summary { color: var(--muted); font-size: var(--fs-sm); cursor: pointer; width: fit-content; }
  summary:hover { color: var(--text); }
  ol { list-style: none; margin: var(--sp-2) 0 var(--sp-1); padding: 0; display: grid; gap: var(--sp-1); }
  li { display: flex; align-items: center; gap: var(--sp-2); min-width: 0; font-size: var(--fs-sm); }
  .type { flex: none; display: inline-flex; align-items: center; gap: 5px; min-width: 64px; box-sizing: border-box; border-radius: var(--r-sm); padding: 0 7px; background: color-mix(in srgb, var(--tone) 16%, transparent); color: var(--tone); font: 600 var(--fs-xs)/18px var(--mono); }
  .name { flex: none; }
  li code { min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; color: var(--muted); }
</style>
