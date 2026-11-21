<script lang="ts">
  // Small stroke icons (24x24 grid), drawn from lib/icons so the window needs no icon font.
  import { iconPath } from './lib/icons'
  let { name, size = 16 }: { name: string; size?: number } = $props()
</script>

<svg width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
  <path d={iconPath(name)} />
</svg>
