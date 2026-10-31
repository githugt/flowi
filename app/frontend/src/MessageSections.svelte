<script lang="ts">
  // The sections only message (async) and gRPC requests have.
  import { RequestService } from '../bindings/github.com/githugt/flowi/app/internal/bridge'
  import { ui } from './lib/ui.svelte'
  import type { AsyncBlock, GRPCBlock, KV, RequestView } from './lib/types'
  import AreaField from './AreaField.svelte'
  import CodeArea from './CodeArea.svelte'
  import TextField from './TextField.svelte'
  import TemplateField from './TemplateField.svelte'
  import { requestVocabulary } from './lib/vocabulary'
  import KVRows from './KVRows.svelte'
  import EmptyState from './EmptyState.svelte'
  import { parseDuration, showDuration } from './lib/duration'
  import { mergeBinding, optionNames } from './lib/binding'

  let { view, section }: { view: RequestView; section: string } = $props()
  const node = $derived(view.node)
  const a = $derived(view.async)
  const g = $derived(view.grpc)

  let timeoutBad = $state(false)

  type Receive = { Count: number; Timeout: number; Match: string }
  const receive = $derived<Receive | null>(a ? a.block.Receive : (g?.block.Receive ?? null))

  // ---- writes: the block comes from the latest view, with one change
  function putAsync(change: (b: AsyncBlock) => void) {
    const b = structuredClone($state.snapshot(a!.block)) as AsyncBlock
    change(b)
    RequestService.SetAsync(node, b as never)
  }
  function putGRPC(change: (b: GRPCBlock) => void) {
    const b = structuredClone($state.snapshot(g!.block)) as GRPCBlock
    change(b)
    RequestService.SetGRPC(node, b as never)
  }
  const putReceive = (change: (r: Receive) => void) => {
    const edit = (b: { Receive: Receive | null }) => {
      b.Receive ??= { Count: 0, Timeout: 0, Match: '' }
      change(b.Receive)
    }
    if (a) putAsync(edit)
    else putGRPC(edit)
  }
  function setReceiveOn(on: boolean) {
    const edit = (b: { Receive: Receive | null }) => (b.Receive = on ? (b.Receive ?? { Count: 0, Timeout: 0, Match: '' }) : null)
    if (a) putAsync(edit)
    else putGRPC(edit)
  }

  // ---- the message
  const msg = $derived(a ? (a.block.Send?.Body ?? { JSON: '', Text: '' }) : (g?.block.Body ?? { JSON: '', Text: '' }))
  const isJSON = $derived(a ? !(msg.Text && !msg.JSON) : true)
  const msgText = $derived(msg.JSON || msg.Text)
  function setMessage(text: string, json = isJSON) {
    const body = json ? { JSON: text, Text: '' } : { JSON: '', Text: text }
    if (a) putAsync((b) => ((b.Send ??= { Body: { JSON: '', Text: '', File: '', Form: null, Multipart: null }, Headers: null }), (b.Send.Body = { ...b.Send.Body, ...body })))
    else putGRPC((b) => (b.Body = { ...b.Body, ...body }))
  }
  async function prettify() {
    const [pretty, ok] = (await RequestService.PrettyJSON(msgText)) as unknown as [string, boolean]
    if (ok) setMessage(pretty)
    else ui.toast('The message is not valid JSON, so it cannot be formatted. A template must be inside a string to count as JSON.', 'error')
  }

  // ---- credentials and binding share the binding list
  const AUTH = 'auth'
  const binding = $derived(a?.block.Binding ?? [])
  const credentialNames = $derived(a?.credentials ?? [])
  const options = $derived(binding.filter((kv) => kv.Key !== AUTH && !credentialNames.includes(kv.Key)))
  const credentials = $derived(binding.filter((kv) => credentialNames.includes(kv.Key)))
  const authMode = $derived(binding.find((kv) => kv.Key === AUTH)?.Value || 'default')
  // Azure Service Bus: look at the queue, or receive and remove what is kept.
  const readMode = $derived(options.find((kv) => kv.Key === 'read')?.Value || 'lock')
  function setReadMode(mode: string) {
    putBinding([...options.filter((kv) => kv.Key !== 'read'), { Key: 'read', Value: mode, Description: '' }], authMode, credentials)
  }
  // Each key is written once, and an option named like a sign-in value goes to Credentials (and says so),
  // so the two tabs never drop or double each other's rows.
  function putBinding(opts: KV[], mode: string, creds: KV[]) {
    const { list, moved } = mergeBinding(opts, a?.cloud ? mode : 'default', creds, credentialNames)
    // Said once, when the row first moves: typing its value goes on updating the credential, without a toast for each key.
    const fresh = moved.filter((k) => !creds.some((c) => c.Key === k))
    if (fresh.length) ui.toast(`${fresh.join(', ')} ${fresh.length > 1 ? 'are sign-in values' : 'is a sign-in value'}, so ${fresh.length > 1 ? 'they are' : 'it is'} in the Credentials tab.`)
    putAsync((b) => (b.Binding = list))
  }
  // The names the Binding table suggests: the protocol's, without its sign-in values.
  const suggestedNames = $derived(optionNames(a?.bindings ?? [], credentialNames))
  const setCredential = (name: string, value: string) => {
    const others = credentials.filter((c) => c.Key !== name)
    putBinding(options, authMode, value ? [...others, { Key: name, Value: value, Description: '' }] : others)
  }

  // ---- gRPC schema files
  const lines = (list: string[] | null) => (list ?? []).join('\n')
  const splitLines = (t: string) => t.split('\n').map((l) => l.trim()).filter(Boolean)
