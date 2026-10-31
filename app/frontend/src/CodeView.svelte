<script lang="ts">
  // The flow as text: the flow body (editable, applied as one undoable edit)
  // and the Arazzo file saving would write (read-only).
  import { FlowService } from '../bindings/github.com/githugt/flowi/app/internal/bridge'
  import type { FlowView, StandaloneView } from './lib/types'
  import { ui } from './lib/ui.svelte'
  import { stepAtLine } from './lib/codeStep'
  import CodeEditor from './CodeEditor.svelte'

  // editable follows the flow editor's Edit: the code could be changed while the canvas could not.
  let { view, editable = true }: { view: FlowView; editable?: boolean } = $props()
  let tab = $state<'flow' | 'file' | 'standalone'>('flow')
  let text = $state('')
  let file = $state('')
  let edited = $state(false)
  let problems = $state<string[]>([])
  // What is wrong with the flow once applied: it is applied all the same, so these are told, not blocking.
  let notes = $state<string[]>([])
  let editor = $state<CodeEditor>()
  // The flow and the flows it calls as one Arazzo document other tools can run; made from the saved flow.
  let standalone = $state<StandaloneView | null>(null)
  let standaloneError = $state('')

  async function showStandalone() {
    tab = 'standalone'
    standaloneError = ''
    try {
      standalone = (await FlowService.StandaloneArazzo(view.node)) as unknown as StandaloneView
    } catch (e) {
      standaloneError = String(e)
    }
  }
  async function saveStandalone() {
    try {
      const file = await FlowService.SaveStandaloneArazzo(view.node)
      if (file) ui.toast(`Saved ${file}.`)
    } catch (e) {
      standaloneError = String(e)
    }
  }

  // The text follows the document until the user types.
  $effect(() => {
    void view.rev
    if (edited) return
    FlowService.Source(view.node).then((s) => {
      text = s.flow
      file = s.file
    })
  })

  /** Applies typed text. True when there is nothing left to apply. */
  export async function settle(): Promise<boolean> {
    if (!edited) return true
    return apply()
  }

  async function apply(): Promise<boolean> {
    notes = []
    try {
      const applied = await FlowService.ApplySource(view.node, text)
      problems = applied.problems
      notes = applied.notes
    } catch (e) {
      problems = [String(e)]
    }
    if (problems.length) return false
    edited = false
    return true
  }

  async function discard() {
    edited = false
    problems = []
    notes = []
    const s = await FlowService.Source(view.node)
    text = s.flow
  }

  function jump(p: string) {
    const m = /^line (\d+)/.exec(p)
    if (m) editor?.goToLine(Number(m[1]))
  }

  /** A note about a step: the cursor goes to the step in the text. */
  function jumpToStep(note: string) {
    const m = /^step (\S+): /.exec(note)
    if (!m) return
    const at = text.split('\n').findIndex((l) => new RegExp(`^\\s*(- )?id:\\s*["']?${m[1].replace(/[.*+?^${}()|[\]\\]/g, '\\$&')}["']?\\s*$`).test(l))
    if (at >= 0) editor?.goToLine(at + 1)
  }

  // The caret in the text selects the step on the canvas.
  function followCursor(line: number) {
    const id = stepAtLine(text, line)
    if (id && id !== view.selected) FlowService.Select(view.node, id)
  }
</script>

