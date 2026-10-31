<script lang="ts">
  // A request is waiting for the person: open the provider's page and approve (authorization code),
  // or enter a code at an address on any device (device code). The dialog stays until the request
  // goes on (a send or a run ends) or it is closed; the request waits meanwhile, up to five minutes.
  import { Browser, Clipboard } from '@wailsio/runtime'
  import { ui } from './lib/ui.svelte'
  import { app } from './lib/app.svelte'

  const p = $derived(ui.signIn)
  const busy = $derived(Object.values(app.requests).some((r) => r.sending) || Object.values(app.flows).some((f) => f.run.running))
  let opened = $state('')

  // A sign-in in this browser opens at once, once; the dialog is for when it did not open or came to the wrong one.
  $effect(() => {
    if (p && p.kind === 'open' && opened !== p.url) {
      opened = p.url
      Browser.OpenURL(p.url)
    }
  })
  // Whatever waited has gone on when nothing runs any more.
  $effect(() => {
    if (p && !busy) ui.signIn = null
  })
</script>

{#if p}
  <div class="scrim" role="presentation">
    <div class="dialog" role="dialog" aria-label="Sign in to continue">
      <h3>Sign in to continue</h3>
      {#if p.kind === 'device'}
        <p>Open this address on any device and enter the code:</p>
        <p class="code" aria-label="Code">{p.code}</p>
      {:else}
        <p>Flowi opened your browser. Sign in and approve there; this goes on by itself.</p>
      {/if}
      <p class="url"><a href={p.url} onclick={(e) => { e.preventDefault(); Browser.OpenURL(p.url) }}>{p.url}</a></p>
      <div class="actions">
        <button onclick={() => { Clipboard.SetText(p.kind === 'device' ? (p.code ?? '') : p.url); ui.toast('Copied') }}>{p.kind === 'device' ? 'Copy code' : 'Copy link'}</button>
        <button class="primary" onclick={() => Browser.OpenURL(p.url)}>{p.kind === 'device' ? 'Open page' : 'Open again'}</button>
        <button onclick={() => (ui.signIn = null)}>Close</button>
      </div>
    </div>
  </div>
{/if}

<style>
  .scrim { position: fixed; inset: 0; background: rgb(0 0 0 / 0.4); display: grid; place-items: center; z-index: 65; }
  .dialog { width: 420px; max-width: calc(100vw - 32px); display: flex; flex-direction: column; gap: 10px; padding: 16px; background: var(--panel); border: 1px solid var(--line); border-radius: var(--r-lg); }
  h3, p { margin: 0; }
  .code { font: 600 28px/1.2 var(--mono); letter-spacing: 0.12em; text-align: center; padding: 8px 0; -webkit-user-select: text; user-select: text; }
  .url { word-break: break-all; font-size: var(--fs-sm); color: var(--muted); }
  .actions { display: flex; justify-content: flex-end; gap: 8px; }
  button { background: transparent; color: var(--text); border: 1px solid var(--line); border-radius: var(--r-md); padding: 4px 14px; font: inherit; }
  .primary { background: var(--accent); color: var(--on-accent); border-color: var(--accent); }
</style>
