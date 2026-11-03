<script lang="ts">
  // The flowi mark: the app icon's tile (a white Fi whose F is a wave ending in the i's yellow dot), drawn from the
  // same art as build/appicon.svg. It keeps its brand colours in every theme.
  let { size = 20 }: { size?: number } = $props()
  // Each copy clips with its own id: copies on one page must not share one.
  const clip = `logo-tile-${Math.random().toString(36).slice(2)}`
</script>

<svg width={size} height={size} viewBox="0 0 100 100" aria-hidden="true">
  <defs><clipPath id={clip}><rect width="100" height="100" rx="22.4" /></clipPath></defs>
  <g clip-path={`url(#${clip})`}>
    <rect class="tile" width="100" height="100" />
    <path class="wave1" d="M0 68C30 52 55 82 100 40V100H0Z" />
    <path class="wave2" d="M0 84C30 70 58 96 100 62V100H0Z" />
    <g class="ink" fill="none" stroke-width="10" stroke-linecap="round" stroke-linejoin="round">
      <path d="M26 78V26" />
      <path d="M26 26Q40 12 54 26T76 26" />
      <path d="M26 52H50" />
      <path d="M78 52V76" />
    </g>
    <circle class="dot" cx="78" cy="34" r="7" />
  </g>
</svg>

<style>
  .tile { fill: var(--brand-tile); }
  .wave1 { fill: var(--brand-wave-1); }
  .wave2 { fill: var(--brand-wave-2); }
  .ink { stroke: var(--brand-ink); }
  .dot { fill: var(--brand-dot); }
</style>
