<script lang="ts">
  import Modal from './Modal.svelte'
  import Button from './Button.svelte'
  // The chosen flowi overlay does not fit this version (or its base has changed since it was saved):
  // each part that was left out, with the reason. The rest of the overlay is applied.
  let { overlay, notes, version, onclose }: { overlay: string; notes: string[]; version: string; onclose: () => void } = $props()
</script>

<Modal title={`“${overlay}” does not fit version ${version} in full`} label="The overlay does not fit" width={480} scroll alert layer="top" {onclose}>
    <p class="muted">What fits is applied. These parts were left out, or applied although the version has changed since the overlay was saved:</p>
    <ul>{#each notes as n}<li>{n}</li>{/each}</ul>
  {#snippet actions()}<Button variant="primary" onclick={onclose}>OK</Button>{/snippet}
</Modal>

<style>
  p { margin: 0; }
  .muted { color: var(--muted); font-size: var(--fs-sm); }
  ul { margin: 0; padding-left: 18px; display: flex; flex-direction: column; gap: 4px; font-size: var(--fs-sm); }
</style>
