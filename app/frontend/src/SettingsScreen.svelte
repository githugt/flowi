<script lang="ts">
  import EmptyState from './EmptyState.svelte'
  import { radiogroup } from './lib/radiogroup'
  import { acceptsText, defaultNote } from './lib/settinghint'
  import { isEnter } from './lib/enterkey'
  import Select from './Select.svelte'
  import TextInput from './TextInput.svelte'
  import Toggle from './Toggle.svelte'
  import { SettingsService } from '../bindings/github.com/githugt/flowi/app/internal/bridge'
  import { app } from './lib/app.svelte'
  import { SECTIONS, labelOf, matches, modifiedFields, sectionOf, type Section } from './lib/settings'
  import Button from './Button.svelte'
  import Tabs from './Tabs.svelte'
  import { posture, weakLines } from './lib/posture'
  import { settingAnnouncement } from './lib/settingsaid'
  import type { SettingView } from './lib/types'

  let query = $state('')
  // The last change, said once to a screen reader: settings apply as a field changes or loses focus.
  let announced = $state('')
  let seen: { name: string; value: string; problem?: string }[] = []
  // Show only the settings the person has changed, across every section.
  let onlyModified = $state(false)
  const all = $derived((app.settings?.groups ?? []).flatMap((g) => g.fields))
  const modified = $derived(modifiedFields(all).length)
  $effect(() => {
    const now = all.map((f) => ({ name: f.name, value: f.value, problem: f.problem }))
    const said = settingAnnouncement(seen, now)
    seen = now
    if (said) announced = said
  })
  const fields = $derived(all.filter((f) => matches(f, query) && (!onlyModified || f.userSet)))
  // A search or the Modified filter looks across all sections; otherwise a tab narrows the page.
  const searching = $derived(!!query || onlyModified)
  const isURLColour = (f: SettingView) => f.name.startsWith('appearance.url.')
  // Each section's settings, URL colours kept apart at the end of Appearance.
  const bySection = $derived(
    SECTIONS.map((s) => ({ name: s, fields: fields.filter((f) => sectionOf(f.name) === s && !isURLColour(f)) })),
  )
  const urlColours = $derived(fields.filter(isURLColour))
  const shown = (s: { name: Section; fields: SettingView[] }) => s.fields.length > 0 || (s.name === 'Appearance' && urlColours.length > 0)

  // One tab for each section. A search looks across all of them and shows what matches whichever tab is in front, so
  // a tab only narrows the page while the search is empty; the tab remembers where you were.
  const TAB_KEY = 'flowi.settings.tab'
  const remembered = (): Section => {
    try {
      const saved = localStorage.getItem(TAB_KEY)
      return SECTIONS.find((s) => s === saved) ?? SECTIONS[0]
    } catch {
      return SECTIONS[0] // storage can be blocked; the first tab is a fine start
    }
  }
  let tab = $state<Section>(remembered())
  function pick(s: Section) {
    tab = s
    try {
      localStorage.setItem(TAB_KEY, s)
    } catch {
      // a tab that is not remembered is not a problem
    }
  }
  const visible = $derived(searching ? bySection.filter(shown) : bySection.filter((s) => s.name === tab && shown(s)))
  // While searching, a tab shows how many of its settings match, so you can see where the hits are.
  const hits = (s: Section) => (searching ? (bySection.find((b) => b.name === s)?.fields.length ?? 0) + (s === 'Appearance' ? urlColours.length : 0) : 0)

  // Text inputs apply on Enter or blur; the typed text stays until then.
  let typed = $state<Record<string, string>>({})
  const text = (f: SettingView) => typed[f.name] ?? f.value
  function apply(f: SettingView) {
    if (typed[f.name] === undefined || typed[f.name] === f.value) return
    SettingsService.Set(f.name, typed[f.name])
    delete typed[f.name]
  }
  // The themes app.css defines colours for; system and custom follow the OS or a file, so their swatch shows the current one.
  const THEMES_WITH_COLOURS = ['classic', 'light', 'dark', 'high-contrast', 'solarized-light', 'solarized-dark', 'nord', 'anuppuccin-dark', 'anuppuccin-light', 'flowi-dark', 'flowi-light']
</script>

