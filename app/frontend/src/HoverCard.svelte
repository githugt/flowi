<script lang="ts">
  // The card of the row the pointer rests on (lib/hovercard): what it is and where it lives, what it talks to, what it
  // says about itself, its status and tags, and what is wrong with it, with the code and what to do. It sits beside the
  // sidebar, level with the row, kept inside the window. Not clickable: the pointer passes through it, so it never
  // covers the next row.
  import { portal } from './lib/portal'
  import { hover } from './lib/hovercard.svelte'
  import Verb from './Verb.svelte'
  import Icon from './Icon.svelte'

  const WIDTH = 320
  const EDGE = 8
  let drawn = $state(0)
  const card = $derived(hover.card)
  const left = $derived(Math.max(EDGE, Math.min(hover.anchor.x, window.innerWidth - WIDTH - EDGE)))
  const top = $derived(Math.max(EDGE, Math.min(hover.anchor.top, window.innerHeight - drawn - EDGE)))
</script>

<svelte:window onkeydowncapture={(e) => { if (e.key === 'Escape' && hover.card) hover.leave() }} onblur={() => hover.leave()} />

{#if card}
  <div class="card" use:portal role="tooltip" bind:offsetHeight={drawn} style:left="{left}px" style:top="{top}px" style:width="{WIDTH}px">
    <div class="head">
      {#if card.verb}<Verb verb={card.verb} short compact />{/if}
      <strong class="title">{card.title}</strong>
    </div>
    <div class="kind">{card.kind}{#if card.where}{" · "}{card.where}{/if}</div>
    {#if card.target}
      <div class="target"><span class="k">{card.targetLabel}</span><code>{card.target}</code></div>
    {/if}
    {#if card.description}<p class="desc">{card.description}</p>{/if}
    {#if card.facts.length}
      <dl>
        {#each card.facts as f (f.label)}<dt>{f.label}</dt><dd>{f.value}</dd>{/each}
      </dl>
    {/if}
    {#if card.problems.length}
      <ul class="problems" aria-label="Problems">
        {#each card.problems as p, i (i)}
          <li class={p.severity}>
            <span class="mark"><Icon name="alert" size={13} /></span>
            <span class="what">{p.message}{#if p.code}<span class="meta"><code>{p.code}</code></span>{/if}{#if p.hint}<span class="hint">{p.hint}</span>{/if}</span>
          </li>
        {/each}
        {#if card.moreProblems}<li class="more">and {card.moreProblems} more in the Problems panel</li>{/if}
      </ul>
    {/if}
  </div>
{/if}

<style>
  .card {
    position: fixed; z-index: var(--z-top); box-sizing: border-box; max-height: calc(100vh - 16px); overflow: hidden; pointer-events: none;
    display: flex; flex-direction: column; gap: 6px; padding: 10px 12px;
    background: var(--raised); color: var(--text); border: 1px solid var(--line); border-radius: var(--r-lg); box-shadow: var(--shadow);
    font-size: var(--fs-sm); line-height: 1.4;
  }
  .head { display: flex; align-items: center; gap: 8px; min-width: 0; }
  .title { font-size: var(--fs-md); overflow-wrap: anywhere; }
  .kind { color: var(--muted); font-size: var(--fs-xs); }
  .target { display: flex; flex-direction: column; gap: 2px; }
  .k { color: var(--muted); font-size: var(--fs-xs); }
  code { font-family: var(--mono); font-size: var(--fs-xs); overflow-wrap: anywhere; }
  .desc { margin: 0; color: var(--text); overflow-wrap: anywhere; }
  dl { display: grid; grid-template-columns: max-content 1fr; gap: 2px 10px; margin: 0; }
  dt { color: var(--muted); }
  dd { margin: 0; overflow-wrap: anywhere; }
  .problems { list-style: none; margin: 2px 0 0; padding: 6px 0 0; border-top: 1px solid var(--line); display: flex; flex-direction: column; gap: 6px; }
  .problems li { display: flex; gap: 6px; align-items: flex-start; }
  .problems .mark { flex: none; display: inline-flex; margin-top: 2px; }
  .problems .error .mark { color: var(--fail); }
  .problems .warning .mark { color: var(--warn); }
  .what { min-width: 0; }
  .meta { display: block; color: var(--muted); }
  .hint { display: block; color: var(--muted); font-size: var(--fs-xs); }
  .more { color: var(--muted); font-size: var(--fs-xs); padding-left: 19px; }
</style>
