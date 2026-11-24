<script lang="ts">
  // A "?" that opens what an expression field can read: the names, the
  // operators and functions, and examples. What it shows depends on where the
  // field is (a flow step, a request, a message request, an "only if" filter).
  import Popover from './Popover.svelte'
  import { copy } from './lib/clip'
  import Button from './Button.svelte'
  import HelpButton from './HelpButton.svelte'
  import Tabs from './Tabs.svelte'
  import { SystemService } from '../bindings/github.com/githugt/flowi/app/internal/bridge'
  import type { ExprHelpView } from './lib/types'

  let { context }: { context: string } = $props()

  let open = $state(false)
  let help = $state<ExprHelpView | null>(null)
  let tab = $state<'names' | 'tools' | 'generators'>('names')
  let at = $state({ x: 0, y: 0 })
  let opener = $state<HTMLButtonElement>()

  async function toggle(e: MouseEvent) {
    e.preventDefault()
    if (open) return void (open = false)
    const r = (e.currentTarget as HTMLElement).getBoundingClientRect()
    help = (await SystemService.ExpressionHelp(context)) as unknown as ExprHelpView
    tab = 'names'
    at = { x: r.left, y: r.bottom }
    open = true
  }
  // Escape and a press outside both land here; the focus goes back to the ? that opened it.
  function close() {
    open = false
    opener?.focus()
  }

  const rows = $derived(help ? help[tab] : [])
</script>

<span class="ask"><HelpButton bind:el={opener} label="Help with this expression" title="What can I write here?" aria-haspopup="dialog" aria-expanded={open} onclick={toggle} /></span>

{#if open && help}
  <Popover x={at.x} y={at.y} width={440} height={420} maxHeight="min(70vh, 560px)" label={help.title} onclose={close}>
    <!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions -->
    <div class="help" onclick={(e) => e.stopPropagation()}>
      <h4>{help.title}</h4>
      <p class="intro">{help.intro}</p>
      <div class="tabbar"><Tabs label="What can be written" size="sm" items={[{ id: 'names', label: 'Names' }, { id: 'tools', label: 'Operators and functions' }, { id: 'generators', label: '$generators' }]} active={tab} onpick={(id) => (tab = id as typeof tab)} /></div>
      <div class="rows">
        {#each rows as r, i (`${tab}:${i}`)}
          <div class="row">
            <Button small variant="ghost" title={`Copy ${r.name}`} onclick={() => copy(r.name, r.name)}><code class="name">{r.name}</code></Button>
            <span class="what">{r.what}</span>
            {#if r.example}<code class="ex">{r.example}</code>{/if}
          </div>
        {/each}
      </div>
      {#if help.examples.length}
        <h5>Examples</h5>
        {#each help.examples as e}<code class="ex block">{e}</code>{/each}
      {/if}
    </div>
  </Popover>
{/if}

<style>
  .ask { display: inline-flex; vertical-align: middle; margin-left: 6px; }
  .help { font-size: var(--fs-sm); color: var(--text); }
  h4 { margin: 0 0 4px; font-size: var(--fs-md); }
  h5 { margin: 10px 0 4px; font-size: var(--fs-xs); color: var(--muted); text-transform: uppercase; letter-spacing: 0.05em; }
  .intro { margin: 0 0 8px; color: var(--muted); line-height: 1.45; }
  .tabbar { margin-bottom: 6px; }
  .row { display: grid; grid-template-columns: 1fr; gap: 2px; padding: 5px 0; border-bottom: 1px solid color-mix(in srgb, var(--line) 50%, transparent); }
  .name { color: var(--accent); font: var(--fs-sm) var(--mono); }
  .what { line-height: 1.4; }
  .ex { font: var(--fs-xs) var(--mono); color: var(--muted); background: var(--bg); border-radius: var(--r-sm); padding: 1px 5px; width: fit-content; -webkit-user-select: text; user-select: text; }
  .ex.block { display: block; margin: 3px 0; }
</style>
