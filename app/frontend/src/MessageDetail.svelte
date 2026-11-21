<script lang="ts">
  // An opened message: its facts as chips, its headers, and its body as a tree, as text or as hex.
  // Before: the detail was a flat text block with no tree, no hex and no partition or offset.
  import Button from './Button.svelte'
  import Code from './Code.svelte'
  import Dropdown from './Dropdown.svelte'
  import JsonNode from './JsonNode.svelte'
  import { copy } from './lib/clip'
  import { sizeText } from './lib/bytesize'
  import { bodyBytes, copyChoices, copyName, copyText, defaultMode, detailModes, hexView, metaChips, parseTree, type CopyAs, type DetailMessage, type DetailMode } from './lib/messagedetail'

  let { message }: { message: DetailMessage } = $props()

  const modes = $derived(detailModes(message))
  let picked = $state<DetailMode | null>(null)
  // The mode chosen, or the message's own while none was: a row opens as a tree for JSON and as hex for bytes.
  const mode = $derived(picked && modes.includes(picked) ? picked : defaultMode(message))
  const tree = $derived(mode === 'tree' ? parseTree(message.body) : null)
  const hex = $derived(mode === 'hex' ? hexView(message) : null)
  const chips = $derived(metaChips(message))
  const label: Record<DetailMode, string> = { tree: 'Tree', text: 'Text', hex: 'Hex' }
  let selected = $state('')
  // Picking a node marks it and shows its path; picking it again lets go. The feed has its own filter, so the tree has no search.
  const pick = (path: string) => (selected = selected === path ? '' : path)
  const copyAs = (as: string) => void copy(copyText(message, as as CopyAs), copyName(as as CopyAs))
</script>

<div class="det">
  <div class="bar">
    {#if modes.length > 1}
      <div class="modes" role="group" aria-label="Show the body as">
        {#each modes as m (m)}
          <Button small pressed={mode === m} onclick={() => (picked = m)}>{label[m]}</Button>
        {/each}
      </div>
    {/if}
    <span class="grow"></span>
    <Button small onclick={() => copyAs('body')} title="Copy the body">Copy</Button>
    <Dropdown title="Copy in another form" label="Copy as…" options={copyChoices(message)} value="" onpick={copyAs} />
  </div>
  {#if chips.length}
    <div class="chips">
      {#each chips as c (c.label)}<span class="chip" title={c.label}><span class="cl">{c.label}</span> {c.value}</span>{/each}
      <span class="chip" title="The body's size"><span class="cl">Size</span> {sizeText(bodyBytes(message))}</span>
    </div>
  {/if}
  {#if message.headers?.length}
    <table class="hdrs"><tbody>{#each message.headers as h}<tr><td class="hk">{h.key}</td><td>{h.value}</td></tr>{/each}</tbody></table>
  {/if}
  {#if tree}
    <div class="tree"><JsonNode value={tree.value} name="$" path="$" {selected} query="" onpick={pick} /></div>
    {#if selected}<p class="note">Selected <code>{selected}</code></p>{/if}
  {:else if hex}
    {#if hex.text}<pre class="hex">{hex.text}</pre>{:else}<p class="note">There are no bytes to show.</p>{/if}
    {#if hex.shown < hex.total}<p class="note">Showing the first {sizeText(hex.shown)} of {sizeText(hex.total)}. Copy as… Hex dump has the same part.</p>{/if}
  {:else}
    <Code text={message.body} />
  {/if}
</div>

<style>
  .det { margin: 0 0 6px; padding: 10px 16px 12px 40px; border-radius: 0 0 var(--r-md) var(--r-md); background: var(--sunk); }
  .bar { display: flex; align-items: center; gap: 8px; margin: 0 0 8px; }
  .modes { display: flex; gap: 4px; }
  .grow { flex: 1; }
  .chips { display: flex; flex-wrap: wrap; gap: 6px; margin: 0 0 8px; }
  .chip { padding: 0 8px; border-radius: var(--r-pill); border: 1px solid var(--line); font: var(--fs-xs) var(--mono); line-height: 18px; word-break: break-all; }
  .cl { color: var(--muted); }
  .hdrs { border-collapse: collapse; margin: 0 0 8px; font: var(--fs-xs) var(--mono); }
  .hdrs td { padding: 1px 12px 1px 0; vertical-align: top; word-break: break-all; }
  .hk { color: var(--muted); white-space: nowrap; }
  .tree { padding: 8px; background: var(--bg); border: 1px solid var(--line); border-radius: var(--r-md); font: var(--fs-sm)/1.45 var(--mono); overflow: auto; max-height: 420px; }
  .hex { margin: 0; padding: 8px; background: var(--bg); border: 1px solid var(--line); border-radius: var(--r-md); font: var(--fs-xs)/1.45 var(--mono); overflow: auto; max-height: 420px; -webkit-user-select: text; user-select: text; }
  .note { margin: 6px 0 0; color: var(--muted); font-size: var(--fs-xs); }
  .note code { font: var(--fs-xs) var(--mono); }
</style>
