<script lang="ts">
  import HiddenChars from './HiddenChars.svelte'
  import { onMount, onDestroy } from 'svelte'
  import { errorText } from './lib/errors'
  import { live } from './lib/live'
  import { ui } from './lib/ui.svelte'
  import { PassphraseService, SecretsService } from '../bindings/github.com/githugt/flowi/app/internal/bridge'
  import type { SecretsView } from './lib/types'

  let view = $state<SecretsView>({ secrets: [] })
  let values = $state<Record<string, string>>({})

  // The user turns to secrets: a passphrase they cancelled may be asked again, before the state is read.
  onMount(() => live<SecretsView>('secrets', async () => (await PassphraseService.Unlock(), SecretsService.State()), (v) => (view = v)))

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
      clearTimeout(hideTimer)
      hideTimer = setTimeout(hide, REVEAL_MS)
    } catch (e) {
      hide()
      revealError = errorText(e)
    }
  }

  // Removing a stored value cannot be undone: the vault keeps no copy.
  function remove(s: SecretsView['secrets'][number]) {
    ui.confirm('Remove secret value', `Remove the stored value of "${s.name}" in ${s.environment}? It cannot be recovered; you would have to enter it again.`, () => SecretsService.Unset(s.environment, s.name), 'Remove')
  }

  const hideWhenAway = () => document.visibilityState === 'hidden' && hide()
  onDestroy(hide)

  const key = (s: { environment: string; name: string }) => `${s.environment}\u0000${s.name}`
  const byEnv = $derived(Object.entries(Object.groupBy(view.secrets, (s) => s.environment)))

  async function save(s: SecretsView['secrets'][number]) {
    const v = values[key(s)]
    if (!v) return
    await PassphraseService.Unlock()
    await SecretsService.Set(s.environment, s.name, v)
    delete values[key(s)]
  }
</script>

<svelte:window onblur={hide} />
<svelte:document onvisibilitychange={hideWhenAway} />

<div class="page">
  <p class="note">Secret values live in the encrypted workspace vault, never in a file. They stay hidden; <b>Show</b> reveals one secret for {REVEAL_MS / 1000} seconds. On the command line: <code>flowi secret get &lt;env&gt; &lt;name&gt; --reveal</code>.</p>
  {#if revealError}<p class="err">{revealError}</p>{/if}
  {#if view.error}<p class="err">{view.error}</p>{/if}
  {#if !view.secrets.length}<p class="note">No environment declares a secret yet.</p>{/if}
  {#each byEnv as [env, list]}
    <h3>{env}</h3>
    {#each list ?? [] as s (key(s))}
      <form class="row" onsubmit={(e) => { e.preventDefault(); save(s) }}>
        <code>{s.name}</code>
        <span class="state {s.state}">{s.state}</span>
        <span class="cell">
          {#if shown?.id === key(s)}
            <!-- The stored value takes the box's place while it is shown; read only, so it cannot be mistaken for a new value. -->
            <input class="revealed" type="text" readonly value={shown.value} aria-label="Stored value of {s.name}" spellcheck="false" />
          {:else}
            <input type="password" placeholder={s.state === 'stored' ? 'type to replace' : 'value'} bind:value={values[key(s)]} disabled={s.state === 'overridden'} /><HiddenChars value={values[key(s)] ?? ''} onclean={(t) => (values[key(s)] = t)} />
          {/if}
        </span>
        <button type="submit" disabled={!values[key(s)] || shown?.id === key(s)}>Store</button>
        {#if shown?.id === key(s)}
          <button type="button" onclick={hide}>Hide</button>
        {:else}
          <button type="button" disabled={s.state !== 'stored'} title={s.state === 'overridden' ? 'An environment variable provides this value: it is already in your environment' : 'Show the stored value for a few seconds'} onclick={() => reveal(s)}>Show</button>
        {/if}
        <button type="button" disabled={s.state !== 'stored'} onclick={() => remove(s)}>Remove</button>
      </form>
    {/each}
  {/each}
</div>

<style>
  .page { position: absolute; inset: 0; overflow: auto; padding: 16px 24px; max-width: 820px; }
  .note { color: var(--muted); margin-top: 0; }
  .err { color: var(--fail); }
  h3 { margin: 20px 0 6px; font-size: var(--fs-md); border-bottom: 1px solid var(--line); padding-bottom: 4px; }
  .row { display: grid; grid-template-columns: 160px 90px 1fr auto auto auto; gap: 8px; align-items: center; margin: 6px 0; }
  code { font: var(--fs-sm) var(--mono); }
  .state { font-size: var(--fs-xs); color: var(--muted); }
  .state.stored { color: var(--ok); }
  .state.missing { color: var(--warn); }
  .state.overridden { color: var(--accent); }
  .cell { position: relative; display: flex; flex: 1; min-width: 0; }
  .cell input { flex: 1; }
  input { background: var(--bg); color: var(--text); border: 1px solid var(--line); border-radius: var(--r-md); padding: 5px 8px; font: inherit; min-width: 0; -webkit-user-select: text; user-select: text; }
  button { background: transparent; color: var(--text); border: 1px solid var(--line); border-radius: var(--r-md); padding: 3px 12px; font: inherit; }
  button:disabled { opacity: 0.4; }
  .revealed { border-color: var(--accent); font-family: var(--mono, ui-monospace, monospace); }
</style>
