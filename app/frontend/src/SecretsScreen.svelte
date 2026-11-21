<script lang="ts">
  import Checkbox from './Checkbox.svelte'
  import ScreenPage from './ScreenPage.svelte'
  import Status from './Status.svelte'
  import EmptyState from './EmptyState.svelte'
  import { reported } from './lib/reported'
  import TextInput from './TextInput.svelte'
  import HiddenChars from './HiddenChars.svelte'
  import { onMount, onDestroy } from 'svelte'
  import { errorText } from './lib/errors'
  import { live } from './lib/live'
  import { ui } from './lib/ui.svelte'
  import { Clipboard } from '@wailsio/runtime'
  import Button from './Button.svelte'
  import { helpUrl } from './lib/helplinks'
  import { LOADING, READY, failed, mayShowEmpty, type LoadState } from './lib/loadstate'
  import { copyAndClear } from './lib/clipclear'
  import { revealsNote } from './lib/revealsnote'
  import { PassphraseService, SecretsService } from '../bindings/github.com/githugt/flowi/app/internal/bridge'
  import type { SecretsView } from './lib/types'
  import { app } from './lib/app.svelte'
  import { usedByText, visibleSecrets } from './lib/secretlist'
  import { vaultChip } from './lib/vaultchip'

  let view = $state<SecretsView>({ vault: { exists: false, unlocked: false, prompts: false, autoLockSeconds: 0, locksInSeconds: 0 }, secrets: [], reveals: 0 })
  let load = $state<LoadState>(LOADING)
  let values = $state<Record<string, string>>({})

  // The user turns to secrets: a passphrase they cancelled may be asked again, before the state is read.
  // A failed load is told, with Try again, instead of showing an empty list.
  let stopLive = () => {}
  function start() {
    load = LOADING
    stopLive()
    stopLive = live<SecretsView>('secrets', async () => (await PassphraseService.Unlock(), SecretsService.State()), (v) => ((view = v), (load = READY)), (e) => (load = failed(e)))
  }
  onMount(() => {
    start()
    return () => stopLive()
  })

  // One secret at a time is shown, only for a moment: it is hidden again after REVEAL_MS, when the window loses
  // focus or hides, and when this page goes. The value lives in this variable and nowhere else.
  const REVEAL_MS = 15_000
  let shown = $state<{ id: string; value: string } | null>(null)
  let revealError = $state('')
  let hideTimer: ReturnType<typeof setTimeout> | undefined

  function hide() {
    clearTimeout(hideTimer)
    shown = null
  }

  async function reveal(s: SecretsView['secrets'][number]) {
    revealError = ''
    try {
      await PassphraseService.Unlock()
      const value = await SecretsService.Reveal(s.environment, s.name)
      shown = { id: key(s), value }
      view.reveals++
      clearTimeout(hideTimer)
      hideTimer = setTimeout(hide, REVEAL_MS)
    } catch (e) {
      hide()
      revealError = errorText(e)
    }
  }

  // Removing a stored value cannot be undone: the vault keeps no copy.
  function remove(s: SecretsView['secrets'][number]) {
    // Before: a refusal from the vault was dropped, so the row stayed as it was with no word why.
    ui.confirm('Remove secret value', `Remove the stored value of "${s.name}" in ${s.environment}? It cannot be recovered; you would have to enter it again.`, () => void reported(SecretsService.Unset(s.environment, s.name), ui.toast, `Removed ${s.name} from ${s.environment}`), 'Remove')
  }

  // Copied for 30 seconds: the clipboard is cleared then, unless something else was copied since.
  let cancelClear: (() => void) | undefined
  async function copySecret() {
    if (!shown) return
    cancelClear?.()
    cancelClear = await copyAndClear(shown.value, {
      write: (t) => Clipboard.SetText(t),
      read: () => Clipboard.Text(),
      later: (fn, ms) => {
        const id = setTimeout(fn, ms)
        return () => clearTimeout(id)
      },
    })
    ui.toast('Copied. The clipboard is cleared in 30 seconds.')
  }

  const hideWhenAway = () => document.visibilityState === 'hidden' && hide()
  onDestroy(hide)

  const key = (s: { environment: string; name: string }) => `${s.environment}\u0000${s.name}`
  let onlyMissing = $state(false)
  const chip = $derived(vaultChip(view.vault))
  const missingCount = $derived(view.secrets.filter((s) => s.state === 'missing').length)
  const byEnv = $derived(Object.entries(Object.groupBy(visibleSecrets(view.secrets, onlyMissing), (s) => s.environment)))
  // The secret whose users are listed.
  let usersOf = $state<string | null>(null)
  const openUse = (node: string) => {
    const n = node ? app.find(node) : undefined
    if (n) void app.open(n)
    else ui.toast('That file is not in the open collections.', 'error')
  }

  async function save(s: SecretsView['secrets'][number]) {
    const v = values[key(s)]
    if (!v) return
    // The typed value stays in the box when the vault refuses it.
    if (await reported((async () => { await PassphraseService.Unlock(); await SecretsService.Set(s.environment, s.name, v) })(), ui.toast, `Stored ${s.name} in ${s.environment}`)) delete values[key(s)]
  }
</script>

