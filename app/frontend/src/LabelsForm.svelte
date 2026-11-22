<script lang="ts">
  import { onMount, untrack } from 'svelte'
  import { WorkspaceService } from '../bindings/github.com/githugt/flowi/app/internal/bridge'
  import { app } from './lib/app.svelte'
  import { ui } from './lib/ui.svelte'
  import { errorText } from './lib/errors'
  import { cleanStatusName, cleanTag, cleanTags, statusColor, statusWords, suggestTags } from './lib/devstatus'
  import type { LabelsView } from './lib/types'
  import Button from './Button.svelte'
  import Field from './Field.svelte'
  import Icon from './Icon.svelte'
  import IconButton from './IconButton.svelte'
  import StatusIcon from './StatusIcon.svelte'
  import TextInput from './TextInput.svelte'

  // Tags and development status of one request, flow or folder (node is its row in the tree), written to its file at
  // once. The status is one of the collection's (the five defaults and its own); a new one is made here, with a name,
  // a colour and an icon. Tags are free words: the ones in use in the collection are offered, most used first, so a tag
  // is not typed twice in two spellings.
  let { node }: { node: string } = $props()

  let view = $state<LabelsView | null>(null)
  let typed = $state('')
  let error = $state('')
  let making = $state(false)
  let name = $state('')
  let color = $state('blue')
  let icon = $state('flag')

  // The tree row carries the status and tags, so a change from anywhere (the sidebar, git, another tab) shows here.
  const row = $derived(app.find(node))
  const sig = $derived(JSON.stringify([row?.status, row?.tags]))
  const load = () =>
    WorkspaceService.Labels(node)
      .then((v) => (view = v as unknown as LabelsView))
      .catch((e) => (error = errorText(e)))
  onMount(() => void load())
  $effect(() => {
    void sig
    untrack(() => {
      if (view) void load()
    })
  })

  const suggestions = $derived(view ? suggestTags(view.suggest, view.tags, typed) : [])
  const known = $derived(view ? view.statuses.some((s) => s.name === view!.status) : true)

  async function save(tags: string[], status: string, drop = false) {
    if (!view) return
    error = ''
    // Shown at once; the tree event brings the real thing, and a failure reloads it.
    view.tags = tags
    view.status = status
    try {
      if (await WorkspaceService.SetLabels(node, tags, status, drop)) {
        ui.confirm('Rewrite the file?', 'The file has comments. Saving rewrites it in Flowi’s own layout and drops them.', () => save(tags, status, true), 'Save anyway')
        void load()
      }
    } catch (e) {
      error = errorText(e)
      void load()
    }
  }

  function addTags(extra: string[]) {
    if (!view) return
    const next = cleanTags([...view.tags, ...extra])
    typed = ''
    if (next.length !== view.tags.length) void save(next, view.status)
  }
  const removeTag = (t: string) => view && void save(view.tags.filter((x) => x !== t), view.status)
  const pickStatus = (s: string) => view && view.status !== s && void save(view.tags, s)

  // A comma typed or pasted ends a tag, however it arrived (a pasted "a, b, c" makes three).
  function tagInput(e: Event) {
    const input = e.currentTarget as HTMLInputElement
    if (!input.value.includes(',')) return
    const parts = input.value.split(',')
    const rest = parts.pop() ?? ''
    addTags(parts)
    // The bound text went "" → "a," → "" inside one tick, so Svelte sees no change and would leave the comma in the field.
    input.value = rest
    typed = rest
  }

  function tagKeys(e: KeyboardEvent) {
    if (e.key === 'Enter' || e.key === ',') {
      if (!cleanTag(typed)) return void (e.key === ',' && e.preventDefault())
      e.preventDefault()
      addTags([typed])
    } else if (e.key === 'Backspace' && !typed && view?.tags.length) removeTag(view.tags[view.tags.length - 1])
    else if (e.key === 'Escape' && typed) {
      typed = ''
      e.stopPropagation()
    }
  }

  const cleanName = $derived(cleanStatusName(name))
  const taken = $derived(!!view?.statuses.some((s) => s.name === cleanName))
  async function makeStatus(drop = false) {
    if (!view || !cleanName) return
    error = ''
    try {
      if (await WorkspaceService.AddStatus(node, cleanName, color, icon, drop)) {
        ui.confirm('Rewrite the file?', 'The collection’s flowi.yaml has comments. Saving rewrites it in Flowi’s own layout and drops them.', () => makeStatus(true), 'Save anyway')
        return
      }
      const made = cleanName
      making = false
      name = ''
      await load()
      void pickStatus(made)
    } catch (e) {
      error = errorText(e)
    }
  }
</script>

