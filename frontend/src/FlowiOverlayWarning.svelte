<script lang="ts">
  // The chosen flowi overlay does not fit this version (or its base has changed since it was saved):
  // each part that was left out, with the reason. The rest of the overlay is applied.
  let { overlay, notes, version, onclose }: { overlay: string; notes: string[]; version: string; onclose: () => void } = $props()
</script>

<div class="scrim" role="presentation" onpointerdown={onclose}>
  <div class="dialog" role="alertdialog" aria-label="The overlay does not fit" tabindex="-1" onpointerdown={(e) => e.stopPropagation()}>
    <h3>“{overlay}” does not fit version {version} in full</h3>
    <p class="muted">What fits is applied. These parts were left out, or applied although the version has changed since the overlay was saved:</p>
    <ul>{#each notes as n}<li>{n}</li>{/each}</ul>
    <div class="actions"><button class="primary" onclick={onclose}>OK</button></div>
  </div>
</div>

<style>
  .scrim { position: fixed; inset: 0; background: rgb(0 0 0 / 0.4); display: grid; place-items: center; z-index: 70; }
  .dialog { width: 480px; max-width: calc(100vw - 32px); max-height: 80vh; overflow: auto; display: flex; flex-direction: column; gap: 10px; padding: 16px; background: var(--panel); border: 1px solid var(--line); border-radius: var(--r-lg); }
  h3, p { margin: 0; }
  h3 { font-size: var(--fs-md, 1em); }
  .muted { color: var(--muted); font-size: var(--fs-sm); }
  ul { margin: 0; padding-left: 18px; display: flex; flex-direction: column; gap: 4px; font-size: var(--fs-sm); }
  .actions { display: flex; justify-content: flex-end; }
  button { background: var(--accent); color: var(--on-accent); border: 1px solid var(--accent); border-radius: var(--r-md); padding: 3px 16px; font: inherit; }
</style>
