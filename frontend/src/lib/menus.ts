// The tree's context menu: the entries come from Go; this runs them, asking first where they say to.
import { BlueprintsService, WorkspaceService } from '../../bindings/github.com/githugt/flowi/internal/bridge'
import { app } from './app.svelte'
import { ui, type MenuItem } from './ui.svelte'
import type { MenuEntry } from './types'

const needsTabsClosed = ['rename', 'delete', 'discardDraft']

export async function rowMenu(id: string): Promise<MenuItem[]> {
  const ids = app.marked.includes(id) ? [...app.marked] : [id]
  if (!app.marked.includes(id)) app.marked = []
  const entries = (await WorkspaceService.Menu(ids)) as unknown as MenuEntry[]
  const items: MenuItem[] = entries.map((e) => ({ label: e.label, run: () => run(e, ids) }))
  // Wherever a flow can be made, it can also be made from a blueprint, there.
  const at = items.findIndex((_, i) => entries[i].action === 'newFlow')
  if (at >= 0 && ids.length === 1) {
    const where = (await BlueprintsService.Where(ids[0])) as unknown as { collection: string; folder: string }
    items.splice(at + 1, 0, { label: 'New flow from blueprint…', run: () => (ui.blueprint = where) })
  }
  return items
}

async function run(e: MenuEntry, ids: string[]) {
  if (e.guard === 'dirty' && (await touchesUnsaved(ids))) {
    ui.toast('Save or close the unsaved tabs of these rows first.', 'error')
    return
  }
  const go = async (text = '', protocol = '') => {
    if (needsTabsClosed.includes(e.action)) await app.closeAll(await tabsAffected(ids))
    await WorkspaceService.Do(e.action, ids, text, protocol)
  }
  if (e.dialog === 'move') {
    ui.moving = ids
  } else if (e.prompt) {
    ui.prompt({
      title: e.prompt.title,
      label: e.prompt.label,
      initial: e.prompt.initial,
      protocols: e.prompt.protocols?.map((p) => ({ id: p.id, label: p.label })),
      done: (text, protocol) => go(text, protocol),
    })
  } else if (e.confirm) {
    ui.confirm('Delete', e.confirm, () => go())
  } else {
    await go()
  }
}

export async function tabsAffected(ids: string[]): Promise<string[]> {
  const out = new Set(ids)
  for (const id of ids) for (const inner of (await WorkspaceService.Inside(id)) as string[]) out.add(inner)
  return [...out]
}

export async function touchesUnsaved(ids: string[]): Promise<boolean> {
  return (await tabsAffected(ids)).some((id) => app.isDirty(id))
}

// The "New" menu: quick starts and creation in a chosen collection.
export function newMenu(): MenuItem[] {
  const collections = (app.tree?.roots ?? []).filter((n) => n.kind === 'collection').map((n) => ({ id: n.id, label: n.label }))
  const where = collections.length ? { label: 'Collection', options: collections } : undefined
  const make = (kind: string, title: string, placeholder = '') => () =>
    ui.prompt({
      title,
      label: `Path (for example ${placeholder})`,
      initial: '',
      select: where,
      protocols: kind === 'async' ? protocols : undefined,
      done: (path, protocol, collection) => WorkspaceService.NewIn(kind, collection, path, protocol),
    })
  return [
    { label: 'New request', run: () => WorkspaceService.NewScratchRequest() },
    { label: 'New flow', run: () => WorkspaceService.NewScratchFlow() },
    { separator: true, label: '', run: () => {} },
    { label: 'New request in a collection…', run: make('request', 'New request', 'users/create'), disabled: !where },
    { label: 'New message request…', run: make('async', 'New message request', 'orders/placed'), disabled: !where },
    { label: 'New gRPC request…', run: make('grpc', 'New gRPC request', 'shop/get-order'), disabled: !where },
    { label: 'New flow in a collection…', run: make('flow', 'New flow', 'journeys/signup'), disabled: !where },
    { label: 'New flow from blueprint…', run: () => (ui.blueprint = { collection: '', folder: '' }) },
    { separator: true, label: '', run: () => {} },
    { label: 'New environment…', run: promptNewEnvironment },
    {
      label: 'New collection…',
      run: () => app.openScreen('screen:manager', 'Collections'),
    },
  ]
}

const protocols = [
  ['ws', 'WebSocket'], ['sse', 'Server-Sent Event'], ['mqtt', 'MQTT'], ['kafka', 'Kafka'], ['amqp', 'AMQP (RabbitMQ)'], ['amqp10', 'AMQP 1.0 (Artemis, Qpid…)'], ['nats', 'NATS'],
  ['redis', 'Redis'], ['sqs', 'AWS SQS'], ['sns', 'AWS SNS'], ['servicebus', 'Azure Service Bus'], ['pubsub', 'Google Pub/Sub'],
].map(([id, label]) => ({ id, label }))

/** Asks for a name and creates an environment in the workspace; its tab opens. */
export function promptNewEnvironment() {
  ui.prompt({
    title: 'New environment',
    label: 'Name',
    initial: '',
    done: (name) => WorkspaceService.Do('newEnvironment', ['environments'], name, ''),
  })
}
