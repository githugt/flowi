<script lang="ts">
  import { SettingsService } from '../bindings/github.com/githugt/flowi/internal/bridge'
  import { app } from './lib/app.svelte'
  import { SECTIONS, labelOf, matches, sectionOf, type Section } from './lib/settings'
  import type { SettingView } from './lib/types'

  let query = $state('')
  const fields = $derived((app.settings?.groups ?? []).flatMap((g) => g.fields).filter((f) => matches(f, query)))
  const isURLColour = (f: SettingView) => f.name.startsWith('appearance.url.')
  // Each section's settings, URL colours kept apart at the end of Appearance.
  const bySection = $derived(
    SECTIONS.map((s) => ({ name: s, fields: fields.filter((f) => sectionOf(f.name) === s && !isURLColour(f)) })),
  )
  const urlColours = $derived(fields.filter(isURLColour))
  const shown = (s: { name: Section; fields: SettingView[] }) => s.fields.length > 0 || (s.name === 'Appearance' && urlColours.length > 0)

  // Text inputs apply on Enter or blur; the typed text stays until then.
  let typed = $state<Record<string, string>>({})
  const text = (f: SettingView) => typed[f.name] ?? f.value
  function apply(f: SettingView) {
    if (typed[f.name] === undefined || typed[f.name] === f.value) return
    SettingsService.Set(f.name, typed[f.name])
    delete typed[f.name]
  }
</script>

{#snippet field(f: SettingView)}
  <div class="field">
    <div class="head">
      <label for={'set-' + f.name}>{labelOf(f.name)}</label>
      {#if f.userSet}<button class="link" onclick={() => SettingsService.Reset(f.name)}>Reset</button>{/if}
    </div>
    <p class="desc">{f.description} <code title="The key in settings.yaml">{f.name}</code>{#if !f.userSet} <span class="src">default</span>{/if}</p>
    {#if f.kind === 'bool'}
      <input id={'set-' + f.name} type="checkbox" checked={f.value === 'true'} onchange={(e) => SettingsService.Set(f.name, String(e.currentTarget.checked))} />
    {:else if f.kind === 'enum'}
      <select id={'set-' + f.name} value={f.value} onchange={(e) => SettingsService.Set(f.name, e.currentTarget.value)}>
        {#each f.enum ?? [] as o}<option value={o}>{o}</option>{/each}
      </select>
    {:else}
      <input
        id={'set-' + f.name}
        class="text"
        value={text(f)}
        placeholder={f.kind === 'list' ? 'comma-separated' : ''}
        oninput={(e) => (typed[f.name] = e.currentTarget.value)}
        onblur={() => apply(f)}
        onkeydown={(e) => e.key === 'Enter' && apply(f)}
        spellcheck="false"
      />
    {/if}
    {#if f.problem}<p class="problem">{f.problem}</p>{/if}
  </div>
{/snippet}

<div class="page">
  <header>
    <h2>Settings</h2>
    <input class="search" type="search" placeholder="Search settings" aria-label="Search settings" bind:value={query} spellcheck="false" />
  </header>
  <p class="note">Your personal settings. Collections, folders and requests can override some of them.</p>
  {#each bySection.filter(shown) as s (s.name)}
    {#if s.name === 'Advanced' && !query}
      <details class="advanced">
        <summary>Advanced <span class="count">{s.fields.length}</span></summary>
        {#each s.fields as f (f.name)}{@render field(f)}{/each}
      </details>
    {:else}
      <section>
        <h3>{s.name}</h3>
        {#each s.fields as f (f.name)}{@render field(f)}{/each}
        {#if s.name === 'Appearance' && urlColours.length}
          <details class="sub">
            <summary>URL colours <button class="link" onclick={() => SettingsService.ResetGroup('appearance.url')}>Reset all</button></summary>
            {#each urlColours as f (f.name)}{@render field(f)}{/each}
          </details>
        {/if}
      </section>
    {/if}
  {:else}
    <p class="note">No setting matches “{query}”.</p>
  {/each}
</div>

<style>
  .page { position: absolute; inset: 0; overflow: auto; padding: var(--sp-4) var(--sp-6) 48px; max-width: 760px; }
  header { display: flex; align-items: center; gap: var(--sp-3); }
  h2 { margin: 0; font-size: var(--fs-lg); flex: 1; }
  .search { width: 260px; background: var(--bg); color: var(--text); border: 1px solid var(--line); border-radius: var(--r-md); padding: 5px 10px; font: inherit; -webkit-user-select: text; user-select: text; }
  .note { color: var(--muted); margin: var(--sp-2) 0 0; font-size: var(--fs-sm); }
  h3, summary { margin: var(--sp-6) 0 var(--sp-2); font-size: var(--fs-md); font-weight: 600; border-bottom: 1px solid var(--line); padding-bottom: 4px; }
  summary { cursor: pointer; display: flex; align-items: center; gap: var(--sp-2); }
  .sub summary { font-weight: 500; margin-top: var(--sp-4); }
  .count { color: var(--muted); font-weight: 400; font-size: var(--fs-xs); }
  .field { margin: var(--sp-3) 0; }
  .head { display: flex; justify-content: space-between; align-items: baseline; }
  label { font-weight: 500; }
  code { margin-left: 0.4em; font: var(--fs-xs) var(--mono); color: var(--muted); opacity: 0.8; }
  .desc { margin: 2px 0 6px; color: var(--muted); font-size: var(--fs-sm); }
  .src { margin-left: 0.4em; font-size: var(--fs-xs); opacity: 0.7; }
  .text, select {
    width: 100%; box-sizing: border-box; background: var(--bg); color: var(--text); border: 1px solid var(--line); border-radius: var(--r-md); padding: 5px 8px; font: inherit;
    -webkit-user-select: text; user-select: text;
  }
  select { width: auto; }
  .problem { color: var(--fail); margin: 4px 0 0; font-size: var(--fs-sm); }
  .link { background: none; border: 0; color: var(--accent); font: inherit; font-size: var(--fs-sm); font-weight: 400; margin-left: auto; }
</style>
