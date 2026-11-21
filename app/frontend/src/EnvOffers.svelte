<script lang="ts">
  import Banner from './Banner.svelte'
  import Button from './Button.svelte'
  // A mounted collection that ships environments the workspace does not have
  // (an example, a colleague's folder) offers to add them. The workspace's
  // environments are global, so without this they would be silently unused.
  import { errorText } from './lib/errors'
  import { WorkspaceService } from '../bindings/github.com/githugt/flowi/app/internal/bridge'
  import { app } from './lib/app.svelte'
  import { ui } from './lib/ui.svelte'
  import type { EnvOffer } from './lib/types'

  let offers = $state<EnvOffer[]>([])
  let declined = $state<string[]>([]) // collections whose offer was declined this session

  // The offers change with the workspace: a mount, a new environment, an import.
  $effect(() => {
    void app.tree
    WorkspaceService.EnvironmentOffers().then((o) => (offers = o as EnvOffer[]))
  })

  const offer = $derived(offers.find((o) => !declined.includes(o.collection)))

  async function add(o: EnvOffer) {
    try {
      const added = (await WorkspaceService.ImportEnvironments(o.collection)) as string[]
      if (added.length && !app.tree?.selected) WorkspaceService.SelectEnvironment(added[0])
      ui.toast(`Added ${added.join(', ')}. Set secret values in each environment's tab.`)
    } catch (e) {
      ui.toast(errorText(e), 'error')
    }
  }

  const list = (names: string[]) => (names.length === 1 ? names[0] : `${names.slice(0, -1).join(', ')} and ${names.at(-1)}`)
</script>

{#if offer}
  <Banner>
    <strong>{offer.name}</strong> comes with {offer.environments.length === 1 ? 'an environment' : `${offer.environments.length} environments`}
    ({list(offer.environments)}) that this workspace does not have yet.
    {#snippet actions()}
      <Button variant="primary" small onclick={() => add(offer)}>Add {offer.environments.length === 1 ? 'it' : 'them'}</Button>
      <Button variant="ghost" small onclick={() => (declined = [...declined, offer.collection])}>Not now</Button>
    {/snippet}
  </Banner>
{/if}
