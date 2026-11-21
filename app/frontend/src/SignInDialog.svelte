<script lang="ts">
  import Modal from './Modal.svelte'
  import Button from './Button.svelte'
  import { copy } from './lib/clip'
  // A request is waiting for the person: open the provider's page and approve (authorization code),
  // or enter a code at an address on any device (device code). The dialog stays until the request
  // goes on (a send or a run ends) or it is closed; the request waits meanwhile, up to five minutes.
  import { openExternal } from './lib/openurl'
  import { ui } from './lib/ui.svelte'
  import { app } from './lib/app.svelte'
  import { hostOf, opensByItself, rememberHost, seenHosts } from './lib/signinhost'
  import { countdown } from './lib/countdown'
  import { FlowService, RequestService } from '../bindings/github.com/githugt/flowi/app/internal/bridge'

  const p = $derived(ui.signIn)
  const busy = $derived(Object.values(app.requests).some((r) => r.sending) || Object.values(app.flows).some((f) => f.run.running))
  let opened = $state('')

  // A sign-in in this browser opens at once, once; the dialog is for when it did not open or came to the wrong one.
  const host = $derived(p ? hostOf(p.url) : '')
  const firstUse = $derived(!!p && !opensByItself(p.url, seenHosts()))
  function open() {
    if (!p) return
    rememberHost(host)
    openExternal(p.url)
  }
  // Only a host seen before opens by itself; a new one waits for a click, with its name in front of the person.
  $effect(() => {
    if (p && p.kind === 'open' && opened !== p.url && !firstUse) {
      opened = p.url
      open()
    }
  })
  // The time left on a code, counted down in the window.
  let now = $state(Date.now())
  $effect(() => {
    if (!p?.expires) return
    const tick = setInterval(() => (now = Date.now()), 1000)
    return () => clearInterval(tick)
  })
  const left = $derived(p ? countdown(p.expires, now) : '')
  // Close only hides the dialog and the request goes on waiting; this ends the wait: the sends and runs that wait stop.
  function cancelWait() {
    for (const [node, r] of Object.entries(app.requests)) if (r.sending) void RequestService.Cancel(node)
    for (const [node, f] of Object.entries(app.flows)) if (f.run.running) void FlowService.Cancel(node)
    ui.signIn = null
  }
  // Whatever waited has gone on when nothing runs any more.
  $effect(() => {
    if (p && !busy) ui.signIn = null
  })
</script>

{#if p}
  <Modal title="Sign in to continue" layer="above">
      {#if p.kind === 'device'}
        <p>Open this address on any device and enter the code:</p>
        <p class="code" aria-label="Code">{p.code}</p>
        {#if left}<p class="left" class:gone={left === 'expired'} role="timer">{left === 'expired' ? 'The code has expired. Cancel and send again.' : `The code works for ${left} more.`}</p>{/if}
      {:else}
        {#if firstUse}
          <p>A request is asking you to sign in at <strong class="host">{host || 'an address that is not a web page'}</strong>.</p>
          <p class="newhost" role="note">Host not seen before. Open it only if you expect this sign-in.</p>
        {:else}
          <p>Flowi opened your browser at <strong class="host">{host}</strong>. Sign in and approve there; this goes on by itself.</p>
        {/if}
      {/if}
      <p class="url"><a href={p.url} onclick={(e) => { e.preventDefault(); open() }}>{p.url}</a></p>
    {#snippet actions()}
        <Button onclick={() => void copy(p.kind === 'device' ? (p.code ?? '') : p.url, p.kind === 'device' ? 'the code' : 'the link')}>{p.kind === 'device' ? 'Copy code' : 'Copy link'}</Button>
        <Button variant="primary" disabled={!host} onclick={open}>{p.kind === 'device' ? 'Open page' : firstUse ? 'Open' : 'Open again'}</Button>
        <Button variant="danger" onclick={cancelWait}>Cancel sign-in</Button>
        <Button onclick={() => (ui.signIn = null)}>Hide</Button>
    {/snippet}
  </Modal>
{/if}

<style>
  p { margin: 0; }
  .code { font: 600 28px/1.2 var(--mono); letter-spacing: 0.12em; text-align: center; padding: 8px 0; -webkit-user-select: text; user-select: text; }
  .url { word-break: break-all; font-size: var(--fs-sm); color: var(--muted); }
  .host { overflow-wrap: anywhere; }
  .left { text-align: center; color: var(--muted); font-size: var(--fs-sm); }
  .left.gone { color: var(--fail); }
  .newhost { color: var(--warn); font-size: var(--fs-sm); }
</style>