{#snippet field(f: SettingView)}
  <div class="field">
    <div class="head">
      <label for={'set-' + f.name}>{labelOf(f.name)}</label>
      {#if f.userSet}<Button variant="link" onclick={() => SettingsService.Reset(f.name)}>Reset</Button>{/if}
    </div>
    <p class="desc">{f.description} <code title="The key in settings.yaml">{f.name}</code>{#if !f.userSet} <span class="src">default</span>{/if}</p>
    {#if f.kind === 'bool'}
      <Toggle id={'set-' + f.name} checked={f.value === 'true'} onchange={(on) => SettingsService.Set(f.name, String(on))}>{f.value === 'true' ? 'On' : 'Off'}</Toggle>
    {:else if f.name === 'appearance.send_button'}
      <div class="swatches" role="radiogroup" use:radiogroup aria-label="Send button colour">
        {#each f.enum ?? [] as o}
          <button class="swatch" role="radio" aria-checked={f.value === o} class:on={f.value === o} data-send={o} title={o} onclick={() => SettingsService.Set(f.name, o)}>
            <span class="sample">Send</span>{o}
          </button>
        {/each}
      </div>
    {:else if f.name === 'appearance.theme'}
      <div class="swatches" role="radiogroup" use:radiogroup aria-label="Theme">
        {#each f.enum ?? [] as o}
          <button class="swatch themeswatch" role="radio" aria-checked={f.value === o} class:on={f.value === o} data-theme={THEMES_WITH_COLOURS.includes(o) ? o : undefined} title={o} onclick={() => SettingsService.Set(f.name, o)}>
            <span class="themesample" aria-hidden="true"><i class="bar"></i><i class="line"></i><i class="dot"></i></span>{o.replace(/-/g, ' ')}
          </button>
        {/each}
      </div>
    {:else if f.kind === 'enum'}
      <Select id={'set-' + f.name} value={f.value} onchange={(e) => SettingsService.Set(f.name, e.currentTarget.value)}>
        {#each f.enum ?? [] as o}<option value={o}>{o}</option>{/each}
      </Select>
    {:else}
      <TextInput
        id={'set-' + f.name}
        value={text(f)}
        placeholder={f.kind === 'list' ? 'comma-separated' : ''}
        invalid={!!f.problem}
        describedby={f.problem ? 'prob-' + f.name : undefined}
        oninput={(e) => (typed[f.name] = (e.currentTarget as HTMLInputElement).value)}
        onblur={() => apply(f)}
        onkeydown={(e) => isEnter(e) && apply(f)}
      />
    {/if}
    {#if acceptsText(f) && f.kind !== 'bool' && f.kind !== 'enum'}<p class="desc">{acceptsText(f)}</p>{/if}
    {#if defaultNote(f)}<p class="desc">{defaultNote(f)}</p>{/if}
    {#if f.problem}<p class="problem" id={'prob-' + f.name} role="alert">{f.problem}</p>{/if}
    {#if (f.name === 'appearance.theme' || f.name === 'appearance.palette_file') && app.paletteProblem}
      <p class="problem">The custom theme was not applied: {app.paletteProblem} The window keeps the theme it has.</p>
    {/if}
  </div>
{/snippet}

<div class="page">
  <p class="sr-only" aria-live="polite" role="status">{announced}</p>
  <header>
    <h2>Settings</h2>
    <TextInput bind:value={query} placeholder="Search settings" label="Search settings" style="width: 260px" />
    {#if modified > 0 || onlyModified}<Button small aria-pressed={onlyModified} variant={onlyModified ? 'primary' : 'secondary'} title="Show only the settings you have changed" onclick={() => (onlyModified = !onlyModified)}>Modified ({modified})</Button>{/if}
  </header>
  <p class="note">Your personal settings. Collections, folders and requests can override some of them.</p>
  <div class="tabbar"><Tabs label="Settings sections" wrap active={searching ? null : tab} items={SECTIONS.map((s) => ({ id: s, label: s, count: hits(s) > 0 ? hits(s) : undefined }))} onpick={(s) => { query = ''; onlyModified = false; pick(s as Section) }} /></div>
  {#each visible as s (s.name)}
    <section role="tabpanel">
      {#if searching}<h3>{s.name}</h3>{/if}
      {#if s.name === 'Security'}
        <ul class="posture" aria-label="Security posture">
          {#each posture(all) as p (p.name)}
            <li class:weak={!p.ok}><span aria-hidden="true">{p.ok ? '✓' : '!'}</span> {p.text}{#if !p.ok} <Button small onclick={() => SettingsService.Set(p.name, p.safe)}>Fix</Button>{/if}</li>
          {/each}
        </ul>
        {#if weakLines(posture(all)).length > 1}
          <Button small title="Put every weaker choice above back to the built-in one" onclick={() => weakLines(posture(all)).forEach((p) => SettingsService.Set(p.name, p.safe))}>Reset to safe defaults</Button>
        {/if}
      {/if}
      {#each s.fields as f (f.name)}{@render field(f)}{/each}
      {#if s.name === 'Appearance' && urlColours.length}
        <details class="sub">
          <summary>URL colours <Button variant="link" onclick={() => SettingsService.ResetGroup('appearance.url')}>Reset all</Button></summary>
          {#each urlColours as f (f.name)}{@render field(f)}{/each}
        </details>
      {/if}
    </section>
  {:else}
    <EmptyState kind="filtered" compact title={onlyModified && !query ? 'You have not changed any setting' : undefined} {query} onclear={() => { query = ''; onlyModified = false }} />
  {/each}
</div>

<style>
  .page { position: absolute; inset: 0; overflow: auto; padding: var(--sp-4) var(--sp-6) 48px; max-width: 760px; }
  header { display: flex; align-items: center; gap: var(--sp-3); }
  h2 { margin: 0; font-size: var(--fs-lg); flex: 1; }
  .note { color: var(--muted); margin: var(--sp-2) 0 0; font-size: var(--fs-sm); }
  .tabbar { margin: var(--sp-3) 0 0; }
  h3, summary { margin: var(--sp-6) 0 var(--sp-2); font-size: var(--fs-lg); font-weight: 600; }
  summary { cursor: pointer; display: flex; align-items: center; gap: var(--sp-2); }
  .sub summary { font-weight: 500; margin-top: var(--sp-4); }
  .field { margin: var(--sp-3) 0; }
  .head { display: flex; justify-content: space-between; align-items: baseline; }
  label { font-weight: 500; }
  code { margin-left: 0.4em; font: var(--fs-xs) var(--mono); color: var(--muted); opacity: 0.8; }
  .desc { margin: 2px 0 6px; color: var(--muted); font-size: var(--fs-sm); }
  .src { margin-left: 0.4em; font-size: var(--fs-xs); opacity: 0.7; }
  .swatches { display: flex; flex-wrap: wrap; gap: var(--sp-2); }
  .swatch { display: flex; flex-direction: column; align-items: center; gap: 6px; padding: 10px 14px; background: var(--panel); border: 1px solid var(--line); border-radius: var(--r-lg); color: var(--muted); font: inherit; font-size: var(--fs-xs); text-transform: capitalize; }
  .swatch.on { border-color: var(--accent); color: var(--text); box-shadow: 0 0 0 3px var(--tint-accent); }
  .sample { display: inline-block; padding: 0 14px; line-height: 26px; border-radius: var(--r-md); background: var(--send-fill); color: var(--on-send); box-shadow: var(--send-edge), 0 4px 14px var(--send-glow); font-size: var(--fs-sm); font-weight: 600; text-transform: none; }
  .themeswatch { background: var(--panel); }
  .themesample { position: relative; display: block; width: 72px; height: 44px; border-radius: var(--r-md); background: var(--bg); border: 1px solid var(--line); overflow: hidden; }
  .themesample i { position: absolute; display: block; }
  .themesample .bar { left: 0; top: 0; bottom: 0; width: 18px; background: var(--panel); border-right: 1px solid var(--line); }
  .themesample .line { left: 26px; top: 12px; width: 34px; height: 5px; border-radius: var(--r-pill); background: var(--text); opacity: 0.7; }
  .themesample .dot { left: 26px; top: 26px; width: 22px; height: 8px; border-radius: var(--r-pill); background: var(--accent); }
  .problem { color: var(--fail); margin: 4px 0 0; font-size: var(--fs-sm); }
</style>
