<script lang="ts">
  import HiddenChars from './HiddenChars.svelte'
  import { onMount } from 'svelte'
  import { live } from './lib/live'
  import { PassphraseService, SecretsService } from '../bindings/github.com/githugt/flowi/internal/bridge'
  import type { SecretsView } from './lib/types'

  let view = $state<SecretsView>({ secrets: [] })
  let values = $state<Record<string, string>>({})

  // The user turns to secrets: a passphrase they cancelled may be asked again, before the state is read.
  onMount(() => live<SecretsView>('secrets', async () => (await PassphraseService.Unlock(), SecretsService.State()), (v) => (view = v)))

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

<div class="page">
  <p class="note">Secret values live in the encrypted workspace vault, never in a file. They are never shown again after you store them.</p>
  {#if view.error}<p class="err">{view.error}</p>{/if}
  {#if !view.secrets.length}<p class="note">No environment declares a secret yet.</p>{/if}
  {#each byEnv as [env, list]}
    <h3>{env}</h3>
    {#each list ?? [] as s (key(s))}
      <form class="row" onsubmit={(e) => { e.preventDefault(); save(s) }}>
        <code>{s.name}</code>
        <span class="state {s.state}">{s.state}</span>
        <span class="cell"><input type="password" placeholder={s.state === 'stored' ? 'type to replace' : 'value'} bind:value={values[key(s)]} disabled={s.state === 'overridden'} /><HiddenChars value={values[key(s)] ?? ''} onclean={(t) => (values[key(s)] = t)} /></span>
        <button type="submit" disabled={!values[key(s)]}>Store</button>
        <button type="button" disabled={s.state !== 'stored'} onclick={() => SecretsService.Unset(s.environment, s.name)}>Remove</button>
      </form>
    {/each}
  {/each}
</div>

<style>
  .page { position: absolute; inset: 0; overflow: auto; padding: 16px 24px; max-width: 820px; }
  .note { color: var(--muted); margin-top: 0; }
  .err { color: var(--fail); }
  h3 { margin: 20px 0 6px; font-size: var(--fs-md); border-bottom: 1px solid var(--line); padding-bottom: 4px; }
  .row { display: grid; grid-template-columns: 160px 90px 1fr auto auto; gap: 8px; align-items: center; margin: 6px 0; }
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
</style>