{#if view}
  <div class="labels">
    <div class="group" role="group" aria-label="Status">
      <span class="label">Status</span>
      <div class="chips">
        <Button small pressed={view.status === ''} onclick={() => pickStatus('')} title={view.inherited ? `Use the status of the folder ${view.inherited.from?.split('/').slice(1).join('/')}` : 'No status'}>
          {#if view.inherited}<StatusIcon mark={view.inherited} size={13} />{/if}{view.inherited ? `${statusWords(view.inherited.name)} (folder)` : 'None'}
        </Button>
        {#each view.statuses as s (s.name)}
          <Button small pressed={view.status === s.name} onclick={() => pickStatus(s.name)}><StatusIcon mark={s} size={13} />{statusWords(s.name)}</Button>
        {/each}
        {#if !known}
          <Button small pressed><StatusIcon mark={{ name: view.status, color: 'gray', icon: 'alert', unknown: true }} size={13} />{statusWords(view.status)}</Button>
        {/if}
        <Button small variant="ghost" aria-expanded={making} onclick={() => (making = !making)}><Icon name="plus" size={13} />New status</Button>
      </div>
    </div>

    {#if making}
      <div class="making">
        <Field label="Name of the new status">
          <TextInput bind:value={name} placeholder="needs-design" label="Name of the new status" onenter={() => void makeStatus()} error={taken ? 'There is a status of this name; adding it changes its colour and icon.' : ''} />
        </Field>
        <div class="group" role="group" aria-label="Colour">
          <span class="label">Colour</span>
          <div class="swatches">
            {#each view.colors as c (c)}
              <Button small pressed={color === c} aria-label={c} title={c} onclick={() => (color = c)}><span class="swatch" style:background={statusColor(c)}></span></Button>
            {/each}
          </div>
        </div>
        <div class="group" role="group" aria-label="Icon">
          <span class="label">Icon</span>
          <div class="swatches">
            {#each view.icons as i (i)}
              <Button small pressed={icon === i} aria-label={i} title={i} style={`color: ${statusColor(color)}`} onclick={() => (icon = i)}><Icon name={i} size={15} /></Button>
            {/each}
          </div>
        </div>
        <div class="row">
          <Button small variant="primary" disabled={!cleanName} onclick={() => void makeStatus()}>{taken ? 'Change status' : 'Add status'}</Button>
          <Button small variant="ghost" onclick={() => (making = false)}>Cancel</Button>
          <span class="preview"><StatusIcon mark={{ name: cleanName || 'status', color, icon }} size={14} />{statusWords(cleanName)}</span>
        </div>
      </div>
    {/if}

    <div class="group" role="group" aria-label="Tags">
      <span class="label">Tags</span>
      <div class="tags">
        {#each view.folderTags as t (t)}<span class="tag fromfolder" title="From a folder above; change it on the folder">#{t}</span>{/each}
        {#each view.tags as t (t)}
          <span class="tag"><span>#{t}</span><IconButton icon="close" size={10} label={`Remove the tag ${t}`} onclick={() => removeTag(t)} /></span>
        {/each}
        <span class="add"><TextInput bind:value={typed} placeholder="Add a tag" label="Add a tag" oninput={tagInput} onkeydown={tagKeys} onblur={() => addTags([typed])} /></span>
      </div>
      {#if suggestions.length}
        <div class="chips" role="group" aria-label="Tags in use in this collection">
          {#each suggestions as s (s.tag)}
            <Button small variant="ghost" title={`${s.count} ${s.count === 1 ? 'item uses' : 'items use'} it`} onclick={() => addTags([s.tag])}><Icon name="plus" size={12} />{s.tag}</Button>
          {/each}
        </div>
      {/if}
    </div>
    {#if error}<p class="err" role="alert">{error}</p>{/if}
  </div>
{:else if error}
  <p class="err" role="alert">{error}</p>
{/if}

<style>
  .labels { display: flex; flex-direction: column; gap: var(--sp-3); }
  .group { display: flex; flex-direction: column; gap: 4px; min-width: 0; }
  .label { color: var(--muted); font-size: var(--fs-xs); }
  .chips, .swatches, .tags { display: flex; flex-wrap: wrap; align-items: center; gap: 4px; }
  .chips :global(svg) { flex: none; }
  .chips :global(button) { display: inline-flex; align-items: center; gap: 5px; }
  .making { display: flex; flex-direction: column; gap: var(--sp-2); padding: var(--sp-2); background: var(--panel); border-radius: var(--r-md); box-shadow: inset 0 0 0 1px var(--line); }
  .swatch { display: block; width: 14px; height: 14px; border-radius: 50%; box-shadow: inset 0 0 0 1px var(--line); }
  .swatches :global(button) { display: inline-grid; place-items: center; padding: 0 6px; }
  .row { display: flex; align-items: center; gap: 6px; }
  .preview { margin-left: auto; display: inline-flex; align-items: center; gap: 5px; color: var(--muted); font-size: var(--fs-sm); }
  .tag { display: inline-flex; align-items: center; gap: 2px; padding: 0 2px 0 8px; height: 22px; border-radius: var(--r-pill); background: var(--tint-accent); color: var(--text); font-size: var(--fs-sm); }
  .tag.fromfolder { padding: 0 8px; background: none; box-shadow: inset 0 0 0 1px var(--line); color: var(--muted); }
  .add { width: 140px; flex: none; }
  .err { margin: 0; color: var(--fail); font-size: var(--fs-sm); }
</style>
