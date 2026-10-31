<script lang="ts">
  // A request's own hooks: pre and post steps, and for message requests a step run for each message, each a small flow canvas.
  import { HooksService } from '../bindings/github.com/githugt/flowi/internal/bridge'
  import { app } from './lib/app.svelte'
  import { ui } from './lib/ui.svelte'
  import type { HooksView } from './lib/types'
  import FlowEditor from './FlowEditor.svelte'

  let { view }: { view: HooksView } = $props()
  let problem = $state('')
  let blueprint = $state('')

  async function save(dropping = false) {
    problem = ''
    const r = (dropping ? await HooksService.SaveDroppingComments(view.request) : await HooksService.Save(view.request)) as string
    if (r === 'drops-comments') {
      ui.confirm('Save and drop comments', 'Saving rewrites the request file in Flowi’s own format, which drops its comments.', () => save(true))
    } else {
      problem = r
    }
  }

  async function exit() {
    if (view.dirty) {
      ui.confirm('Discard hook edits', 'Close the hook editor and lose the edits you have not saved?', () => app.close('hooks:' + view.request))
    } else {
      app.close('hooks:' + view.request)
    }
  }

  function onkeydown(e: KeyboardEvent) {
    if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 's') {
      e.preventDefault()
      save()
    }
  }
</script>

<svelte:window {onkeydown} />

<div class="hooks">
  <header>
    <strong>Hooks</strong>
    {#if view.dirty}<span class="dirty">unsaved</span>{/if}
    <span class="spacer"></span>
    {#if view.blueprints.length}
      <select bind:value={blueprint} onchange={() => { if (blueprint) HooksService.AddBlueprint(view.request, blueprint); blueprint = '' }}>
        <option value="">Add from a blueprint…</option>
        {#each view.blueprints as b}<option value={b.id}>{b.label}</option>{/each}
      </select>
    {/if}
    <button onclick={exit}>Exit</button>
    <button class="primary" disabled={!view.dirty} onclick={() => save()}>Save</button>
  </header>
  {#if problem}<p class="err">{problem}</p>{/if}
  <div class="phases">
    {#each [[view.pre, 'Before sending', view.othersPre], [view.post, 'After the response', view.othersPost], ...(view.showMessage ? [[view.message, 'For each message received', view.othersMessage]] : [])] as [row, title, others]}
      <section>
        <h4>{title}{#if others}<span class="muted"> · {others} more step(s) from other levels run too</span>{/if}</h4>
        <div class="canvas">{#if app.flows[row as string]}<FlowEditor view={app.flows[row as string]} embedded />{/if}</div>
      </section>
    {/each}
  </div>
</div>

<style>
  .hooks { position: absolute; inset: 0; display: flex; flex-direction: column; }
  header { height: 40px; flex: none; display: flex; align-items: center; gap: 8px; padding: 0 12px; border-bottom: 1px solid var(--line); }
  .dirty { color: var(--accent); font-size: var(--fs-xs); }
  .spacer { flex: 1; }
  .err { margin: 0; padding: 6px 12px; color: var(--fail); }
  .phases { flex: 1; min-height: 0; display: grid; grid-auto-rows: 1fr; }
  section { display: flex; flex-direction: column; min-height: 0; border-bottom: 1px solid var(--line); }
  h4 { margin: 0; padding: 6px 12px; font-size: var(--fs-sm); }
  .muted { color: var(--muted); font-weight: 400; }
  .canvas { flex: 1; min-height: 0; position: relative; }
  button, select { background: transparent; color: var(--text); border: 1px solid var(--line); border-radius: var(--r-md); padding: 3px 12px; font: inherit; }
  select { background: var(--bg); }
  .primary:not(:disabled) { background: var(--accent); color: var(--on-accent); border-color: var(--accent); }
  button:disabled { opacity: 0.4; }
</style>
