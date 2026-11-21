<script lang="ts">
  import Modal from './Modal.svelte'
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

<Modal title={`Version ${label}`} width={440} scroll onclose={oncancel} onsubmit={() => onsave(title, notes)}>
    <label><span>Title (optional, shown beside the label)</span>
      <input bind:this={first} bind:value={title} placeholder="Before the tax change" spellcheck="false" />
    </label>
    <label><span>Description (Markdown; the same text as the Docs section)</span>
      <textarea bind:value={notes} rows="6" spellcheck="true"></textarea>
    </label>
  {#snippet actions()}
    <Button type="button" onclick={oncancel}>Cancel</Button>
    <Button variant="primary" type="submit">Save</Button>
  {/snippet}
</Modal>

<style>
  label { display: flex; flex-direction: column; gap: 3px; color: var(--muted); font-size: var(--fs-xs); }
  textarea { resize: vertical; }
</style>
