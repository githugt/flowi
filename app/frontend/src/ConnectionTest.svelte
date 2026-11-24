<script lang="ts">
  // "Test" for any request: a message or database connects, signs in and disconnects; an HTTP request goes out as a
  // HEAD; a gRPC call connects and sends its metadata; a data file is opened and read for its tables. Nothing is
  // sent, read or changed. The verdict stays beside the address until the address or the credentials change.
  // Before: only a message or a database request had it.
  import { RequestService } from '../bindings/github.com/githugt/flowi/app/internal/bridge'
  import type { ConnectionInfo } from './lib/types'
  import Button from './Button.svelte'

  let { node, info, kind = 'connect' }: { node: string; info: ConnectionInfo; kind?: 'connect' | 'http' | 'grpc' | 'file' } = $props()

  const tips = {
    connect: "Connect and sign in with this request's credentials, without sending or reading anything",
    http: 'Ask the address with a HEAD request, using the headers and authentication but no body: it shows whether the server is there and accepts the sign-in',
    grpc: 'Connect to the server and sign in with the metadata, without calling a method',
    file: 'Check that the file or folder exists and reads as its format, without running the query',
  }

  const words = $derived(
    info.busy
      ? 'Testing…'
      : info.status === 'ok'
        ? kind === 'connect' || kind === 'grpc'
          ? `Connected · ${info.tookMs} ms`
          : `${info.detail ?? 'OK'}${kind === 'http' ? ` · ${info.tookMs} ms` : ''}`
        : info.status === 'failed'
          ? kind === 'http'
            ? (info.detail ?? 'Failed')
            : `Failed: ${info.detail ?? ''}`
          : info.status === 'unsupported'
            ? (info.detail ?? '')
            : '',
  )
</script>

<span class="conn">
  <Button variant="ghost" small loading={info.busy} title={tips[kind]} onclick={() => RequestService.TestConnection(node)}>Test</Button>
  {#if words}
    <span class="verdict" class:compact={kind !== 'connect'} class:ok={info.status === 'ok'} class:bad={info.status === 'failed'} role="status" title={info.target ? `${info.target}\n${info.detail ?? ''}` : info.detail}>{words}</span>
  {/if}
</span>

<style>
  .conn { flex: none; display: flex; align-items: center; gap: 6px; padding-right: 6px; min-width: 0; }
  .verdict { font-size: var(--fs-xs); color: var(--muted); max-width: 22em; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  /* An HTTP, gRPC or file address is the part of the row that matters most, so its verdict stays short; the tooltip has the rest. */
  .verdict.compact { max-width: 11em; }
  .verdict.ok { color: var(--ok); }
  .verdict.bad { color: var(--fail); }
</style>