<svelte:window onblur={hide} />
<svelte:document onvisibilitychange={hideWhenAway} />

<ScreenPage>
  <p class="note">Secret values live in the encrypted workspace vault, never in a file. They stay hidden; <b>Show</b> reveals one secret for {REVEAL_MS / 1000} seconds. On the command line: <code>flowi secret get &lt;env&gt; &lt;name&gt; --reveal</code>.</p>
  {#if chip}
    <p class="note vault" role="status" title={chip.hint}>
      <span>{chip.text}</span>
      <Button small variant="ghost" disabled={!chip.canLock} title={chip.hint} onclick={() => void reported(SecretsService.LockVault(), ui.toast, 'Vault locked')}>Lock now</Button>
    </p>
  {/if}
  {#if view.reveals > 0}<p class="note" role="status">{revealsNote(view.reveals)}</p>{/if}
  {#if revealError}<p class="err">{revealError}</p>{/if}
  {#if view.error}<p class="err">{view.error}</p>{/if}
  {#if load.status === 'error'}
    <p class="err" role="alert">Could not load the secrets: {load.error} <Button small onclick={start}>Try again</Button></p>
  {:else if mayShowEmpty(load, view.secrets.length)}<EmptyState icon="auth" learn={helpUrl('docs')} title="No secret is declared yet" hint={'Open an environment in the sidebar, add a variable and tick Secret; it then appears here to be given a value. Example: a database password, kept out of the request file as {{secret.db_password}}.'} compact />{/if}
  {#if view.secrets.length}
    <p class="tools">
      <Checkbox bind:checked={onlyMissing}>Only secrets with no value{missingCount ? ` (${missingCount})` : ''}</Checkbox>
      <Button small variant="ghost" title="See every variable of every environment side by side" onclick={() => app.openScreen('screen:envcompare', 'Compare environments')}>Compare environments</Button>
    </p>
    {#if onlyMissing && !byEnv.length}<p class="note" role="status">Every secret has a value.</p>{/if}
  {/if}
  {#each byEnv as [env, list]}
    <h3>{env}</h3>
    {#each list ?? [] as s (key(s))}
      <form class="row" onsubmit={(e) => { e.preventDefault(); save(s) }}>
        <code>{s.name}</code>
        <Status tone={s.state === 'stored' ? 'ok' : s.state === 'missing' ? 'warn' : 'info'}>{s.state}</Status>
        <span class="cell">
          {#if shown?.id === key(s)}
            <!-- The stored value takes the box's place while it is shown; read only, so it cannot be mistaken for a new value. -->
            <TextInput readonly mono value={shown.value} label="Stored value of {s.name}" />
          {:else}
            <TextInput type="password" label="Value of {s.name}" placeholder={s.state === 'stored' ? 'type to replace' : 'value'} bind:value={values[key(s)]} disabled={s.state === 'overridden'} /><HiddenChars value={values[key(s)] ?? ''} onclean={(t) => (values[key(s)] = t)} />
          {/if}
        </span>
        <Button type="submit" disabled={!values[key(s)] || shown?.id === key(s)}>Store</Button>
        {#if shown?.id === key(s)}
          <Button small title="Copy the value; the clipboard is cleared after 30 seconds" onclick={copySecret}>Copy</Button>
          <Button type="button" onclick={hide}>Hide</Button>
        {:else}
          <Button type="button" disabled={s.state !== 'stored'} title={s.state === 'overridden' ? 'An environment variable provides this value: it is already in your environment' : 'Show the stored value for a few seconds'} onclick={() => reveal(s)}>Show</Button>
        {/if}
        <Button type="button" disabled={s.state !== 'stored'} onclick={() => remove(s)}>Remove</Button>
      </form>
      <p class="uses">
        <Button small variant="ghost" pressed={usersOf === key(s)} title="Which requests, flows and authentications write {'{{secret.' + s.name + '}}'}" onclick={() => (usersOf = usersOf === key(s) ? null : key(s))}>{usedByText(s.usedBy)}</Button>
        {#if usersOf === key(s)}
          {#each s.usedBy as u (u.kind + u.label)}
            {#if u.node}<Button small variant="ghost" title="Open {u.label}" onclick={() => openUse(u.node)}>{u.label}</Button>{:else}<span class="auth">{u.label} (authentication)</span>{/if}
          {/each}
        {/if}
      </p>
    {/each}
  {/each}
</ScreenPage>

<style>
  .note { color: var(--muted); margin-top: 0; }
  .vault { display: flex; align-items: center; gap: 8px; }
  .err { color: var(--fail); }
  h3 { margin: 20px 0 6px; font-size: var(--fs-lg); font-weight: 600; }
  .row { display: grid; grid-template-columns: 160px 90px 1fr auto auto auto auto; gap: 8px; align-items: center; margin: 6px 0; }
  code { font: var(--fs-sm) var(--mono); }
  .tools { display: flex; gap: 12px; align-items: center; margin: 0 0 6px; }
  .uses { margin: -2px 0 8px; display: flex; flex-wrap: wrap; gap: 2px 6px; align-items: center; font-size: var(--fs-xs); }
  .auth { color: var(--muted); }
  .cell { position: relative; display: flex; flex: 1; min-width: 0; }
</style>
