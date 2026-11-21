<script lang="ts">
  import Checkbox from './Checkbox.svelte'
  import Menu, { type MenuItem } from './Menu.svelte'
  import { anchorAt, type MenuAt } from './lib/menuanchor'
  import Modal from './Modal.svelte'
  import Button from './Button.svelte'
  import { errorText } from './lib/errors'
  import { FlowService } from '../bindings/github.com/githugt/flowi/app/internal/bridge'
  import type { FlowView } from './lib/types'
  import Icon from './Icon.svelte'
  import { ui } from './lib/ui.svelte'
  import { firstFailed } from './lib/retry'
  import { app } from './lib/app.svelte'
  import { runNeedsConfirm, writesQuestion, writesUnknownQuestion } from './lib/protect'
  import { inputsToSend, lastInputs, rememberInputs } from './lib/runinputs'
  import { runCommand } from './lib/cliline'
  import { copy } from './lib/clip'

  let { view }: { view: FlowView } = $props()
  const node = $derived(view.node)
  let open = $state(false)
  let menuAt = $state<MenuAt>({ x: 0, y: 0 })
  let asking = $state<null | { scope: string }>(null)
  let values = $state<Record<string, string>>({})
  // Inputs sent as the empty text on purpose.
  let empty = $state<string[]>([])
  let error = $state('')
  let cancelling = $state(false)
  $effect(() => {
    if (!view.run.running) cancelling = false
  })

  // After a failure, one click runs again from the step that failed.
  const failed = $derived(firstFailed(view.run.statuses, view.diagram.units.flatMap((u) => u.nodes)))
  async function retry() {
    if (view.selected !== failed) await FlowService.Select(node, failed)
    await launch('from', false)
  }

  const inputs = $derived(view.form.flow.inputs)
  const needsInputs = $derived(inputs.some((i) => i.value === ''))

  async function launch(scope: string, force: boolean) {
    open = false
    error = ''
    if (inputs.length && (force || needsInputs)) {
      const kept = lastInputs(node, inputs.map((i) => i.key))
      values = { ...kept.values }
      empty = kept.empty
      asking = { scope }
      return
    }
    await go(scope, {})
  }

  // Skipped steps whose values a later step needs are listed before the run starts, not found out halfway.
  async function go(scope: string, given: Record<string, string>) {
    const env = app.tree?.selected ?? ''
    if (runNeedsConfirm(env, app.tree?.protected ?? [])) {
      // Asks only when the flow changes data, and names the steps; when that cannot be read it asks as before.
      let text = ''
      try {
        const lines = (await FlowService.Writes(node)) ?? []
        if (lines.length) text = writesQuestion(env, lines)
      } catch {
        text = writesUnknownQuestion(env)
      }
      if (text) {
        ui.confirm(`Run in ${env}?`, text, () => go2(scope, given), 'Run')
        return
      }
    }
    await go2(scope, given)
  }
  async function go2(scope: string, given: Record<string, string>) {
    const broken = view.skips.problems.filter((p) => p.required)
    if (broken.length) {
      ui.confirm('Run with skipped steps?', broken.map((p) => p.text).join('\n\n'), () => start(scope, given), 'Run anyway')
      return
    }
    await preflight(scope, given)
  }

  // What the environment lacks is listed before the first request goes out, not found out halfway. A failed check
  // never stops the run: it is a warning only.
  async function preflight(scope: string, given: Record<string, string>) {
    let lines: string[] = []
    try {
      lines = (await FlowService.Preflight(node)) ?? []
    } catch {
      // the run itself reports what is wrong
    }
    if (lines.length) {
      ui.confirm('The environment may be missing something', lines.join('\n'), () => start(scope, given), 'Run anyway')
      return
    }
    await start(scope, given)
  }

  async function start(scope: string, given: Record<string, string>) {
    const filled = inputsToSend(given, empty)
    try {
      if (scope === 'all') await FlowService.Run(node, filled)
      else if (scope === 'debug') await FlowService.Debug(node, filled)
      else await FlowService.RunPart(node, scope, filled)
    } catch (e) {
      error = errorText(e)
    }
  }

  async function submit() {
    const scope = asking!.scope
    asking = null
    rememberInputs(node, values, empty)
    await go(scope, values)
  }

  /** Puts the cursor in the first box when the dialog opens. */
  function focusFirst(el: HTMLInputElement, first: boolean) {
    if (first) queueMicrotask(() => el.focus())
  }

  // The command line that does what Run does: the saved flow, the selected environment, the inputs last given and the
  // last run's seed. A command line runs the file as saved, so unsaved edits are said not to be in it.
  function copyCommand() {
    open = false
    const kept = lastInputs(node, inputs.map((i) => i.key))
    const line = runCommand({ path: view.path, collection: view.collection, env: app.tree?.selected ?? '', inputs: inputsToSend(kept.values, kept.empty), seed: view.run.seed ?? '' })
    void copy(line, view.dirty ? 'the flowi command (it runs the saved flow, not your unsaved edits)' : 'the flowi command')
  }

  // What the menu offers besides ways to run, below the line.
  const tools = $derived([
    { label: 'Copy as flowi command', title: 'The flowi run command for this flow, environment and inputs', disabled: !view.path, run: copyCommand },
    { label: 'Clear results', title: '', disabled: false, run: () => { open = false; FlowService.ClearResults(node) } },
  ])

  const choices: [string, string, boolean][] = [
    ['Run', 'all', false],
    ['Run with inputs…', 'all', true],
    ['Run only the selected step', 'only', false],
    ['Run until the selected step', 'until', false],
    ['Run from the selected step', 'from', false],
    ['Debug: pause at breakpoints', 'debug', false],
  ]
  const menuItems = $derived<MenuItem[]>([
    ...choices.map(([label, scope, force]) => ({ label, onselect: () => launch(scope, force), disabled: (scope === 'only' || scope === 'until' || scope === 'from') && !view.selected })),
    ...tools.map((t, i) => ({ label: t.label, title: t.title, disabled: t.disabled, onselect: t.run, separated: i === 0 })),
  ])
