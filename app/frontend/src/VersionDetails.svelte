<script lang="ts">
  import Button from './Button.svelte'
  // The title and the description of one version of a request. The title shows
  // beside the label in the version picker; the description is the version's Docs.
  let { label, name, docs, onsave, oncancel }: { label: string; name: string; docs: string; onsave: (name: string, docs: string) => void; oncancel: () => void } = $props()

  let title = $state('')
  let notes = $state('')
  let first = $state<HTMLInputElement>()
  $effect(() => {
    title = name
    notes = docs
    queueMicrotask(() => first?.focus())
  })
</script>

<div class="scrim" role="presentation" onpointerdown={oncancel}>
  <form class="dialog" onpointerdown={(e) => e.stopPropagation()} onsubmit={(e) => { e.preventDefault(); onsave(title, notes) }}>
    <h3>Version {label}</h3>
    <label><span>Title (optional, shown beside the label)</span>
      <input bind:this={first} bind:value={title} placeholder="Before the tax change" spellcheck="false" />
    </label>
    <label><span>Description (Markdown; the same text as the Docs section)</span>
      <textarea bind:value={notes} rows="6" spellcheck="true"></textarea>
    </label>
    <div class="actions">
      <Button type="button" onclick={oncancel}>Cancel</Button>
      <Button variant="primary" type="submit">Save</Button>
    </div>
  </form>
</div>

<style>
  .scrim { position: fixed; inset: 0; background: rgb(0 0 0 / 0.4); display: grid; place-items: center; z-index: 60; }
  .dialog { width: 440px; max-height: 80vh; overflow: auto; display: flex; flex-direction: column; gap: 10px; padding: 16px; background: var(--panel); border: 1px solid var(--line); border-radius: var(--r-lg); }
  .dialog h3 { margin: 0; }
  label { display: flex; flex-direction: column; gap: 3px; color: var(--muted); font-size: var(--fs-xs); }
  input, textarea { background: var(--panel); color: var(--text); border: 1px solid var(--line); border-radius: var(--r-md); padding: 5px 8px; font: inherit; -webkit-user-select: text; user-select: text; }
  textarea { resize: vertical; }
  .actions { display: flex; justify-content: flex-end; gap: 8px; }
</style>
