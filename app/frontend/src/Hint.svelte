<script lang="ts">
  // A "?" beside a heading or label: the explanation shows on hover or keyboard focus,
  // so forms stay short and the words are there for whoever wants them.
  import type { Snippet } from 'svelte'
  import HelpButton from './HelpButton.svelte'
  import Tip from './Tip.svelte'

  let { children }: { children: Snippet } = $props()
</script>

<span class="hint">
  <Tip>
    {#snippet content()}{@render children()}{/snippet}
    <HelpButton label="Explain" />
  </Tip>
</span>

<style>
  .hint { display: inline-flex; vertical-align: middle; margin-left: var(--sp-1); font-weight: 400; text-transform: none; letter-spacing: 0; }
</style>
