<script lang="ts">
  import type { Snippet } from 'svelte'
  import type { HTMLButtonAttributes } from 'svelte/elements'

  // An on/off switch. Its position says the state, not only its colour. With children it shows them beside the
  // track; `framed` puts both in a raised box like a button (a state that is also a control, such as "Running").
  type Props = Omit<HTMLButtonAttributes, 'onchange'> & { checked: boolean; onchange: (next: boolean) => void; framed?: boolean; tone?: 'accent' | 'ok'; children?: Snippet }
  let { checked, onchange, framed = false, tone = 'accent', children, ...rest }: Props = $props()
</script>

<button type="button" role="switch" aria-checked={checked} class="toggle {tone}" class:on={checked} class:framed {...rest} onclick={() => onchange(!checked)}>
  <span class="track" aria-hidden="true"><span class="knob"></span></span>{#if children}{@render children()}{/if}
</button>

<style>
  .toggle { display: inline-flex; align-items: center; gap: 8px; background: none; border: 0; padding: 0; font: inherit; color: var(--muted); }
  .toggle.on { color: var(--text); }
  .toggle.ok.on { color: var(--ok); }
  .toggle:disabled { opacity: 0.4; }
  .framed { padding: 0 12px 0 8px; min-height: var(--ctl-h); font-weight: 500; background: var(--raised); border-radius: var(--r-md); box-shadow: inset 0 0 0 1px var(--line); }
  .framed:hover:not(:disabled) { background: var(--raised-3); }
  .track { flex: none; width: 28px; height: 16px; border-radius: 8px; background: var(--line); display: flex; align-items: center; padding: 2px; box-sizing: border-box; transition: background var(--t-fast) var(--ease-out); }
  .on .track { background: var(--accent); }
  .ok.on .track { background: var(--ok); }
  .knob { width: 12px; height: 12px; border-radius: 50%; background: var(--on-accent); transition: transform var(--t-fast) var(--ease-out); }
  .on .knob { transform: translateX(12px); }
  .toggle:focus-visible .track { outline: 2px solid var(--accent); outline-offset: 2px; }
</style>
