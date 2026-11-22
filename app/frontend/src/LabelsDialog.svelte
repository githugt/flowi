<script lang="ts">
  import { app } from './lib/app.svelte'
  import { ui } from './lib/ui.svelte'
  import Modal from './Modal.svelte'
  import Button from './Button.svelte'
  import LabelsForm from './LabelsForm.svelte'

  // Tags and status…, from the right-click menu of a request, a flow or a folder: the same editor as the Docs section
  // of a request and the flow panel, for a row that is not open (and the only place a folder's are edited).
  const node = $derived(ui.labelling)
  const row = $derived(node ? app.find(node) : undefined)
  const close = () => (ui.labelling = null)
</script>

{#if node}
  <Modal title={`Tags and status${row ? `: ${row.label}` : ''}`} width={460} onclose={close}>
    <LabelsForm {node} />
    <p class="note">Saved as you go, in the {row?.kind === 'folder' ? '_folder.yaml of the folder, and applies to everything in it that sets none of its own' : 'file of the ' + (row?.kind ?? 'item')}.</p>
    {#snippet actions()}
      <Button variant="primary" onclick={close}>Done</Button>
    {/snippet}
  </Modal>
{/if}

<style>
  .note { margin: var(--sp-3) 0 0; color: var(--muted); font-size: var(--fs-xs); }
</style>
