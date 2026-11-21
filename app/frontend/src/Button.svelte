<script lang="ts">
  import type { Snippet } from 'svelte'
  import type { HTMLButtonAttributes } from 'svelte/elements'

  // One button for every screen. secondary is the everyday one; primary is the main action of a page; go is the
  // Send-coloured "start something" action; ghost has no outline until hovered; danger is for what removes things.
  // loading shows a spinner and refuses clicks while keeping the label (the width does not jump); pressed is a
  // toggle's on state, announced as aria-pressed.
  // link is words in the accent colour with no box, for "+ Add", "Bulk edit" and an action inside a line of text.
  // danger also works as a modifier (a ghost button that removes something); variant="danger" is the same thing on the secondary look.
  type Props = HTMLButtonAttributes & { variant?: 'secondary' | 'primary' | 'go' | 'ghost' | 'danger' | 'link'; danger?: boolean; small?: boolean; loading?: boolean; pressed?: boolean; /** The button element, for moving focus to it. */ el?: HTMLButtonElement; children: Snippet }
  let { variant = 'secondary', danger = false, small = false, loading = false, pressed, el = $bindable(), children, ...rest }: Props = $props()
</script>

<button bind:this={el} class={variant} class:danger class:small class:loading class:pressed aria-busy={loading || undefined} aria-pressed={pressed} {...rest} disabled={rest.disabled || loading}>{#if loading}<span class="spin" aria-hidden="true"></span>{/if}{@render children()}</button>

<style>
  button { background: var(--raised); color: var(--text); border: 0; box-shadow: inset 0 0 0 1px var(--line); border-radius: var(--r-md); padding: 0 var(--sp-3); min-height: var(--ctl-h); font: inherit; font-weight: 500; white-space: nowrap; }
  button:hover:not(:disabled) { background: var(--raised-3); }
  button:disabled { opacity: var(--op-disabled); }
  .spin { display: inline-block; width: 0.9em; height: 0.9em; margin-right: 6px; vertical-align: -0.12em; border: 2px solid currentColor; border-right-color: transparent; border-radius: 50%; animation: spin 0.7s linear infinite; }
  @keyframes spin { to { transform: rotate(360deg); } }
  .loading:disabled { opacity: var(--op-busy); }
  .pressed { background: var(--tint-sel); box-shadow: inset 0 0 0 1px var(--accent); }
  .small { min-height: var(--ctl-h-sm); padding: 0 10px; }
  .primary:not(:disabled) { background: var(--primary-fill); color: var(--on-primary); font-weight: 600; box-shadow: inset 0 1px 0 var(--shine); }
  .primary:not(:disabled):hover { background: var(--primary-fill); filter: brightness(1.1); }
  .go:not(:disabled) { background: var(--send-fill); color: var(--on-send); font-weight: 600; box-shadow: var(--send-edge); }
  .go:not(:disabled):hover { background: var(--send-fill); filter: brightness(1.08); }
  .ghost { background: none; box-shadow: none; color: var(--muted); }
  .ghost:hover:not(:disabled) { color: var(--text); background: var(--tint-hover); }
  .link { background: none; box-shadow: none; color: var(--accent); padding: 2px 0; min-height: 0; font-weight: inherit; text-align: left; }
  .link:hover:not(:disabled) { background: none; text-decoration: underline; }
  .danger { color: var(--fail); }
  .danger:hover:not(:disabled) { background: var(--fail-t); }
</style>
