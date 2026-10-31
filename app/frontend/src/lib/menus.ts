// The tree's context menu: the entries come from Go; this runs them, asking first where they say to.
import { BlueprintsService, WorkspaceService } from '../../bindings/github.com/githugt/flowi/app/internal/bridge'
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
      protocolsLabel: e.action === 'newDataset' ? 'Database' : undefined,
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

// The "New" menu: quick starts and creation in a chosen collection or folder.
export function newMenu(): MenuItem[] {
  const ask = (kind: string, title: string, tree: 'requests' | 'flows', label: string, withProtocol = false, choices = protocols, choiceLabel?: string) => () => {
    ui.place = {
      title,
      action: 'Create',
      tree,
      name: { label, initial: '' },
      protocols: withProtocol ? choices : undefined,
      protocolsLabel: choiceLabel,
      note: 'Leave the name empty for the next "untitled".',
      done: (row, name, protocol) => WorkspaceService.NewAt(kind, row, name, protocol),
    }
  }
  return [
    { label: 'New request', run: () => WorkspaceService.NewScratchRequest() },
    { label: 'New flow', run: () => WorkspaceService.NewScratchFlow() },
    { label: 'New message request', run: newScratchMessage },
    { label: 'New gRPC request', run: () => WorkspaceService.NewAt('grpc', '', '', '') },
    { label: 'New dataset query', run: newScratchDataset },
    { separator: true, label: '', run: () => {} },
    { label: 'New request in a collection…', run: ask('request', 'New request', 'requests', 'Name or path (for example users/create)') },
    { label: 'New message request in a collection…', run: ask('async', 'New message request', 'requests', 'Name or path (for example orders/placed)', true) },
    { label: 'New gRPC request in a collection…', run: ask('grpc', 'New gRPC request', 'requests', 'Name or path (for example shop/get-order)') },
    { label: 'New dataset query in a collection…', run: ask('dataset', 'New dataset query', 'requests', 'Name or path (for example customers/by-id)', true, databases, 'Database') },
    { label: 'New flow in a collection…', run: ask('flow', 'New flow', 'flows', 'Name or path (for example journeys/signup)') },
    { label: 'New flow from blueprint…', run: () => (ui.blueprint = { collection: '', folder: '' }) },
    { separator: true, label: '', run: () => {} },
    { label: 'New environment…', run: promptNewEnvironment },
    { label: 'New collection…', run: () => app.openScreen('screen:manager', 'Collections') },
  ]
}

const protocols = [
  ['ws', 'WebSocket'], ['sse', 'Server-Sent Event'], ['mqtt', 'MQTT'], ['kafka', 'Kafka'], ['amqp', 'AMQP (RabbitMQ)'], ['amqp10', 'AMQP 1.0 (Artemis, Qpid…)'], ['nats', 'NATS'],
  ['redis', 'Redis'], ['sqs', 'AWS SQS'], ['sns', 'AWS SNS'], ['servicebus', 'Azure Service Bus'], ['pubsub', 'Google Pub/Sub'],
].map(([id, label]) => ({ id, label }))

const databases = [
  ['postgres', 'PostgreSQL'], ['mysql', 'MySQL'], ['mariadb', 'MariaDB'], ['sqlite', 'SQLite'],
].map(([id, label]) => ({ id, label }))

// A dataset query needs its database before it can be made; the draft goes to Scratch like New request.
function newScratchDataset() {
  ui.prompt({
    title: 'New dataset query',
    label: 'Name (blank for untitled)',
    initial: '',
    protocols: databases,
    protocolsLabel: 'Database',
    done: (path, driver) => WorkspaceService.NewAt('dataset', '', path, driver),
  })
}

// A message request needs its protocol before it can be made; the draft goes to Scratch like New request.
function newScratchMessage() {
  ui.prompt({
    title: 'New message request',
    label: 'Name (blank for untitled)',
    initial: '',
    protocols,
    done: (path, protocol) => WorkspaceService.NewAt('async', '', path, protocol),
  })
}

/** Asks for a name and creates an environment in the workspace; its tab opens. */
export function promptNewEnvironment() {
  ui.prompt({
    title: 'New environment',
    label: 'Name',
    initial: '',
    done: (name) => WorkspaceService.Do('newEnvironment', ['environments'], name, ''),
  })
}
