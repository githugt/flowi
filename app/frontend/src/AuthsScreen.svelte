<script lang="ts">
  import { onMount } from 'svelte'
  import { live } from './lib/live'
  import { AuthsService } from '../bindings/github.com/githugt/flowi/app/internal/bridge'
  import { ui } from './lib/ui.svelte'
  import AuthFields from './AuthFields.svelte'
  import { authSummary } from './lib/authsummary'
  import Button from './Button.svelte'
  import Dropdown from './Dropdown.svelte'
  import EmptyState from './EmptyState.svelte'
  import TextField from './TextField.svelte'
  import type { AuthField, AuthsView, NamedAuth } from './lib/types'

  let view = $state<AuthsView>({ auths: [], collections: [], schemes: [], forms: {} })

  onMount(() => live<AuthsView>('auths', () => AuthsService.State(), (v) => (view = v)))

  // The new one: where it goes, what requests call it, and its scheme.
  let collection = $state('')
  let name = $state('')
  let scheme = $state('scheme:apikey')
  const into = $derived(view.collections.some((c) => c.id === collection) ? collection : (view.collections[0]?.id ?? ''))
  const schemeOptions = $derived(view.schemes.map((s) => ({ value: s.id, label: s.label })))

  // The name just made: its form opens as soon as the list shows it, so its values can be filled in at once.
  let justMade = $state<string | null>(null)
  function add() {
    if (!name.trim() || !into) return
    justMade = `${into}\u0000${name.trim()}`
    AuthsService.Create(into, name, scheme)
    opened = null
    name = ''
  }
  $effect(() => {
    if (justMade === null) return
    const [coll, made] = justMade.split('\u0000')
    const a = view.auths.find((x) => x.collection === coll && x.name === made)
    if (a) {
      justMade = null
      edit(a)
    }
  })

  // The one being edited, and what the form holds until Save.
  let opened = $state<string | null>(null)
  let draft = $state<{ description: string; scheme: string; values: Record<string, string> }>({ description: '', scheme: '', values: {} })

  function edit(a: NamedAuth) {
    if (opened === a.key) return void (opened = null)
    opened = a.key
    draft = { description: a.description, scheme: a.scheme, values: Object.fromEntries(a.fields.map((f) => [f.name, f.value])) }
  }

  // The stored fields while the scheme is the stored one (they keep a mistyped parameter in view), else the scheme's own.
  const fieldsOf = (a: NamedAuth): AuthField[] =>
    (draft.scheme === a.scheme ? a.fields : (view.forms[draft.scheme] ?? [])).map((f) => ({ ...f, value: draft.values[f.name] ?? '' }))

  function save(a: NamedAuth) {
    const names = fieldsOf(a).map((f) => f.name)
    AuthsService.Save(a.key, draft.description, draft.scheme, Object.fromEntries(names.map((n) => [n, draft.values[n] ?? ''])))
  }

  function rename(a: NamedAuth) {
    const uses = a.uses.length ? ` The ${a.uses.length === 1 ? 'request' : `${a.uses.length} requests`} that use it will follow.` : ''
    ui.prompt({ title: 'Rename authentication', label: `New name for ${a.name}.${uses}`, initial: a.name, done: (name) => AuthsService.Rename(a.key, name) })
  }

  function remove(a: NamedAuth) {
    const used = a.uses.length
      ? ` ${a.uses.length === 1 ? 'One request uses' : `${a.uses.length} requests use`} it (${a.uses.join(', ')}) and cannot be sent until another authentication is picked there.`
      : ' No request uses it.'
    ui.confirm('Delete authentication', `Delete the authentication ${a.name}?${used}`, () => AuthsService.Delete(a.key))
  }
</script>

<div class="page shell-page">
  <p class="note">An authentication defined here has a name. A request uses it by that name instead of repeating the values: in the request's Auth section, pick Named authentication. Each is a file in the collection's <code>auths</code> folder.</p>

  <div class="new">
    {#if view.collections.length > 1}
      <Dropdown title="Collection" options={view.collections.map((c) => ({ value: c.id, label: c.label }))} value={into} onpick={(v) => (collection = v)} />
    {/if}
    <div class="grow"><TextField label="Name" placeholder="partner-api" value={name} oninput={(t) => (name = t)} onenter={add} /></div>
    <Dropdown title="Scheme" options={schemeOptions} value={scheme} onpick={(v) => (scheme = v)} />
    <Button variant="primary" disabled={!name.trim() || !into} onclick={add}>Add authentication</Button>
  </div>
  {#if !view.collections.length}<p class="note">Open or create a collection first: an authentication belongs to one.</p>{/if}

  {#if !view.auths.length}
    <EmptyState icon="auth" title="No authentications yet" hint="Name one above, then fill in its values." compact />
  {/if}
  {#each view.auths as a (a.key)}
    <div class="shell-card">
      <div class="shell-top">
        <strong>{a.name}</strong><span class="shell-tag">{a.schemeLabel || 'Unknown scheme'}</span><span class="muted">{a.collectionName}</span>
        <span class="muted">{a.uses.length ? `Used by ${a.uses.length} ${a.uses.length === 1 ? 'request' : 'requests'}` : 'Not used yet'}</span>
        <span class="shell-spacer"></span>
        <Button aria-expanded={opened === a.key} onclick={() => edit(a)}>{opened === a.key ? 'Close' : 'Edit'}</Button>
        <Button onclick={() => rename(a)}>Rename…</Button>
        <Button variant="danger" onclick={() => remove(a)}>Delete…</Button>
      </div>
      {#if a.description && opened !== a.key}<p class="muted">{a.description}</p>{/if}
      {#if opened === a.key}
        <div class="form">
          <TextField label="Description" value={draft.description} oninput={(t) => (draft.description = t)} />
          <div class="typebar"><Dropdown title="Scheme" options={schemeOptions} value={draft.scheme} onpick={(v) => (draft.scheme = v)} /></div>
          <AuthFields fields={fieldsOf(a)} onparam={(n, t) => (draft.values[n] = t)} />
          {#if authSummary('scheme:' + draft.scheme, '', fieldsOf(a))}<p class="muted">When sent, it {authSummary('scheme:' + draft.scheme, '', fieldsOf(a)).replace(/`([^`]+)`/g, '“$1”')}.</p>{/if}
          {#if a.uses.length}<p class="muted">Used by {a.uses.join(', ')}.</p>{/if}
          <div class="actions"><Button variant="primary" onclick={() => save(a)}>Save</Button></div>
        </div>
      {/if}
    </div>
  {/each}
</div>

<style>
  .note, .muted { color: var(--muted); font-size: var(--fs-sm); margin: 0 0 4px; }
  .new { display: flex; align-items: flex-end; gap: 10px; margin: 12px 0; }
  .grow { flex: 1; min-width: 0; }
  .form { display: flex; flex-direction: column; gap: 8px; padding: 10px 0 4px; }
  .typebar, .actions { display: flex; gap: 10px; }
  code { font: var(--fs-xs) var(--mono); }
</style>
