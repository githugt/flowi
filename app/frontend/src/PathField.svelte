<script lang="ts">
  // A path typed in, or chosen with the system's dialog when there is one.
  import { SystemService } from '../bindings/github.com/githugt/flowi/app/internal/bridge'

  let {
    value = $bindable(''),
    label = '',
    title = 'Choose',
    kind = 'folder',
  }: { value?: string; label?: string; title?: string; kind?: 'folder' | 'file' } = $props()

  async function browse() {
    try {
      const picked = kind === 'folder' ? await SystemService.PickFolder(title) : await SystemService.PickFile(title)
      if (picked) value = picked
    } catch {
      // no dialogs in this build: the path is typed
    }
  }
</script>

<label class="f">
  {#if label}<span>{label}</span>{/if}
  <div>
    <input bind:value spellcheck="false" placeholder={kind === 'folder' ? '/path/to/folder' : '/path/to/file'} />
    <button type="button" onclick={browse}>Browse…</button>
  </div>
</label>

<style>
  .f { display: flex; flex-direction: column; gap: 3px; color: var(--muted); font-size: var(--fs-xs); }
  div { display: flex; gap: 6px; }
  input { flex: 1; min-width: 0; background: var(--bg); color: var(--text); border: 1px solid var(--line); border-radius: var(--r-md); padding: 5px 8px; font: var(--fs-sm) var(--mono); -webkit-user-select: text; user-select: text; }
  input:focus { border-color: var(--accent); }
  button { background: transparent; color: var(--text); border: 1px solid var(--line); border-radius: var(--r-md); padding: 3px 12px; font: inherit; }
</style>
