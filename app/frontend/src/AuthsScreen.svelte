<script lang="ts">
  import { reported } from './lib/reported'
  import { onMount } from 'svelte'
  import { live } from './lib/live'
  import { AuthsService } from '../bindings/github.com/githugt/flowi/app/internal/bridge'
  import { ui } from './lib/ui.svelte'
  import AuthFields from './AuthFields.svelte'
  import { authSummary } from './lib/authsummary'
  import { authDirty, draftOf, type AuthDraft } from './lib/authdraft'
  import Button from './Button.svelte'
  import Dropdown from './Dropdown.svelte'
  import EmptyState from './EmptyState.svelte'
  import TextInput from './TextInput.svelte'
  import Field from './Field.svelte'
  import type { AuthField, AuthsView, NamedAuth, TokenView } from './lib/types'

  let view = $state<AuthsView>({ auths: [], collections: [], schemes: [], forms: {} })

  onMount(() => live<AuthsView>('auths', () => AuthsService.State(), (v) => (view = v)))

  // The sign-ins a person approved and the app kept (the vault holds them), listed with when each runs out. Reading the
  // vault may ask for its passphrase, so this is its own call and never holds up the list above.
  let tokens = $state<TokenView[]>([])
  async function loadTokens() {
    try {
      tokens = ((await AuthsService.Tokens()) as unknown as TokenView[]) ?? []
    } catch {
      tokens = []
    }
  }
  onMount(loadTokens)
  function forget(t: TokenView) {
    ui.confirm('Forget this sign-in?', `${t.label}\n\nThe next send that needs it asks you to sign in again.`, async () => {
      if (await reported(AuthsService.ForgetToken(t.id), ui.toast)) await loadTokens()
    }, 'Forget')
  }

  // The new one: where it goes, what requests call it, and its scheme.
  let collection = $state('')
  let name = $state('')
  let scheme = $state('scheme:apikey')
  const into = $derived(view.collections.some((c) => c.id === collection) ? collection : (view.collections[0]?.id ?? ''))
  const schemeOptions = $derived(view.schemes.map((s) => ({ value: s.id, label: s.label })))

  // The name just made: its form opens as soon as the list shows it, so its values can be filled in at once.
  let justMade = $state<string | null>(null)
  // The name stays in the field until the authentication is made, so a refusal (a name already taken) can be mended.
  async function add() {
    if (!name.trim() || !into) return
    const made = `${into}\u0000${name.trim()}`
    if (!(await reported(AuthsService.Create(into, name, scheme), ui.toast))) return
    justMade = made
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
  let draft = $state<AuthDraft>({ description: '', scheme: '', values: {} })
  const dirty = $derived.by(() => {
    const a = view.auths.find((x) => x.key === opened)
    return !!a && authDirty(a, draft)
  })

  function open(a: NamedAuth) {
    opened = a.key
    draft = draftOf(a)
  }

  // Closing the form, or opening another authentication's, asks first when the form holds edits that are not saved.
  function edit(a: NamedAuth) {
    const leaving = view.auths.find((x) => x.key === opened)
    const go = () => (opened === a.key ? void (opened = null) : open(a))
    if (!leaving || !dirty) return go()
    ui.unsaved('Unsaved changes', `${leaving.name} has changes that are not saved.`, async () => (await save(leaving)) && go(), go, { save: 'Save', discard: 'Discard' })
  }

  // The stored fields while the scheme is the stored one (they keep a mistyped parameter in view), else the scheme's own.
  const fieldsOf = (a: NamedAuth): AuthField[] =>
    (draft.scheme === a.scheme ? a.fields : (view.forms[draft.scheme] ?? [])).map((f) => ({ ...f, value: draft.values[f.name] ?? '' }))

  function save(a: NamedAuth): Promise<boolean> {
    const names = fieldsOf(a).map((f) => f.name)
    return reported(AuthsService.Save(a.key, draft.description, draft.scheme, Object.fromEntries(names.map((n) => [n, draft.values[n] ?? '']))), ui.toast, `Saved ${a.name}`)
  }

  function rename(a: NamedAuth) {
    const uses = a.uses.length ? ` The ${a.uses.length === 1 ? 'request' : `${a.uses.length} requests`} that use it will follow.` : ''
    ui.prompt({ title: 'Rename authentication', label: `New name for ${a.name}.${uses}`, initial: a.name, done: (name) => reported(AuthsService.Rename(a.key, name), ui.toast, `Renamed ${a.name} to ${name.trim()}`) })
  }

  function remove(a: NamedAuth) {
    const used = a.uses.length
      ? ` ${a.uses.length === 1 ? 'One request uses' : `${a.uses.length} requests use`} it (${a.uses.join(', ')}) and cannot be sent until another authentication is picked there.`
      : ' No request uses it.'
    ui.confirm('Delete authentication', `Delete the authentication ${a.name}?${used}`, () => void reported(AuthsService.Delete(a.key), ui.toast, `Deleted ${a.name}`))
  }
</script>

<div class="page shell-page">
  <p class="note">An authentication defined here has a name. A request uses it by that name instead of repeating the values: in the request's Auth section, pick Named authentication. Each is a file in the collection's <code>auths</code> folder.</p>

  <div class="new">
    {#if view.collections.length > 1}
      <Dropdown title="Collection" options={view.collections.map((c) => ({ value: c.id, label: c.label }))} value={into} onpick={(v) => (collection = v)} />
    {/if}
    <div class="grow"><Field label="Name"><TextInput placeholder="partner-api" value={name} ontext={(t) => (name = t)} onenter={add} /></Field></div>
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
          <Field label="Description"><TextInput value={draft.description} ontext={(t) => (draft.description = t)} /></Field>
          <div class="typebar"><Dropdown title="Scheme" options={schemeOptions} value={draft.scheme} onpick={(v) => (draft.scheme = v)} /></div>
          <AuthFields fields={fieldsOf(a)} onparam={(n, t) => (draft.values[n] = t)} />
          {#if authSummary('scheme:' + draft.scheme, '', fieldsOf(a))}<p class="muted">When sent, it {authSummary('scheme:' + draft.scheme, '', fieldsOf(a)).replace(/`([^`]+)`/g, '“$1”')}.</p>{/if}
          {#if a.uses.length}<p class="muted">Used by {a.uses.join(', ')}.</p>{/if}
          <div class="actions"><Button variant="primary" onclick={() => save(a)}>Save</Button>{#if dirty}<span class="muted" role="status">Unsaved changes</span>{/if}</div>
        </div>
      {/if}
    </div>
  {/each}

  {#if tokens.length}
    <h3 class="tokens-head">Saved sign-ins</h3>
    <p class="note">A sign-in you approved in a browser is kept in the vault, encrypted, so a restart does not ask again.</p>
    {#each tokens as t (t.id)}
      <div class="shell-card">
        <div class="shell-top">
          <strong>{t.label}</strong>
          <span class="token-status" class:bad={t.expired}>{t.status}</span>
          <span class="shell-spacer"></span>
          <Button variant="danger" onclick={() => forget(t)}>Forget…</Button>
        </div>
      </div>
    {/each}
  {/if}
</div>

<style>
  .tokens-head { margin: 18px 0 4px; font-size: var(--fs-md); }
  .token-status { color: var(--ok); font-size: var(--fs-sm); }
  .token-status.bad { color: var(--warn); }
  .note, .muted { color: var(--muted); font-size: var(--fs-sm); margin: 0 0 4px; }
  .new { display: flex; align-items: flex-end; gap: 10px; margin: 12px 0; }
  .grow { flex: 1; min-width: 0; }
  .form { display: flex; flex-direction: column; gap: 8px; padding: 10px 0 4px; }
  .typebar, .actions { display: flex; align-items: center; gap: 10px; }
  code { font: var(--fs-xs) var(--mono); }
</style>
