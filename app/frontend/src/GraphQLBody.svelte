<script lang="ts">
  // The body of a GraphQL request: the query, its variables, and which operation runs when the query has several.
  // Before: a GraphQL request was a JSON body with the query inside a string, with no colour, no variables box and
  // no way to see what kind of operation it was.
  import { RequestService } from '../bindings/github.com/githugt/flowi/app/internal/bridge'
  import CodeArea from './CodeArea.svelte'
  import Dropdown from './Dropdown.svelte'
  import Field from './Field.svelte'
  import TextInput from './TextInput.svelte'
  import TemplateField from './TemplateField.svelte'
  import { parseCount } from './lib/count'
  import { parseDuration, showDuration } from './lib/duration'
  import { requestVocabulary } from './lib/vocabulary'
  import type { DropOption, RequestView } from './lib/types'

  let { view, node }: { view: RequestView; node: string } = $props()

  const body = $derived(view.current.Body.GraphQL ?? { Query: '', Variables: '', Operation: '' })
  const info = $derived(view.graphql)
  // Naming an operation is only needed when the query has more than one; an operation already named stays pickable.
  const operations = $derived<DropOption[]>(
    (info?.operations ?? []).filter((op) => op.name).map((op) => ({ value: op.name, label: op.name, detail: op.kind })),
  )
  const showOperation = $derived(operations.length > 1 || (!!body.Operation && operations.length > 0))
  const kindWord = $derived(info?.kind ? info.kind[0].toUpperCase() + info.kind.slice(1) : '')
  const getNote = $derived(view.current.Method.toUpperCase() === 'GET' && info?.kind && !info.getOK ? `A ${info.kind} cannot be sent with GET: use POST.` : '')
  const set = (field: string, text: string) => RequestService.SetGraphQL(node, field, text)

  // A subscription is a stream: how many events to wait for, for how long, and which ones count.
  const isSubscription = $derived(info?.kind === 'subscription')
  const receive = $derived(body.Receive ?? { Count: 0, Timeout: 0, Match: '' })
  let countError = $state('')
  let timeoutBad = $state(false)
  const putReceive = (change: (r: { Count: number; Timeout: number; Match: string }) => void) => {
    const next = { ...$state.snapshot(receive) }
    change(next)
    RequestService.SetGraphQLReceive(node, next as never)
  }
</script>

<div class="gql">
  <div class="bar">
    {#if kindWord}<span class="kind" title="The kind of operation that runs">{kindWord}</span>{/if}
    {#if showOperation}
      <Dropdown title="Operation to run" options={operations} value={body.Operation} onpick={(name) => set('operation', name)} />
    {/if}
    <span class="muted hint">Sent as written. Put values in the variables and use <code>$name</code> in the query.</span>
  </div>
  <div class="pane query">
    <CodeArea label="Query" lang="text" minHeight={140} value={body.Query} oninput={(t) => set('query', t)}
      empty={{ icon: 'body', title: 'Empty query', hint: 'Click to write a query, a mutation or a subscription.' }} />
  </div>
  {#if info?.problem && body.Query.trim()}<p class="note bad" role="status">{info.problem}</p>{/if}
  {#if getNote}<p class="note bad" role="status">{getNote}</p>{/if}
  <div class="pane vars">
    <CodeArea label="Variables" lang="json" minHeight={90} value={body.Variables} oninput={(t) => set('variables', t)}
      empty={{ icon: 'body', title: 'No variables', hint: 'Click to write a JSON object, such as {"id": "{{env.user_id}}"}.' }} />
  </div>
  {#if info?.variablesProblem}<p class="note bad" role="status">{info.variablesProblem}</p>{/if}
  {#if isSubscription}
    <div class="stream">
      <div class="row">
        <Field label="Events (empty: 1)"><TextInput value={receive.Count ? String(receive.Count) : ''} ontext={(t) => {
            const n = parseCount(t)
            countError = n.error
            if (!n.error) putReceive((r) => (r.Count = n.value))
          }} /></Field>
        <Field label="Timeout (empty: 30s)"><TextInput value={showDuration(receive.Timeout)} placeholder="30s" ontext={(t) => {
            const ns = parseDuration(t)
            timeoutBad = ns === null
            if (ns !== null) putReceive((r) => (r.Timeout = ns))
          }} /></Field>
        <TemplateField label="Only if" help="message.filter" placeholder={'body.data.tick > 1'} value={receive.Match} oninput={(t) => putReceive((r) => (r.Match = t))} vocabulary={requestVocabulary('message.filter', node, view.response, view.kind)} />
      </div>
      {#if countError}<p class="note bad" role="status">{countError}</p>{/if}
      {#if timeoutBad}<p class="note bad" role="status">Write the timeout as a duration like 500ms, 30s or 2m.</p>{/if}
      <p class="note">Opens a WebSocket (graphql-transport-ws) on the URL, subscribes, and stops after this many events, when the server completes, or fails at the timeout. The headers sign in with the handshake.</p>
    </div>
  {/if}
</div>

<style>
  .gql { display: flex; flex: 1; min-height: 0; flex-direction: column; gap: 8px; }
  /* The query gets the room; each box keeps a height it can be used at, so neither collapses into the other. */
  .pane { display: flex; flex-direction: column; min-height: 0; }
  .query { flex: 3 1 0; min-height: 160px; }
  .vars { flex: 1 1 0; min-height: 110px; }
  .bar { display: flex; flex-wrap: wrap; align-items: center; gap: 6px 10px; }
  .kind { padding: 1px 8px; border-radius: var(--r-sm); border: 1px solid var(--line); color: var(--v-GQL); font-size: var(--fs-xs); font-weight: 600; letter-spacing: 0.02em; }
  .hint { font-size: var(--fs-xs); }
  .stream { display: flex; flex-direction: column; gap: 6px; }
  .row { display: grid; grid-template-columns: 120px 120px minmax(0, 1fr); gap: 8px; align-items: end; }
  .note { margin: 0; font-size: var(--fs-xs); color: var(--muted); }
  .note.bad { color: var(--warn); }
</style>
