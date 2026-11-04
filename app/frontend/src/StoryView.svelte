<script lang="ts">
  import Button from './Button.svelte'
  // The storyboard: the flow told in sentences, one per step, branches and loops indented. After a run each row
  // carries how its step ended; a row selects its step for the other views. Problems (a variable nothing reads,
  // one nothing sets) are listed below and marked on the step's row.
  import { Clipboard } from '@wailsio/runtime'
  import { FlowService } from '../bindings/github.com/githugt/flowi/app/internal/bridge'
  import { ui } from './lib/ui.svelte'
  import { problemsOf, resultTone, storyText } from './lib/story'
  import type { ExplainView, FlowView } from './lib/types'

  let { view }: { view: FlowView } = $props()

  let story = $state<ExplainView>({ sentences: [], problems: [] })
  const node = $derived(view.node)

  // Told again whenever the flow changes: the revision counts every edit.
  $effect(() => {
    void view.rev
    FlowService.Explain(node).then((e) => (story = e as unknown as ExplainView))
  })

  function copy() {
    Clipboard.SetText(storyText(story))
    ui.toast('Copied')
  }
</script>

<div class="story">
  <div class="bar">
    <span class="muted">The flow in words. Click a line to select its step.</span>
    <Button small disabled={!story.sentences.length} onclick={copy}>Copy as text</Button>
  </div>
  {#if story.sentences.length}
    <ol class="lines" aria-label="Steps in words">
      {#each story.sentences as s, i (i)}
        {@const result = s.node ? view.run.results[s.node] : undefined}
        {@const issues = s.node ? problemsOf(story, s.node) : []}
        <li style:padding-left={`${s.indent * 20}px`}>
          {#if s.node}
            <button type="button" class="row" class:on={view.selected === s.node} onclick={() => FlowService.Select(node, s.node)}>
              <span class="text">{s.text}</span>
              {#if issues.length}<span class="flag" title={issues.join('\n')}>!</span>{/if}
              {#if result}<span class="result {resultTone(result)}">{result}</span>{/if}
            </button>
          {:else}
            <span class="group">{s.text}</span>
          {/if}
        </li>
      {/each}
    </ol>
  {:else}
    <p class="muted empty">This flow has no steps yet.</p>
  {/if}
  {#if story.problems.length}
    <section class="problems" aria-label="Problems">
      <h4>Worth a look</h4>
      <ul>
        {#each story.problems as p, i (i)}
          <li><button type="button" class="link" onclick={() => FlowService.Select(node, p.node)}>{p.text}</button> <span class="muted">(step {p.node})</span></li>
        {/each}
      </ul>
    </section>
  {/if}
</div>

<style>
  .story { display: flex; flex-direction: column; gap: 10px; padding: 14px 18px; max-width: 760px; margin: 0 auto; width: 100%; overflow: auto; }
  .bar { display: flex; align-items: center; justify-content: space-between; gap: 8px; }
  .muted { color: var(--muted); font-size: var(--fs-sm); }
  .empty { margin: 0; }
  .lines { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: 2px; }
  .row { width: 100%; display: flex; align-items: baseline; gap: 8px; text-align: left; background: none; border: 0; border-radius: var(--r-sm); color: var(--text); font: inherit; padding: 5px 8px; }
  .row:hover { background: var(--tint-hover); }
  .row.on { background: color-mix(in srgb, var(--accent) 22%, transparent); }
  .text { flex: 1; }
  .group { display: block; padding: 5px 8px; color: var(--muted); }
  .flag { color: var(--warn, var(--accent)); font-weight: 700; }
  .result { font: var(--fs-xs) var(--mono); color: var(--muted); white-space: nowrap; }
  .result.pass { color: var(--ok, var(--muted)); }
  .result.fail { color: var(--err, var(--muted)); font-weight: 600; }
  .problems { border-top: 1px solid var(--line); padding-top: 8px; }
  .problems h4 { margin: 0 0 4px; font-size: var(--fs-sm); }
  .problems ul { margin: 0; padding-left: 18px; display: flex; flex-direction: column; gap: 2px; }
  .link { background: none; border: 0; padding: 0; color: var(--accent); font: inherit; text-align: left; }
  .link:hover { text-decoration: underline; }
</style>