</script>

<div class="run">
  {#if view.run.running}
    <!-- Before: Cancel looked the same until the engine's grace period ended, so the click seemed to do nothing. -->
    <Button variant="primary" disabled={cancelling} onclick={() => { cancelling = true; FlowService.Cancel(node) }}>{cancelling ? 'Cancelling…' : 'Cancel'}</Button>
  {:else}
    {#if failed}<button class="retry" title={`Run again from ${failed}, the step that failed`} onclick={retry}>Retry from {failed}</button>{/if}
    <button class="primary main" onclick={() => launch('all', false)}>Run</button>
    <button class="more" aria-label="More ways to run" aria-haspopup="menu" aria-expanded={open} onclick={(e) => { menuAt = anchorAt(e.currentTarget.getBoundingClientRect(), { end: true }); open = !open }}><Icon name="chevd" size={14} /></button>
  {/if}
  {#if open}<Menu at={menuAt} minWidth={230} label="More ways to run" items={menuItems} onclose={() => (open = false)} />{/if}
</div>
{#if error}<span class="err">{error}</span>{/if}

{#if asking}
  <Modal title="Run flow" width={360} onclose={() => (asking = null)} onsubmit={() => submit()}>
      {#each inputs as i (i.key)}
        <div class="inrow">
          <label for={`in-${i.key}`}>{i.key}</label>
          <input id={`in-${i.key}`} placeholder={i.value} bind:value={values[i.key]} disabled={empty.includes(i.key)} spellcheck="false" use:focusFirst={i.key === inputs[0]?.key} />
          <div class="emptybox" title="Send this input as the empty text, not its default">
            <Checkbox checked={empty.includes(i.key)} onchange={(v) => (empty = v ? [...empty, i.key] : empty.filter((k) => k !== i.key))}>∅ send empty</Checkbox>
          </div>
        </div>
      {/each}
    {#snippet actions()}
      <Button type="button" onclick={() => (asking = null)}>Cancel</Button>
      <Button variant="primary" type="submit">Run</Button>
    {/snippet}
  </Modal>
{/if}

<style>
  .run { position: relative; display: flex; gap: 0; }
  .retry { margin-right: 8px; border-color: color-mix(in srgb, var(--fail) 55%, var(--line)); }
  button {
    background: transparent;
    color: var(--text);
    border: 1px solid var(--line);
    border-radius: var(--r-md);
    padding: 3px 12px;
    font: inherit;
  }
  button:hover:not(:disabled) { background: var(--raised-3); }
  button:disabled { opacity: var(--op-disabled); }
  /* the same strong blue as a request's Send, not the paler accent */
  .primary { background: var(--send-fill); color: var(--on-send); border: 0; font-weight: 600; box-shadow: var(--send-edge), 0 2px 6px var(--send-glow); }
  .primary:hover:not(:disabled) { filter: brightness(1.1); }
  /* One joined pair, the same size and glow as a request's Send. */
  .run button { min-height: var(--ctl-h); padding: 0 12px; border: 0; }
  .run .primary { border-radius: var(--r-md) 0 0 var(--r-md); }
  .run .primary:only-child { border-radius: var(--r-md); }
  .run .more { display: inline-flex; align-items: center; justify-content: center; padding: 0 8px; border-radius: 0 var(--r-md) var(--r-md) 0; background: var(--send-fill); color: var(--on-send); box-shadow: var(--send-edge), 0 2px 6px var(--send-glow); border-left: 1px solid var(--shade); }
  .run .more:hover:not(:disabled) { background: var(--send-fill); filter: brightness(1.1); }
  .err { color: var(--fail); font-size: var(--fs-sm); }
  .inrow { display: flex; flex-direction: column; gap: 3px; }
  .emptybox { display: flex; align-items: center; gap: 4px; }
  .inrow label { display: flex; flex-direction: column; gap: 3px; color: var(--muted); font-size: var(--fs-xs); }
  .inrow input {
    background: var(--bg);
    color: var(--text);
    border: 1px solid var(--line);
    border-radius: var(--r-md);
    padding: 5px 8px;
    font: inherit;
    -webkit-user-select: text;
    user-select: text;
  }
</style>
