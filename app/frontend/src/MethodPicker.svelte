<script lang="ts">
  // The method of a gRPC request: a field to type it in, and a list of the schema's methods under
  // their services, each with the kind of call. It replaced a datalist, which the window wrapped at
  // the slash, so long names showed only "package.Service/", and which could show no kinds.
  import Icon from './Icon.svelte'
  import { ui } from './lib/ui.svelte'
  import { errorText } from './lib/errors'
  import { RequestService } from '../bindings/github.com/githugt/flowi/app/internal/bridge'
  import type { GRPCInfo, GRPCMethod } from './lib/types'

  let { node, info }: { node: string; info: GRPCInfo } = $props()

  let open = $state(false)
  let query = $state('')
  let text = $state('')
  let focused = $state(false)
  let loading = $state(false)
  let active = $state(0)
  let list = $state<HTMLElement>()

  // The field shows the saved method unless it is being typed in.
  $effect(() => {
    if (!focused) text = info.block.Method
  })
  // An answer (methods or an error) ends the loading; other request events leave it.
  const answer = $derived(info.methods.map((m) => m.full).join('\n') + '\u0000' + (info.methodsError ?? ''))
  $effect(() => {
    void answer
    loading = false
  })

  const kinds: Record<string, [string, string]> = {
    unary: ['rpc-unary', 'Unary: one message each way'],
    server: ['rpc-server', 'Server streaming: the server answers with a stream'],
    client: ['rpc-client', 'Client streaming: a stream is sent'],
    bidi: ['rpc-bidi', 'Bidirectional streaming'],
  }
  const kindOf = (m: GRPCMethod) => (m.clientStream ? (m.serverStream ? 'bidi' : 'client') : m.serverStream ? 'server' : 'unary')

  const shown = $derived.by(() => {
    const q = query.trim().toLowerCase()
    return q ? info.methods.filter((m) => m.full.toLowerCase().includes(q)) : info.methods
  })
  const groups = $derived.by(() => {
    const out: { service: string; methods: GRPCMethod[] }[] = []
    for (const m of shown) {
      const g = out.find((x) => x.service === m.service)
      if (g) g.methods.push(m)
      else out.push({ service: m.service, methods: [m] })
    }
    return out
  })

  function load() {
    loading = true
    RequestService.LoadMethods(node)
  }
  function show() {
    open = true
    query = ''
    active = Math.max(0, info.methods.findIndex((m) => m.full === info.block.Method))
    if (!info.methods.length && !info.methodsError && !loading) load()
  }
  function pick(m: GRPCMethod) {
    open = false
    text = m.full
    RequestService.PickMethod(node, m.full)
  }
  function type(value: string) {
    text = query = value
    open = true
    active = 0
    RequestService.SetGRPC(node, { ...info.block, Method: value } as never)
  }
  function keys(e: KeyboardEvent) {
    if (e.key === 'Escape' && open) {
      open = false
    } else if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
      if (!open) show()
      else active = (active + (e.key === 'ArrowDown' ? 1 : -1) + shown.length) % Math.max(shown.length, 1)
      queueMicrotask(() => list?.querySelector('.on')?.scrollIntoView({ block: 'nearest' }))
    } else if (e.key === 'Enter' && open && shown[active]) {
      pick(shown[active])
    } else {
      return
    }
    e.preventDefault()
  }
  async function generate() {
    open = false
    try {
      const g = await RequestService.GenerateRequests(node)
      const skipped = g.skipped.length ? ` Skipped: ${g.skipped.join('; ')}` : ''
      ui.toast(`${g.created} request${g.created === 1 ? '' : 's'} made under grpc/.${skipped}`)
    } catch (e) {
      ui.toast(errorText(e), 'error')
    }
  }
  const sourceText = $derived(info.source === 'files' ? 'Using .proto files.' : 'Using server reflection.')
  const loadingText = $derived(info.source === 'files' ? 'Reading the .proto files…' : 'Asking the server for its methods…')
</script>

<svelte:window onpointerdown={() => (open = false)} />