<div class="code">
  <div class="bar">
    <div class="tabs">
      <button class:active={tab === 'flow'} onclick={() => (tab = 'flow')}>Flow</button>
      <button class:active={tab === 'file'} onclick={() => (tab = 'file')}>Arazzo file</button>
      <button class:active={tab === 'standalone'} onclick={showStandalone} title="The flow and the flows it calls as one Arazzo document for other tools">Standalone Arazzo</button>
    </div>
    <span class="hint">
      {#if tab === 'file'}What saving writes. Read-only.{:else if tab === 'standalone'}For other tools: standard steps only, the saved flow and the flows it calls. Read-only.{:else if editable}The flow itself. Apply to change the flow (one undo step).{:else}The flow itself. Read-only while viewing: press Edit to change it.{/if}
    </span>
    <span class="spacer"></span>
    {#if tab === 'standalone' && standalone}
      <button onclick={saveStandalone}>Save as…</button>
    {/if}
    {#if tab === 'flow' && editable}
      <button disabled={!edited} onclick={discard}>Discard</button>
      <button class="primary" disabled={!edited} onclick={apply}>Apply</button>
    {/if}
  </div>
  {#if problems.length}
    <ul class="problems">
      {#each problems as p}<li><button onclick={() => jump(p)}>{p}</button></li>{/each}
    </ul>
  {/if}
  {#if notes.length}
    <ul class="notes">
      {#each notes as n}<li><button onclick={() => jumpToStep(n)}>{n}</button></li>{/each}
    </ul>
  {/if}
  <div class="editor">
    {#if tab === 'flow'}
      <CodeEditor
        bind:this={editor}
        value={text}
        readonly={!editable}
        oncursor={followCursor}
        onchange={(t) => {
          if (t !== text) {
            text = t
            edited = true
          }
        }}
      />
    {:else if tab === 'standalone'}
      {#if standaloneError}
        <p class="err">{standaloneError}</p>
      {:else if standalone}
        {#if standalone.unsaved}<p class="warnline">This is the saved flow: your unsaved edits are not in it.</p>{/if}
        {#if standalone.notes.length}
          <details class="notes-line"><summary>{standalone.notes.length} without an Arazzo equivalent</summary>
            <ul>{#each standalone.notes as n}<li>{n}</li>{/each}</ul>
          </details>
        {/if}
        <p class="muted">Copy <code>descriptions/requests.openapi.yaml</code> (and <code>requests.asyncapi.yaml</code> when the flow has message steps) beside this file, as <code>./requests.openapi.yaml</code>, for the operations to resolve.</p>
        <div class="grow"><CodeEditor value={standalone.text} readonly /></div>
      {/if}
    {:else}
      <CodeEditor value={file} readonly />
    {/if}
  </div>
</div>

<style>
  .code { position: absolute; inset: 0; display: flex; flex-direction: column; }
  .bar { display: flex; align-items: center; gap: 8px; padding: 6px 12px; border-bottom: 1px solid var(--line); flex: none; }
  .tabs { display: flex; gap: 2px; }
  .hint { color: var(--muted); font-size: var(--fs-sm); }
  .spacer { flex: 1; }
  button {
    background: transparent;
    color: var(--text);
    border: 1px solid var(--line);
    border-radius: var(--r-md);
    padding: 3px 12px;
    font: inherit;
  }
  button:hover:not(:disabled) { border-color: var(--accent); }
  button:disabled { opacity: 0.4; }
  .tabs .active { border-color: var(--accent); }
  .primary:not(:disabled) { background: var(--accent); color: var(--on-accent); border-color: var(--accent); }
  .problems { margin: 0; padding: 6px 12px; list-style: none; background: color-mix(in srgb, var(--fail) 12%, transparent); flex: none; max-height: 140px; overflow: auto; }
  .problems button { background: none; border: 0; color: var(--fail); padding: 1px 0; text-align: left; font: var(--fs-sm) var(--mono); }
  .notes { margin: 0; padding: 6px 12px; list-style: none; background: color-mix(in srgb, var(--warn) 14%, transparent); flex: none; max-height: 120px; overflow: auto; }
  .notes button { background: none; border: 0; color: var(--warn); padding: 1px 0; text-align: left; font: var(--fs-sm) var(--mono); }
  .editor { flex: 1; min-height: 0; display: flex; flex-direction: column; }
  .editor > :global(.host) { flex: 1; }
  .grow { flex: 1; min-height: 0; }
  .warnline, .muted, .err, .notes-line { margin: 0; padding: 4px 12px; font-size: var(--fs-sm); }
  .warnline { color: var(--warn); }
  .muted { color: var(--muted); }
  .err { color: var(--fail); }
  .notes-line ul { margin: 4px 0 0; padding-left: 18px; color: var(--muted); }
</style>
