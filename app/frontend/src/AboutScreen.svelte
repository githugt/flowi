<script lang="ts">
  // What this is and which build: the logo, the version and where the project lives.
  import { onMount } from 'svelte'
  import { Browser } from '@wailsio/runtime'
  import { WelcomeService } from '../bindings/github.com/githugt/flowi/app/internal/bridge'
  import { live } from './lib/live'
  import Logo from './Logo.svelte'

  const REPOSITORY = 'https://github.com/githugt/flowi'

  let version = $state('')
  onMount(() => live<{ version: string }>('welcome', () => WelcomeService.State(), (v) => (version = v.version)))
</script>

<div class="page">
  <div class="about">
    <Logo size={72} />
    <h2>Flowi</h2>
    <p class="muted">Version {version}</p>
    <p class="muted">Send requests, build flows and replay APIs. Your files stay on this machine.</p>
    <p><a href={REPOSITORY} onclick={(e) => { e.preventDefault(); Browser.OpenURL(REPOSITORY) }}>{REPOSITORY}</a></p>
    <p class="muted">Licensed under the AGPL-3.0-or-later.</p>
  </div>
</div>

<style>
  .page { position: absolute; inset: 0; overflow: auto; display: grid; place-items: center; padding: 28px 32px 56px; }
  .about { display: flex; flex-direction: column; align-items: center; gap: 4px; text-align: center; max-width: 460px; }
  h2 { margin: 8px 0 0; }
  p { margin: 0; }
  .muted { color: var(--muted); font-size: var(--fs-sm); }
  a { color: var(--accent); }
</style>
