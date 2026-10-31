<script lang="ts">
  import EmptyState from './EmptyState.svelte'
  // What is kept for a request: the responses it saved (in the workspace, for
  // the replayer and for showing again) and the examples pinned next to it (in
  // the collection, committed). One at a time can be shown, pinned and deleted.
  import { errorText } from './lib/errors'
  import { RequestService } from '../bindings/github.com/githugt/flowi/app/internal/bridge'
  import { ui } from './lib/ui.svelte'
  import type { RequestView, StoredView } from './lib/types'

  let { view }: { view: RequestView } = $props()
  const node = $derived(view.node)

  let rows = $state<StoredView[]>([])
  const examples = $derived(rows.filter((r) => r.kind === 'example'))
  const saved = $derived(rows.filter((r) => r.kind === 'saved'))

  // Reloaded whenever the request changes (a send saves a response), and after each action.
  async function load() {
    rows = (await RequestService.StoredResponses(node)) as unknown as StoredView[]
  }
  $effect(() => {
    void view.response?.at
    void view.sending
    load()
  })

  const fail = (e: unknown) => ui.toast(errorText(e), 'error')

  async function show(r: StoredView) {
    const problem = (await RequestService.ShowStored(node, r.version, r.name)) as string
    if (problem) ui.toast(problem, 'error')
  }

  async function pin(r: StoredView) {
    ui.prompt({
      title: 'Pin as example',
      label: 'Example name (empty: the status)',
      initial: r.status ? String(r.status) : '',
      warning: (await RequestService.PinStoredWarning(node, r.version, r.name)) as string,
      done: async (name) => {
        try {
          ui.toast('Pinned: ' + (await RequestService.PinStored(node, r.version, r.name, name)))
          load()
        } catch (e) {
          fail(e)
        }
      },
    })
  }

  function remove(r: StoredView) {
    const what = r.kind === 'example' ? `The pinned example "${r.name}" is deleted from the collection's files. If the collection is in git, it can be restored from there.` : `The saved response "${r.name}" of version ${r.version} is deleted from the workspace. This cannot be undone.`
    ui.confirm('Delete response', what, async () => {
      const problem = (await RequestService.DeleteStored(node, r.kind, r.version, r.name)) as string
      if (problem) ui.toast(problem, 'error')
      load()
    })
  }

  const size = (n: number) => (n < 1024 ? `${n} B` : n < 1 << 20 ? `${(n / 1024).toFixed(1)} KB` : `${(n / (1 << 20)).toFixed(1)} MB`)
</script>

{#snippet table(list: StoredView[], isExample: boolean)}
  {#each list as r (r.kind + r.version + r.name)}
    <div class="row">
      <span class="name">{r.name}</span>
      <span class="status" class:ok={r.status >= 200 && r.status < 300} class:bad={r.status >= 400}>{r.status || '—'}</span>
      {#if !isExample}<span class="muted">v{r.version}</span>{/if}
      <span class="muted at">{r.at} · {size(r.size)}</span>
      <span class="spacer"></span>
      {#if !isExample}
        <button onclick={() => show(r)} title="Show it in the response pane">Show</button>
        <button onclick={() => pin(r)} title="Keep a copy next to the request, in the collection">Pin…</button>
      {/if}
      <button class="del" onclick={() => remove(r)} title="Delete this response">Delete…</button>
    </div>
  {/each}
{/snippet}

{#if !examples.length && !saved.length}
  <EmptyState icon="clock" title="No responses kept" hint="A send saves its response here (the setting responses.save chooses which). Pin one to keep it with the collection as an example for the replayer." compact />
{:else}
<p class="muted">The replayer answers with the pinned examples first, then with the saved responses. Pinned examples are kept in the collection and committed; saved responses stay in your workspace.</p>

<h4>Pinned examples</h4>
{@render table(examples, true)}
{#if !examples.length}<p class="muted">None. Pin a response from the response pane, or from the saved responses below.</p>{/if}

<h4>Saved responses</h4>
{@render table(saved, false)}
{#if !saved.length}<p class="muted">None yet. A send saves its response here; <code>responses.save</code> in Settings chooses which.</p>{/if}
{/if}

<style>
  .muted { color: var(--muted); font-size: var(--fs-sm); margin: 0 0 6px; }
  h4 { margin: 14px 0 4px; font-size: var(--fs-sm); }
  .row { display: flex; align-items: center; gap: 8px; padding: 4px 0; border-bottom: 1px solid color-mix(in srgb, var(--line) 50%, transparent); font-size: var(--fs-sm); }
  .name { font: var(--fs-sm) var(--mono); min-width: 70px; }
  .status { font: 600 11px ui-monospace, Menlo, monospace; color: var(--muted); }
  .status.ok { color: var(--ok); }
  .status.bad { color: var(--fail); }
  .at { white-space: nowrap; }
  .spacer { flex: 1; }
  button { background: transparent; color: var(--text); border: 1px solid var(--line); border-radius: var(--r-md); padding: 1px 9px; font: inherit; font-size: var(--fs-xs); cursor: pointer; }
  .del:hover { color: var(--fail); border-color: var(--fail); }
</style>
