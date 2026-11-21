<script lang="ts">
  // What this is and which build: the logo, the version and where the project lives.
  import { onMount } from 'svelte'
  import { openExternal } from './lib/openurl'
  import { SystemService, WelcomeService, WorkspaceService } from '../bindings/github.com/githugt/flowi/app/internal/bridge'
  import { copy } from './lib/clip'
  import { errorText } from './lib/errors'
  import { ui } from './lib/ui.svelte'
  import { app } from './lib/app.svelte'
  import Button from './Button.svelte'
  import { live } from './lib/live'
  import Logo from './Logo.svelte'

  const REPOSITORY = 'https://github.com/githugt/flowi'

  let version = $state('')
  onMount(() => live<{ version: string }>('welcome', () => WelcomeService.State(), (v) => (version = v.version)))

  // What a bug report needs: version, system, workspace folder and counts. No secret and no value of any environment.
  const copyDiagnostics = async () => copy(await SystemService.Diagnostics(), 'the diagnostics')
  const openFolder = () => WorkspaceService.RevealWorkspace().catch((e) => ui.toast(errorText(e), 'error'))
</script>

<div class="page">
  <div class="about">
    <Logo size={72} />
    <h2>Flowi</h2>
    <p class="muted">Version {version}</p>
    <p class="muted">Send requests, build flows and replay APIs. Your files stay on this machine.</p>
    <p><a href={REPOSITORY} onclick={(e) => { e.preventDefault(); openExternal(REPOSITORY) }}>{REPOSITORY}</a></p>
    <p class="muted">Licensed under the AGPL-3.0-or-later.</p>
    <div class="actions">
      <Button small onclick={copyDiagnostics} title="Version, system, workspace folder and counts: no secrets, no values from your requests">Copy diagnostics</Button>
      <Button small onclick={openFolder} disabled={!app.tree?.folder}>Open workspace folder</Button>
    </div>
  </div>
</div>

<style>
  .page { position: absolute; inset: 0; overflow: auto; display: grid; place-items: center; padding: 28px 32px 56px; }
  .about { display: flex; flex-direction: column; align-items: center; gap: 4px; text-align: center; max-width: 460px; }
  h2 { margin: 8px 0 0; }
  p { margin: 0; }
  .muted { color: var(--muted); font-size: var(--fs-sm); }
  a { color: var(--accent); }
  .actions { display: flex; gap: var(--sp-2); margin-top: var(--sp-3); flex-wrap: wrap; justify-content: center; }
</style>
