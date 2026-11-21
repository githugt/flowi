<script lang="ts">
  // A name edited where it is shown. Enter or leaving the field keeps the new name, Escape puts the old one back, and a
  // refusal (a name taken, an id with a space) is said under the field while the typed text stays for mending.
  import TextInput from './TextInput.svelte'
  import { errorText } from './lib/errors'
  import { inlineKey, nameToKeep, shownName } from './lib/inlinename'

  let { value, label, commit, mono = false }: { value: string; label: string; commit: (name: string) => Promise<unknown> | unknown; mono?: boolean } = $props()

  let draft = $state('')
  // Editing lasts from the first key until the name is kept or put back; a refused name stays in the field meanwhile.
  let editing = $state(false)
  let error = $state('')
  let el = $state<HTMLInputElement>()
  const shown = $derived(shownName(draft, value, editing))

  async function keep() {
    const next = nameToKeep(draft, value)
    if (next === null) return revert()
    try {
      await commit(next)
      editing = false
      error = ''
    } catch (e) {
      error = errorText(e)
    }
  }
  function revert() {
    editing = false
    error = ''
    draft = value
  }
  function keys(e: KeyboardEvent) {
    const a = inlineKey(e.key, e.isComposing)
    if (a === 'commit') {
      e.preventDefault()
      void keep()
    } else if (a === 'revert') {
      e.preventDefault()
      revert()
      el?.blur()
    }
  }
</script>

<label class="inline">
  <span class="label">{label}</span>
  <TextInput
    bind:el
    {mono}
    {label}
    {error}
    value={shown}
    oninput={(e) => {
      if (!editing) draft = value
      editing = true
      draft = (e.currentTarget as HTMLInputElement).value
    }}
    onkeydown={keys}
    onblur={() => editing && void keep()}
  />
</label>

<style>
  .inline { display: flex; flex-direction: column; gap: 3px; }
  .label { color: var(--muted); font-size: var(--fs-xs); }
</style>
