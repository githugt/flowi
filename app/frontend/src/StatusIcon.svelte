<script lang="ts">
  import Icon from './Icon.svelte'
  import { statusColor, statusTitle } from './lib/devstatus'
  import type { StatusMark } from './lib/types'

  // A development status as a small coloured icon: the colour is a theme token (app.css --st-*), the shape is the
  // status's own, so it is never told by colour alone. Its name and where it comes from are the tooltip and what a
  // screen reader says. A status a folder gives (inherited) is a little fainter than one the item sets.
  let { mark, size = 14 }: { mark: StatusMark; size?: number } = $props()
  const title = $derived(statusTitle(mark))
</script>

<span class="status" class:inherited={mark.inherited} style:color={statusColor(mark.color)} {title} role="img" aria-label={title}><Icon name={mark.icon} {size} /></span>

<style>
  .status { flex: none; display: inline-flex; }
  .inherited { opacity: 0.75; }
</style>
