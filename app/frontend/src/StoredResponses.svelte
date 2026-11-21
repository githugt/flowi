<script lang="ts">
  import Button from './Button.svelte'
  import EmptyState from './EmptyState.svelte'
  // What is kept for a request: the responses it saved (in the workspace, for
  // the replayer and for showing again) and the examples pinned next to it (in
  // the collection, committed). One at a time can be shown, pinned and deleted.
  import { errorText } from './lib/errors'
  import { statusTone } from './lib/statustone'
  import { RequestService } from '../bindings/github.com/githugt/flowi/app/internal/bridge'
  import { ui } from './lib/ui.svelte'
  import { replaceExampleText } from './lib/pinexample'
  import type { RequestView, StoredView } from './lib/types'
  import { sizeText } from './lib/bytesize'

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
        const go = async () => {
          try {
            ui.toast('Pinned: ' + (await RequestService.PinStored(node, r.version, r.name, name)))
            load()
          } catch (e) {
            fail(e)
          }
        }
        try {
          if (!(await RequestService.ExampleTaken(node, r.version, r.name, name))) return go()
        } catch (e) {
          return fail(e)
        }
        const d = replaceExampleText(name, r.status)
        ui.confirm(d.title, d.text, go, d.action)
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

  const size = sizeText
</script>

{#snippet table(list: StoredView[], isExample: boolean)}
  {#each list as r (r.kind + r.version + r.name)}
    <div class="row">
      <span class="name">{r.name}</span>
      <span class="status {statusTone(r.status)}">{r.status || '—'}</span>
      {#if !isExample}<span class="muted">v{r.version}</span>{/if}
      <span class="muted at">{r.at} · {size(r.size)}</span>
      <span class="spacer"></span>
      {#if !isExample}
        <Button small onclick={() => show(r)} title="Show it in the response pane">Show</Button>
        <Button small onclick={() => pin(r)} title="Keep a copy next to the request, in the collection">Pin…</Button>
      {/if}
      <Button small danger onclick={() => remove(r)} title="Delete this response">Delete…</Button>
    </div>
  {/each}
{/snippet}

{#if !examples.length && !saved.length}
  <EmptyState icon="clock" title="No responses kept" hint="A send saves its response here (the setting responses.save chooses which). Pin one to keep it with the collection as an example for the replayer." compact />
{:else}
<p class="muted">The replayer answers with the pinned examples first, then with the saved responses. Pinned examples are kept in the collection and committed; saved responses stay in your workspace.</p>

<h4>Pinned examples</h4>
{@render table(examples, true)}
{#if !examples.length}<EmptyState icon="clock" title="No pinned examples" hint="Pin a response from the response pane, or from the saved responses below." compact />{/if}

<h4>Saved responses</h4>
{@render table(saved, false)}
{#if !saved.length}<EmptyState icon="clock" title="No saved responses yet" hint="A send saves its response here; the setting responses.save chooses which." compact />{/if}
{/if}

<style>
  .muted { color: var(--muted); font-size: var(--fs-sm); margin: 0 0 6px; }
  h4 { margin: 14px 0 4px; font-size: var(--fs-sm); }
  .row { display: flex; align-items: center; gap: 8px; padding: 4px 0; border-bottom: 1px solid color-mix(in srgb, var(--line) 50%, transparent); font-size: var(--fs-sm); }
  .name { font: var(--fs-sm) var(--mono); min-width: 70px; }
  .status { font: 600 var(--fs-xs) ui-monospace, Menlo, monospace; color: var(--muted); }
  .status.ok { color: var(--ok); }
  .status.warn { color: var(--warn); }
  .status.bad { color: var(--fail); }
  .at { white-space: nowrap; }
  .spacer { flex: 1; }
</style>
