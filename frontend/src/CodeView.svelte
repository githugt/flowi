<script lang="ts">
  // The flow as text: the flow body (editable, applied as one undoable edit)
  // and the Arazzo file saving would write (read-only).
  import { FlowService } from '../bindings/github.com/githugt/flowi/internal/bridge'
  import type { FlowView } from './lib/types'
  import CodeEditor from './CodeEditor.svelte'

  // editable follows the flow editor's Edit: the code could be changed while the canvas could not.
  let { view, editable = true }: { view: FlowView; editable?: boolean } = $props()
  let tab = $state<'flow' | 'file'>('flow')
  let text = $state('')
  let file = $state('')
  let edited = $state(false)
  let problems = $state<string[]>([])
  let editor = $state<CodeEditor>()

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
    try {
      problems = await FlowService.ApplySource(view.node, text)
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
    const s = await FlowService.Source(view.node)
    text = s.flow
  }

  function jump(p: string) {
    const m = /^line (\d+)/.exec(p)
    if (m) editor?.goToLine(Number(m[1]))
  }
</script>

<div class="code">
  <div class="bar">
    <div class="tabs">
      <button class:active={tab === 'flow'} onclick={() => (tab = 'flow')}>Flow</button>
      <button class:active={tab === 'file'} onclick={() => (tab = 'file')}>Arazzo file</button>
    </div>
    <span class="hint">
      {#if tab === 'file'}What saving writes. Read-only.{:else if editable}The flow itself. Apply to change the flow (one undo step).{:else}The flow itself. Read-only while viewing: press Edit to change it.{/if}
    </span>
    <span class="spacer"></span>
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
  <div class="editor">
    {#if tab === 'flow'}
      <CodeEditor
        bind:this={editor}
        value={text}
        readonly={!editable}
        onchange={(t) => {
          if (t !== text) {
            text = t
            edited = true
          }
        }}
      />
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
  .editor { flex: 1; min-height: 0; }
</style>
