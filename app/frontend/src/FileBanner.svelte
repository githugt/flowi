<script lang="ts">
  import Banner from './Banner.svelte'
  // The two things a file can tell an editor: it changed on disk under
  // unsaved edits, or saving would drop its comments.
  import { ui } from './lib/ui.svelte'
  import { cell, diffSummary, fieldName, type DiskDiff } from './lib/diskdiff'
  import Button from './Button.svelte'

  let {
    onDisk,
    needsDrop,
    revert,
    saveDropping,
    declineDrop,
    dirty = false,
    keep,
    compare,
  }: {
    onDisk: boolean
    needsDrop: boolean
    revert: () => void
    saveDropping: () => void
    declineDrop: () => void
    dirty?: boolean // the editor holds edits that are not saved
    compare?: () => Promise<DiskDiff> // "Compare": what differs between the file and the edits
    keep?: () => void // "Keep mine": the next save may overwrite the file (a save is otherwise refused while it differs)
  } = $props()

  // "Keep mine" hides the banner until the file changes on disk again.
  let kept = $state(false)
  $effect(() => {
    if (!onDisk) kept = false
  })
  let diff = $state<DiskDiff | null>(null)
  let diffError = $state('')
  async function showDiff() {
    if (diff) return void (diff = null)
    diffError = ''
    try {
      diff = await compare!()
    } catch (e) {
      diffError = e instanceof Error ? e.message : String(e)
    }
  }
  // A new change on disk makes the list stale.
  $effect(() => {
    if (!onDisk) diff = null
  })
  // Before: one button, "Load from disk", threw away unsaved edits with no question.
  const load = () => {
    if (!dirty) return revert()
    ui.confirm('Load the file from disk?', 'Your unsaved edits will be lost.', revert, 'Load from disk')
  }
</script>

{#if onDisk && !kept}
  <Banner tone="warn">
    This file changed on disk{dirty ? ' and you have unsaved edits' : ''}.
    {#snippet actions()}
      {#if compare}<Button small variant="ghost" onclick={showDiff}>{diff ? 'Hide comparison' : 'Compare'}</Button>{/if}
      <Button small onclick={load}>Load from disk</Button>
      <Button small onclick={() => { kept = true; keep?.() }} title="Hide this message. Saving writes your version over the file.">Keep mine</Button>
    {/snippet}
  </Banner>
{/if}
{#if onDisk && !kept && (diff || diffError)}
  <div class="diff" role="region" aria-label="What differs from the file on disk">
    {#if diffError}
      <p class="note">{diffError}</p>
    {:else if diff}
      <p class="note">{diffSummary(diff)} Load from disk takes the file's side and drops yours; Keep mine saves yours over the file.</p>
      {#if diff.changes.length}
        <table>
          <thead><tr><th scope="col">Field</th><th scope="col">On disk</th><th scope="col">Yours</th></tr></thead>
          <tbody>
            {#each diff.changes as c (c.path)}
              <tr><td class="field">{fieldName(c.path)}</td><td class="disk" class:none={c.disk === ''}>{cell(c.disk)}</td><td class="mine" class:none={c.mine === ''}>{cell(c.mine)}</td></tr>
            {/each}
          </tbody>
        </table>
      {/if}
    {/if}
  </div>
{/if}
{#if needsDrop}
  <Banner tone="warn">
    Saving rewrites this file in Flowi's own format, which drops its comments.
    {#snippet actions()}
      <Button small onclick={saveDropping}>Save and drop comments</Button>
      <Button small onclick={declineDrop}>Cancel</Button>
    {/snippet}
  </Banner>
{/if}

<style>


  .diff { padding: 6px 12px 8px; font-size: var(--fs-sm); background: color-mix(in srgb, var(--warn) 8%, transparent); border-bottom: 1px solid var(--line); max-height: 40vh; overflow: auto; }
  .note { margin: 0 0 6px; color: var(--muted); }
  table { border-collapse: collapse; width: 100%; }
  th { text-align: left; font-weight: 600; color: var(--muted); padding: 2px 8px 2px 0; }
  td { padding: 2px 8px 2px 0; vertical-align: top; font-family: var(--mono); font-size: var(--fs-xs); overflow-wrap: anywhere; }
  .disk { color: var(--fail); }
  .mine { color: var(--ok); }
  .none { color: var(--muted); font-style: italic; }
</style>
