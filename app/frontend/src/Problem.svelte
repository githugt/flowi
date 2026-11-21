<script lang="ts">
  // One or more problems from the library, in plain words, with its FLW code small at the end.
  import { codeParts, describeProblems, secretHostsOf } from './lib/problems'
  import Button from './Button.svelte'
  import { ManagerService } from '../bindings/github.com/githugt/flowi/app/internal/bridge'
  import { reported } from './lib/reported'
  import { ui } from './lib/ui.svelte'

  // A send stopped for taking a secret to a host nobody trusted offers to trust it: the host goes on the person's own list.
  let allowing = $state('')
  async function allow(host: string) {
    allowing = host
    try {
      await reported(ManagerService.AllowSecretHost(host), ui.toast, `Secrets may now be sent to ${host}. Send again.`)
    } finally {
      allowing = ''
    }
  }

  let { text }: { text: string } = $props()
  const problems = $derived(describeProblems(text))
</script>

{#each problems as p, i}{#if i > 0}<br />{/if}<span class="problem">{#each codeParts(p.text) as part}{#if part.code}<code>{part.text}</code>{:else}{part.text}{/if}{/each}{#if p.code}<span class="code" title="Problem code, for searching and bug reports">{p.code}</span>{/if}{#each secretHostsOf(p) as host}<Button small variant="ghost" loading={allowing === host} onclick={() => allow(host)}>Allow secrets to {host}</Button>{/each}</span>{/each}

<style>
  .problem { -webkit-user-select: text; user-select: text; }
  code { font-family: var(--mono); font-size: 0.95em; }
  .code { margin-left: 0.5em; color: var(--muted); font-size: var(--fs-xs); font-family: var(--mono); white-space: nowrap; }
</style>