<div class="picker" onpointerdown={(e) => e.stopPropagation()} role="presentation">
  <div class="field" class:focus={focused || open}>
    <input
      class="mono"
      value={text}
      placeholder="package.Service/Method"
      spellcheck="false"
      autocomplete="off"
      role="combobox"
      aria-expanded={open}
      aria-controls="grpc-methods"
      aria-label="Method"
      onfocus={() => (focused = true)}
      onblur={() => (focused = false)}
      onclick={() => !open && show()}
      oninput={(e) => type(e.currentTarget.value)}
      onkeydown={keys}
    />
    <button class="toggle" type="button" aria-label="Choose a method" title="Choose a method" onclick={() => (open ? (open = false) : show())}>▾</button>
  </div>
  {#if open}
    <div class="pop" id="grpc-methods" role="listbox" bind:this={list}>
      <div class="rows">
        {#if loading}
          <p class="note">{loadingText}</p>
        {:else if info.methodsError}
          <p class="note err">{info.methodsError}</p>
        {:else if !info.methods.length}
          <p class="note">No methods loaded.</p>
        {:else if !shown.length}
          <p class="note">No method matches “{query}”.</p>
        {/if}
        {#if !loading}
          {#each groups as g}
            <div class="service">{g.service}</div>
            {#each g.methods as m}
              {@const k = kinds[kindOf(m)]}
              <button
                type="button"
                role="option"
                aria-selected={m.full === info.block.Method}
                class:on={shown[active] === m}
                class:current={m.full === info.block.Method}
                title={m.input ? `${m.input} → ${m.output}` : m.full}
                onpointerenter={() => (active = shown.indexOf(m))}
                onclick={() => pick(m)}
              >
                <span class="kind" title={k[1]}><Icon name={k[0]} size={15} /></span>
                <span class="name">{m.name}</span>
                {#if m.full === info.block.Method}<span class="tick" aria-hidden="true">✓</span>{/if}
              </button>
            {/each}
          {/each}
        {/if}
      </div>
      <div class="foot">
        <span class="muted">{sourceText}</span>
        <button type="button" class="icon" title="Reload methods" aria-label="Reload methods" onclick={load}><Icon name="refresh" size={14} /></button>
        <span class="grow"></span>
        <button type="button" class="link" disabled={!info.methods.length} title="Make a request for each method, under grpc/" onclick={generate}>Generate collection</button>
      </div>
    </div>
  {/if}
</div>

<style>
  .picker { position: relative; flex: 1; min-width: 0; }
  .field {
    display: flex; align-items: center; height: 100%; min-height: 30px;
    background: var(--bg); border: 1px solid var(--line); border-radius: var(--r-md);
  }
  .field.focus { border-color: var(--accent); }
  input { flex: 1; min-width: 0; background: none; border: 0; outline: none; color: var(--text); padding: 5px 8px; font: inherit; -webkit-user-select: text; user-select: text; }
  input.mono { font-family: var(--mono); font-size: var(--fs-sm); }
  .toggle { background: none; border: 0; color: var(--muted); padding: 0 8px; align-self: stretch; }
  .toggle:hover { color: var(--text); }
  .pop {
    position: absolute; top: calc(100% + 4px); left: 0; z-index: 30; width: max(100%, 380px);
    display: flex; flex-direction: column; background: var(--panel); border: 1px solid var(--line);
    border-radius: var(--r-lg); box-shadow: var(--shadow);
  }
  .rows { max-height: 380px; overflow: auto; padding: 4px; }
  .service { padding: 8px 8px 4px; font-size: var(--fs-xs); color: var(--muted); font-family: var(--mono); overflow-wrap: anywhere; }
  .rows button {
    width: 100%; display: flex; align-items: center; gap: 8px; padding: 5px 8px 5px 12px; text-align: left;
    background: none; border: 0; border-radius: var(--r-sm); color: var(--text); font: inherit;
  }
  .rows button.on { background: color-mix(in srgb, var(--accent) 16%, transparent); }
  .rows button.current .name { font-weight: 600; }
  .kind { display: grid; place-items: center; color: var(--accent); flex: none; }
  .name { font-family: var(--mono); font-size: var(--fs-sm); overflow-wrap: anywhere; }
  .tick { margin-left: auto; color: var(--accent); }
  .note { margin: 0; padding: 10px 12px; color: var(--muted); }
  .note.err { color: var(--fail); white-space: pre-wrap; }
  .foot { display: flex; align-items: center; gap: 6px; padding: 6px 8px; border-top: 1px solid var(--line); font-size: var(--fs-sm); }
  .grow { flex: 1; }
  .muted { color: var(--muted); }
  .icon { display: grid; place-items: center; background: none; border: 0; color: var(--muted); padding: 3px; border-radius: var(--r-sm); }
  .icon:hover { color: var(--text); background: color-mix(in srgb, var(--accent) 12%, transparent); }
  .link { background: none; border: 0; color: var(--accent); padding: 3px 4px; font: inherit; }
  .link:disabled { opacity: 0.4; }
</style>
