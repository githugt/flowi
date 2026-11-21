<script lang="ts">
  // "Test connection" for a message or database request: connect, sign in and disconnect, sending nothing. The
  // verdict stays beside the server box until the server or the credentials change.
  import { RequestService } from '../bindings/github.com/githugt/flowi/app/internal/bridge'
  import type { ConnectionInfo } from './lib/types'
  import Button from './Button.svelte'

  let { node, info }: { node: string; info: ConnectionInfo } = $props()

  const words = $derived(
    info.busy
      ? 'Testing…'
      : info.status === 'ok'
        ? `Connected · ${info.tookMs} ms`
        : info.status === 'failed'
          ? `Failed: ${info.detail ?? ''}`
          : info.status === 'unsupported'
            ? (info.detail ?? '')
            : '',
  )
</script>

<span class="conn">
  <Button variant="ghost" small loading={info.busy} title="Connect and sign in with this request's credentials, without sending or reading anything" onclick={() => RequestService.TestConnection(node)}>Test</Button>
  {#if words}
    <span class="verdict" class:ok={info.status === 'ok'} class:bad={info.status === 'failed'} role="status" title={info.target ? `${info.target}\n${info.detail ?? ''}` : info.detail}>{words}</span>
  {/if}
</span>

<style>
  .conn { flex: none; display: flex; align-items: center; gap: 6px; padding-right: 6px; min-width: 0; }
  .verdict { font-size: var(--fs-xs); color: var(--muted); max-width: 22em; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .verdict.ok { color: var(--ok); }
  .verdict.bad { color: var(--fail); }
</style>
