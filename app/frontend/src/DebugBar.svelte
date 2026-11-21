<script lang="ts">
  import Button from './Button.svelte'
  import Icon from './Icon.svelte'
  import { isEnter } from './lib/enterkey'
  // The open flow's breakpoints, and while a debug session is paused, a bar to continue, step or stop it and to change
  // a variable the next part starts with. Breakpoints belong to the open flow, not to the file.
  import { FlowService } from '../bindings/github.com/githugt/flowi/app/internal/bridge'
  import type { FlowView } from './lib/types'

  let { view }: { view: FlowView } = $props()
  const node = $derived(view.node)
  const debug = $derived(view.debug)
  let name = $state('')
  let text = $state('')
  let editing = $state<Record<string, string>>({})

  function setVar(n: string, value: string) {
    if (!n) return
    FlowService.SetDebugVar(node, n, value)
    delete editing[n]
    if (n === name) {
      name = ''
      text = ''
    }
  }
</script>

{#if debug.paused}
  <div class="bar" role="region" aria-label="Debugger">
    <div class="line">
      <strong>Paused before {debug.paused}</strong>
      <Button small variant="go" onclick={() => FlowService.Continue(node)} title="Run to the next breakpoint, or to the end">Continue</Button>
      <Button small onclick={() => FlowService.Step(node)} title="Run this step, then pause before the next">Step</Button>
      <Button small onclick={() => FlowService.StopDebugging(node)}>Stop</Button>
      <span class="hint">Change a variable, then continue: the next steps start with it.</span>
    </div>
    <div class="vars">
      {#each debug.vars as v (v.name)}
        <label class="var">
          <code>{v.name}</code>
          <input
            value={editing[v.name] ?? v.value}
            spellcheck="false"
            oninput={(e) => (editing[v.name] = e.currentTarget.value)}
            onkeydown={(e) => isEnter(e) && setVar(v.name, editing[v.name] ?? v.value)}
            onblur={() => editing[v.name] !== undefined && editing[v.name] !== v.value && setVar(v.name, editing[v.name])}
          />
        </label>
      {/each}
      <span class="var new">
        <input placeholder="new variable" bind:value={name} spellcheck="false" aria-label="Name of a new variable" />
        <input placeholder="value (7, true, text)" bind:value={text} spellcheck="false" aria-label="Its value" onkeydown={(e) => isEnter(e) && setVar(name, text)} />
        <Button small disabled={!name} onclick={() => setVar(name, text)}>Set</Button>
      </span>
    </div>
  </div>
{:else if debug.breaks.length}
  <div class="strip" role="region" aria-label="Breakpoints">
    <span>{debug.breaks.length} breakpoint{debug.breaks.length > 1 ? 's' : ''}:</span>
    {#each debug.breaks as id (id)}
      <button class="chip" title="Remove this breakpoint" onclick={() => FlowService.ToggleBreak(node, id)}>{id} <Icon name="close" size={10} /></button>
    {/each}
    <span class="hint">Choose Debug in the Run menu to pause before each.</span>
  </div>
{/if}

<style>
  .bar { flex: none; display: flex; flex-direction: column; gap: 6px; padding: 8px 12px; border-bottom: 1px solid var(--line); background: color-mix(in srgb, var(--warn) 9%, var(--panel)); }
  .line { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
  .vars { display: flex; flex-wrap: wrap; gap: 6px 12px; align-items: center; }
  .var { display: flex; align-items: center; gap: 4px; font-size: var(--fs-sm); }
  .var input { width: 150px; padding: 2px 6px; font: var(--fs-sm) var(--mono); }
  .var.new input { width: 130px; }
  .hint { color: var(--muted); font-size: var(--fs-xs); }
  button { background: var(--raised); color: var(--text); border: 0; box-shadow: inset 0 0 0 1px var(--line); border-radius: var(--r-md); padding: 0 12px; min-height: var(--ctl-h-sm); font: inherit; font-weight: 500; }
  button:disabled { opacity: var(--op-disabled); }
  button:hover:not(:disabled) { background: var(--raised-3); }
  .strip { flex: none; display: flex; align-items: center; gap: 6px; flex-wrap: wrap; padding: 4px 12px; border-bottom: 1px solid var(--line); font-size: var(--fs-sm); color: var(--muted); }
  .chip { border-radius: var(--r-pill); padding: 0 10px; font-size: var(--fs-xs); color: var(--fail); border-color: var(--fail); }
</style>
