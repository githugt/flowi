<script lang="ts">
  import TextField from './TextField.svelte'
  import type { AuthField } from './lib/types'

  // The parameters of an auth scheme: the one form behind a request's own auth block and a named authentication.
  let { fields, onparam }: { fields: AuthField[]; onparam: (name: string, value: string) => void } = $props()
</script>

{#each fields as f (f.name)}
  <TextField label={f.name + (f.known ? '' : ' (not a parameter of this scheme)')} mono placeholder={f.secret ? '{{secret.' + f.name + '}}' : ''} value={f.value} oninput={(t) => onparam(f.name, t)} />
{/each}
<p class="muted">Every value is a template. Keep credentials out of the file: {'{{secret.name}}'} reads the vault. The request is signed or authenticated as it is sent.</p>

<style>
  .muted { color: var(--muted); font-size: var(--fs-sm); }
</style>
