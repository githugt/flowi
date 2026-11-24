<script lang="ts">
  import type { HTMLButtonAttributes } from 'svelte/elements'
  import Icon from './Icon.svelte'

  // An icon on its own: 24px, no outline until hovered, and a label that is not optional (it is the name a screen
  // reader says, and the tooltip unless hint gives a longer one, such as why the button is off). danger turns red on
  // hover, for what removes things.
  type Props = Omit<HTMLButtonAttributes, 'aria-label' | 'title'> & { icon: string; label: string; hint?: string; danger?: boolean; size?: number; filled?: boolean }
  let { icon, label, hint, danger = false, size = 14, filled = false, ...rest }: Props = $props()
</script>

<button class="iconbtn" class:danger aria-label={label} title={hint ?? label} {...rest}><Icon name={icon} {size} {filled} /></button>

<style>
  .iconbtn { display: inline-grid; place-items: center; width: 24px; height: 24px; padding: 0; flex: none; border: 0; border-radius: var(--r-md); background: none; color: var(--muted); font: inherit; }
  .iconbtn:hover:not(:disabled) { color: var(--text); background: var(--tint-hover); }
  .danger:hover:not(:disabled) { color: var(--fail); background: var(--fail-t); }
  .iconbtn:disabled { opacity: var(--op-disabled); }
</style>
