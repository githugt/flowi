<script lang="ts">
  import { radiogroup } from './lib/radiogroup'
  import { envSwatch } from './lib/envcolours'

  // The colours an environment can be marked with, as a row of dots: one chosen, or none for the default. Used by the
  // environment's own editor and by Manage environments, so the two look and behave alike.
  let { colours, value, onpick, label = 'Colour' }: { colours: string[]; value: string; onpick: (colour: string) => void; label?: string } = $props()
</script>

<div class="swatches" role="radiogroup" use:radiogroup aria-label={label}>
  <button type="button" class="sw none" class:on={!value} role="radio" aria-checked={!value} title="Default colour" aria-label="Default colour" onclick={() => onpick('')}></button>
  {#each colours as c (c)}
    <button type="button" class="sw" class:on={value === c} role="radio" aria-checked={value === c} title={c} aria-label={c} style:background={envSwatch(c)} onclick={() => onpick(c)}></button>
  {/each}
</div>

<style>
  .swatches { display: flex; align-items: center; gap: 6px; }
  .sw { width: 16px; height: 16px; padding: 0; flex: none; border-radius: 50%; border: 2px solid transparent; box-sizing: border-box; background: var(--bg); }
  .sw.none { border: 1px dashed var(--muted); }
  .sw.on { outline: 2px solid var(--text); outline-offset: 1px; }
</style>
