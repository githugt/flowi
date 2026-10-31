<script lang="ts">
  // One or more problems from the library, in plain words, with its FLW code small at the end.
  import { codeParts, describeProblems } from './lib/problems'

  let { text }: { text: string } = $props()
  const problems = $derived(describeProblems(text))
</script>

{#each problems as p, i}{#if i > 0}<br />{/if}<span class="problem">{#each codeParts(p.text) as part}{#if part.code}<code>{part.text}</code>{:else}{part.text}{/if}{/each}{#if p.code}<span class="code" title="Problem code, for searching and bug reports">{p.code}</span>{/if}</span>{/each}

<style>
  .problem { -webkit-user-select: text; user-select: text; }
  code { font-family: var(--mono); font-size: 0.95em; }
  .code { margin-left: 0.5em; color: var(--muted); font-size: var(--fs-xs); font-family: var(--mono); white-space: nowrap; }
</style>
