<script lang="ts">
  import type { Snippet } from 'svelte'
  import type { HTMLButtonAttributes } from 'svelte/elements'

  // One button for every screen. secondary is the everyday one; primary is the main action of a page; go is the
  // Send-coloured "start something" action; ghost has no outline until hovered; danger is for what removes things.
  type Props = HTMLButtonAttributes & { variant?: 'secondary' | 'primary' | 'go' | 'ghost' | 'danger'; small?: boolean; children: Snippet }
  let { variant = 'secondary', small = false, children, ...rest }: Props = $props()
</script>

<button class={variant} class:small {...rest}>{@render children()}</button>

<style>
  button { background: var(--raised); color: var(--text); border: 0; box-shadow: inset 0 0 0 1px var(--line); border-radius: var(--r-md); padding: 0 var(--sp-3); min-height: var(--ctl-h); font: inherit; font-weight: 500; white-space: nowrap; }
  button:hover:not(:disabled) { background: var(--raised-3); }
  button:disabled { opacity: 0.4; }
  .small { min-height: var(--ctl-h-sm); padding: 0 10px; }
  .primary:not(:disabled) { background: var(--primary-fill); color: var(--on-primary); font-weight: 600; box-shadow: inset 0 1px 0 rgb(255 255 255 / 0.18); }
  .primary:not(:disabled):hover { background: var(--primary-fill); filter: brightness(1.1); }
  .go:not(:disabled) { background: var(--send-fill); color: var(--on-send); font-weight: 600; box-shadow: var(--send-edge); }
  .go:not(:disabled):hover { background: var(--send-fill); filter: brightness(1.08); }
  .ghost { background: none; box-shadow: none; color: var(--muted); }
  .ghost:hover:not(:disabled) { color: var(--text); background: var(--tint-hover); }
  .danger { color: var(--fail); }
  .danger:hover:not(:disabled) { background: var(--fail-t); }
</style>
