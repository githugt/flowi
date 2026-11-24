<script lang="ts">
  // The sidebar's filter: pick what to narrow the rows by (the kind of request, a status, a tag, a collection, or only
  // those with problems). Each choice adds or takes away its term in the "Jump to…" field, so what is picked here can
  // be typed there and the other way round: `type:grpc status:review in:hyperverge`.
  // Before: the filter button listed statuses and tags only, in a menu.
  import Popover from './Popover.svelte'
  import Button from './Button.svelte'
  import Checkbox from './Checkbox.svelte'
  import StatusIcon from './StatusIcon.svelte'
  import { toggleTerm, typeTerm } from './lib/devstatus'
  import { filtersNote, hasTerm, withoutFilters, withoutPrefix, type FilterChoices } from './lib/sidefilter'

  let { query = $bindable(''), choices, x, y, onclose }: { query: string; choices: FilterChoices; x: number; y: number; onclose: () => void } = $props()

  const toggle = (term: string) => (query = toggleTerm(query, term))
  const anyType = $derived(!choices.types.some((t) => hasTerm(query, `type:${typeTerm(t.id)}`)))
  const note = $derived(filtersNote(query))
</script>

<Popover {x} {y} end width={340} height={380} maxHeight="520px" label="Filter the sidebar" {onclose}>
  <section aria-labelledby="flt-type">
    <h3 id="flt-type">Type</h3>
    <div class="chips">
      <Button small pressed={anyType} onclick={() => (query = withoutPrefix(query, 'type:'))}>All</Button>
      {#each choices.types as t (t.id)}
        <Button small pressed={hasTerm(query, `type:${t.term}`)} title="{t.count} {t.count === 1 ? 'row' : 'rows'}" onclick={() => toggle(`type:${t.term}`)}>{t.name}</Button>
      {/each}
    </div>
  </section>
  {#if choices.statuses.length}
    <section aria-labelledby="flt-status">
      <h3 id="flt-status">Status</h3>
      <div class="chips">
        {#each choices.statuses as s (s.name)}
          <Button small pressed={hasTerm(query, `status:${s.name}`)} onclick={() => toggle(`status:${s.name}`)}><span class="with"><StatusIcon mark={s} size={12} />{s.name.replace(/[-_]+/g, ' ')}</span></Button>
        {/each}
      </div>
    </section>
  {/if}
  {#if choices.tags.length}
    <section aria-labelledby="flt-tag">
      <h3 id="flt-tag">Tag</h3>
      <div class="chips scroll">
        {#each choices.tags as t (t.tag)}
          <Button small pressed={hasTerm(query, `tag:${t.tag}`)} onclick={() => toggle(`tag:${t.tag}`)}>{t.tag} <span class="n">{t.count}</span></Button>
        {/each}
      </div>
    </section>
  {/if}
  {#if choices.collections.length > 1}
    <section aria-labelledby="flt-coll">
      <h3 id="flt-coll">Collection</h3>
      <div class="chips scroll">
        {#each choices.collections as c (c.name)}
          <Button small pressed={hasTerm(query, `in:${c.term}`)} onclick={() => toggle(`in:${c.term}`)}>{c.name} <span class="n">{c.count}</span></Button>
        {/each}
      </div>
    </section>
  {/if}
  <section>
    <Checkbox checked={hasTerm(query, 'has:problems')} onchange={() => toggle('has:problems')}>Only items with problems</Checkbox>
  </section>
  <div class="foot">
    <span class="note" aria-live="polite">{note}</span>
    <Button small variant="ghost" disabled={!note} onclick={() => (query = withoutFilters(query))}>Clear filters</Button>
  </div>
</Popover>

<style>
  section { display: flex; flex-direction: column; gap: 6px; }
  h3 { margin: 0; font-size: var(--fs-xs); font-weight: 700; letter-spacing: 0.05em; text-transform: uppercase; color: var(--muted); }
  .chips { display: flex; flex-wrap: wrap; gap: 6px; }
  .scroll { max-height: 96px; overflow: auto; }
  .with { display: inline-flex; align-items: center; gap: 5px; }
  .n { color: var(--muted); font-size: var(--fs-xs); }
  .foot { display: flex; align-items: center; justify-content: space-between; gap: 8px; padding-top: 6px; border-top: 1px solid var(--line); }
  .note { color: var(--muted); font-size: var(--fs-xs); }
</style>
