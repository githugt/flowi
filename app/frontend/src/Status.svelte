<script lang="ts">
  import type { Snippet } from 'svelte'

  // How something is going, as a dot and words: a response's code, a server's state, a secret's. The dot is the shape
  // that goes with the colour, so a status never rests on hue alone. A pill (the default) has a tinted background;
  // plain is the dot and the words in the surrounding text.
  // Before: the response pane and the traffic log used a global pill class, and three screens drew their own dot and
  // coloured word.
  let {
    tone = '',
    plain = false,
    block = false,
    children,
  }: {
    tone?: '' | 'ok' | 'warn' | 'bad' | 'info'
    plain?: boolean
    /** Fills the width it is given and centres the words, for a column of codes. */
    block?: boolean
    children: Snippet
  } = $props()
</script>

<span class="status {tone}" class:plain class:block>{@render children()}</span>

<style>
  .status { display: inline-flex; align-items: center; gap: 5px; padding: 0 9px 0 7px; border-radius: var(--r-pill); font: 600 var(--fs-xs)/20px var(--font, system-ui); background: color-mix(in srgb, var(--muted) 20%, transparent); color: var(--text); }
  .status::before { content: ''; flex: none; width: 6px; height: 6px; border-radius: 50%; background: var(--muted); }
  .ok { background: var(--ok-t); color: var(--ok); }
  .warn { background: var(--warn-t); color: var(--warn); }
  .bad { background: var(--fail-t); color: var(--fail); }
  .info { background: var(--info-t); color: var(--info); }
  .ok::before, .warn::before, .bad::before, .info::before { background: currentColor; }
  .plain { padding: 0; background: none; font-weight: inherit; font-size: inherit; line-height: inherit; color: inherit; }
  .plain::before { width: 8px; height: 8px; }
  .block { display: flex; width: 100%; box-sizing: border-box; justify-content: center; }
</style>
