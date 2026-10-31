<script lang="ts">
  // Code shown, not edited: a body, a message, a schema or a snippet, coloured like the editor.
  // These were plain text before, so JSON read as one grey block.
  import { guessLang, highlight, type CodeLang } from './lib/highlight'

  let { text, lang }: { text: string; lang?: CodeLang } = $props()

  const segments = $derived(highlight(text, lang ?? guessLang(text)))
</script>

<pre class="code">{#each segments as s}{#if s.cls}<span class={s.cls}>{s.text}</span>{:else}{s.text}{/if}{/each}</pre>

<style>
  .code {
    margin: 0; padding: 8px; background: var(--bg); border: 1px solid var(--line); border-radius: var(--r-md);
    font: 12px/1.45 var(--mono); white-space: pre-wrap; word-break: break-word; overflow: auto;
    -webkit-user-select: text; user-select: text;
  }
</style>
