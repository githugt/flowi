<script lang="ts">
  import { Browser } from '@wailsio/runtime'
  import { renderMarkdown } from './lib/markdown'

  let { text }: { text: string } = $props()

  // Links open in the system browser, never inside the window.
  function onclick(e: MouseEvent) {
    const a = (e.target as HTMLElement).closest('a[data-href]')
    if (a) {
      e.preventDefault()
      Browser.OpenURL(a.getAttribute('data-href')!)
    }
  }
</script>

<!-- renderMarkdown escapes the source; see lib/markdown.ts -->
<!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions -->
<div class="md" {onclick}>{@html renderMarkdown(text)}</div>

<style>
  .md { line-height: 1.5; -webkit-user-select: text; user-select: text; }
  .md :global(h2), .md :global(h3), .md :global(h4), .md :global(h5) { margin: 14px 0 4px; font-size: var(--fs-md); }
  .md :global(p) { margin: 4px 0; }
  .md :global(ul) { margin: 4px 0; padding-left: 20px; }
  .md :global(code) { font: var(--fs-sm) var(--mono); background: var(--bg); padding: 0 4px; border-radius: var(--r-sm); }
  .md :global(pre) { background: var(--bg); border: 1px solid var(--line); border-radius: var(--r-md); padding: 8px; overflow: auto; font: var(--fs-sm) var(--mono); }
  .md :global(a) { color: var(--accent); }
</style>
