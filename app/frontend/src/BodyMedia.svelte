<script lang="ts">
  // A response body that is not text to read: a picture drawn as an image, or a card that says what it is and offers to
  // save it, with the first bytes as a hex dump. A page has its own sandboxed preview.
  import Button from './Button.svelte'
  import type { ResponseView } from './lib/types'

  let { r, onsave }: { r: ResponseView; onsave: () => void } = $props()
</script>

{#if r.bodyKind === 'image' && r.bodyImage}
  <img class="pic" src={r.bodyImage} alt="The response body, an image ({r.contentType})" />
{:else}
  <div class="card">
    <strong>{r.size || 'The body'}{r.contentType ? ' · ' + r.contentType : ''}</strong>
    <span class="muted">{r.bodyKind === 'image' ? 'Too large to show here, or the type is not one the window draws.' : 'This is not text, so it is not shown as text.'}</span>
    <span class="save"><Button small onclick={onsave}>Save body…</Button></span>
  </div>
  {#if r.bodyHex}
    <details open>
      <summary>First bytes</summary>
      <pre class="hex" aria-label="Hex dump of the first bytes of the body">{r.bodyHex}</pre>
    </details>
  {/if}
{/if}

<style>
  .pic { max-width: 100%; max-height: 100%; object-fit: contain; background: var(--raised); border-radius: var(--r-md); }
  .card { display: flex; flex-direction: column; align-items: flex-start; gap: var(--sp-2); padding: var(--sp-3); border: 1px solid var(--line); border-radius: var(--r-md); background: var(--raised); }
  .muted { color: var(--muted); font-size: var(--fs-sm); }
  .hex { margin: var(--sp-2) 0 0; font: var(--fs-xs) / 1.5 var(--mono); white-space: pre; overflow: auto; }
  summary { color: var(--muted); font-size: var(--fs-sm); }
</style>
