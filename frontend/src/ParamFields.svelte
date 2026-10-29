<script lang="ts">
  // The parameters of a step as form fields: a template or expression field, a choice, a switch. A list or mapping
  // is edited in the file. Used by the side inspector and by the rows of a card.
  import { FlowService } from '../bindings/github.com/githugt/flowi/internal/bridge'
  import type { FlowView, Param } from './lib/types'
  import TemplateField from './TemplateField.svelte'
  import { completeFor, exprHint, helpFor, vocabularyFor } from './lib/flowfields'

  // names, when given, limits the fields to those parameters.
  let { view, names }: { view: FlowView; names?: string[] } = $props()
  const node = $derived(view.node)
  const step = $derived(view.form.step)
  const shown = $derived((step?.params ?? []).filter((p: Param) => !names || names.includes(p.name)))
</script>

{#if step}
  {#each shown as p (p.name)}
    {#if p.complex}
      <div class="field">
        <span class="label">{p.name}</span>
        <em class="muted">(a list or mapping: edit it in the file)</em>
      </div>
    {:else if p.kind === 'bool'}
      <label class="check">
        <input type="checkbox" checked={p.value === 'true'} onchange={(e) => FlowService.SetParam(node, p.name, String(e.currentTarget.checked))} />
        <span>{p.name}{p.required ? ' *' : ''}</span>
      </label>
    {:else if p.kind === 'enum'}
      <label class="field">
        <span class="label">{p.name}{p.required ? ' *' : ''}</span>
        <select value={p.value} onchange={(e) => FlowService.SetParam(node, p.name, e.currentTarget.value)}>
          <option value=""></option>
          {#each p.enum ?? [] as o}<option value={o}>{o}</option>{/each}
        </select>
      </label>
    {:else}
      <TemplateField
        label={p.name + (p.required ? ' *' : '')}
        help={helpFor(step.type, p)}
        placeholder={p.kind === 'expr' ? 'nodes.login.status == 200' : p.doc}
        value={p.value}
        mono={p.kind === 'text' || p.kind === 'expr'}
        oninput={(v) => FlowService.SetParam(node, p.name, v)}
        suggest={completeFor(node)}
        vocabulary={p.kind === 'expr' ? vocabularyFor(node, 'flow.check') : undefined}
      />
      {#if p.kind === 'expr'}<p class="hint">{p.doc || 'True or false, written without braces.'} {exprHint}</p>{/if}
    {/if}
  {/each}
{/if}

<style>
  .field { display: flex; flex-direction: column; gap: 3px; }
  .label { color: var(--muted); font-size: var(--fs-xs); }
  .check { display: flex; align-items: center; gap: 6px; }
  .hint, .muted { color: var(--muted); font-size: var(--fs-sm); margin: 0; }
  select { background: var(--bg); color: var(--text); border: 1px solid var(--line); border-radius: var(--r-md); padding: 5px 6px; font: inherit; }
</style>