</script>

{#if section === 'Message'}
  <div class="line">
    {#if a}
      <label class="radio">Format
        <select value={isJSON ? 'json' : 'text'} onchange={(e) => setMessage(msgText, e.currentTarget.value === 'json')}>
          <option value="json">JSON</option><option value="text">Text</option>
        </select>
      </label>
    {/if}
    <button disabled={a ? !isJSON : false} onclick={prettify}>Prettify</button>
  </div>
  <CodeArea value={msgText} lang={isJSON ? 'json' : 'text'} oninput={(t) => setMessage(t)} empty={{ icon: 'body', title: 'No message', hint: 'Click to write the message to send.' }} />
{:else if section === 'Receive' && !receive}
  <EmptyState icon="stream" title="Not waiting for messages" hint="This request only sends. Wait for messages to check the replies or events that follow." compact>
    <button onclick={() => setReceiveOn(true)}>Wait for messages</button>
  </EmptyState>
{:else if section === 'Receive'}
  <label class="check"><input type="checkbox" checked={receive !== null} onchange={(e) => setReceiveOn(e.currentTarget.checked)} /> Wait for messages</label>
  {#if receive}
    <TextField label="Messages (empty: 1)" value={receive.Count ? String(receive.Count) : ''} oninput={(t) => putReceive((r) => (r.Count = Math.max(0, parseInt(t) || 0)))} />
    <TextField
      label="Timeout (empty: 30s)"
      value={showDuration(receive.Timeout)}
      placeholder="30s"
      oninput={(t) => {
        const ns = parseDuration(t)
        timeoutBad = ns === null
        if (ns !== null) putReceive((r) => (r.Timeout = ns))
      }}
    />
    {#if timeoutBad}<p class="err">Write the timeout as a duration like 500ms, 30s or 2m.</p>{/if}
    <TemplateField label="Only if" help="message.filter" placeholder={'body.type == "order"'} value={receive.Match} oninput={(t) => putReceive((r) => (r.Match = t))} vocabulary={requestVocabulary('message.filter', node, view.response, view.kind)} />
    <p class="muted">Stops after this many messages, or fails when the timeout passes. Only messages the filter accepts count.</p>
  {/if}
{:else if section === 'Credentials' && a}
  {#if a.cloud}
    <label class="f"><span>Sign in</span>
      <select value={authMode} onchange={(e) => putBinding(options, e.currentTarget.value, credentials)}>
        <option value="default">The provider's own credential lookup</option>
        <option value="explicit">Credentials written here</option>
      </select>
    </label>
  {/if}
  {#if a.cloud && authMode !== 'explicit'}
    <EmptyState icon="auth" title="The provider's own credentials" hint="Found the provider's usual way (its environment variables, profile or signed-in session). Pick “Credentials written here” above to set them for this request." compact />
  {:else}
    {#each credentialNames as name}
      <TextField label={name} mono placeholder={'{{secret.' + name + '}}'} value={credentials.find((c) => c.Key === name)?.Value ?? ''} oninput={(t) => setCredential(name, t)} />
    {/each}
    <p class="muted">Keep credentials out of the file: use <code>{'{{secret.name}}'}</code>.</p>
  {/if}
{:else if section === 'Binding' && a}
  {#if a.block.Protocol === 'servicebus'}
    <label class="f"><span>Reading the queue</span>
      <select value={readMode} onchange={(e) => setReadMode(e.currentTarget.value)}>
        <option value="peek">Only look: leave every message where it is (recommended for testing)</option>
        <option value="lock">Receive: remove the messages it keeps, give the others back</option>
      </select>
    </label>
    {#if readMode === 'lock'}
      <p class="muted">Receiving removes the kept messages from the queue. Every other message it sees counts as a delivery, and a message delivered too often goes to the dead-letter queue.</p>
    {:else}
      <p class="muted">Nothing is locked, removed or counted as delivered. A later run sees the same messages again.</p>
    {/if}
  {/if}
  {#if options.length}<p class="muted">The protocol's own options. Known names: {suggestedNames.join(', ') || 'none'}.{credentialNames.length ? ' Sign-in values are in the Credentials tab.' : ''}</p>{/if}
  <KVRows rows={options} keyLabel="option" suggestKeys={suggestedNames} onchange={(rows) => putBinding(rows, authMode, credentials)}
    empty={{ icon: 'binding', title: 'No protocol options', hint: suggestedNames.length ? `The protocol's own options, such as ${suggestedNames.join(', ')}.` : 'This protocol takes no options of its own.', add: 'Add option' }} />
{:else if section === 'Proto' && g}
  <p class="muted">Schema: {g.source === 'files' ? 'the .proto files below' : 'the server’s reflection (no files listed)'}.</p>
  <AreaField label=".proto files, relative to the collection (one per line)" rows={4} value={lines(g.block.Proto)} oninput={(t) => putGRPC((b) => (b.Proto = splitLines(t)))}
    empty={{ icon: 'proto', title: 'Using reflection', hint: 'Click to list .proto files instead of asking the server.' }} />
  <AreaField label="Import roots (one per line)" rows={3} value={lines(g.block.Import)} oninput={(t) => putGRPC((b) => (b.Import = splitLines(t)))} />
  <label class="check" title="Send the call as an HTTP POST to a gRPC-Web proxy (Envoy, a gateway) at the http(s) address of the server. Needs the .proto files; a method that streams from the client cannot be called.">
    <input type="checkbox" checked={g.block.Web ?? false} onchange={(e) => putGRPC((b) => (b.Web = e.currentTarget.checked))} />
    <span>Call through gRPC-Web (an http:// or https:// server)</span>
  </label>
{/if}

<style>
  .line { display: flex; gap: 12px; align-items: center; }
  .radio { display: flex; gap: 6px; align-items: center; }
  .check { display: flex; align-items: center; gap: 6px; }
  .f { display: flex; flex-direction: column; gap: 3px; color: var(--muted); font-size: var(--fs-xs); }
  .muted { color: var(--muted); font-size: var(--fs-sm); margin: 0; }
  .err { color: var(--fail); font-size: var(--fs-sm); margin: 0; }
  select, button { background: var(--bg); color: var(--text); border: 1px solid var(--line); border-radius: var(--r-md); padding: 4px 10px; font: inherit; }
  button { background: transparent; }
  button:disabled { opacity: 0.4; }
  code { font-family: var(--mono); }
</style>
