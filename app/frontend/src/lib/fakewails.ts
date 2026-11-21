// A fake of what the window talks to: Go's services (the generated bindings) and its events (the Wails runtime), for
// tests of the state classes that hold the races (app.svelte.ts, ui.svelte.ts).
// Before: there was none, so those classes were tested only by reading their source (`?raw` tests); an order of
// answers that loses a click or an event could not be written down.
//
// A test mocks the two modules once and then drives the fake:
//
//   vi.mock('@wailsio/runtime', async () => (await import('./fakewails')).fake.runtime())
//   vi.mock('../../bindings/github.com/githugt/flowi/app/internal/bridge', async () => (await import('./fakewails')).fake.bridge())
//
//   fake.answer('FlowService.Open', () => view)          // what a method answers (a promise is awaited)
//   const slow = fake.deferred<FlowView>()               // an answer the test settles when it chooses
//   fake.answer('RequestService.Open', () => slow.promise)
//   fake.emit('request', data)                           // an event from Go
//   fake.called('RequestService.Save', node)             // what the window asked
//
// A method with no answer resolves to undefined. Nothing here sleeps: order is the test's to set.

/** Every service the bindings export; the mock module must name them all. */
export const SERVICE_NAMES = [
  'AuthsService', 'BlueprintsService', 'ChecksService', 'EnvironmentService', 'FlowService', 'HooksService', 'ImportService',
  'ManagerService', 'PanelsService', 'PassphraseService', 'ReplayerService', 'RequestService', 'SchedulesService',
  'SecretsService', 'SessionService', 'SettingsService', 'SystemService', 'TracingService', 'TrashService', 'WelcomeService',
  'WorkspaceService',
] as const

export interface Call {
  /** "FlowService.Open" */
  name: string
  args: unknown[]
}

export interface Deferred<T> {
  promise: Promise<T>
  resolve: (v: T) => void
  reject: (e: unknown) => void
}

type Handler = (e: { data: unknown }) => void

class FakeWails {
  calls: Call[] = []
  /** What the window sent to Go with Events.Emit. */
  emitted: { name: string; data: unknown }[] = []
  #answers = new Map<string, (...args: any[]) => unknown>()
  #handlers = new Map<string, Set<Handler>>()

  /** Forgets calls, answers and listeners: each test starts from a window that has heard nothing. */
  reset() {
    this.calls = []
    this.emitted = []
    this.copied = []
    this.opened = []
    this.#answers.clear()
    this.#handlers.clear()
  }

  /** What `Service.Method` answers; a returned promise is awaited by the caller, and a throw rejects the call. */
  answer(name: string, fn: (...args: any[]) => unknown) {
    this.#answers.set(name, fn)
  }

  /** A promise the test settles itself, to put an answer after a later one. */
  deferred<T>(): Deferred<T> {
    let resolve!: (v: T) => void
    let reject!: (e: unknown) => void
    const promise = new Promise<T>((res, rej) => ((resolve = res), (reject = rej)))
    return { promise, resolve, reject }
  }

  /** The calls made to `Service.Method`, oldest first. */
  callsTo(name: string): unknown[][] {
    return this.calls.filter((c) => c.name === name).map((c) => c.args)
  }

  /** Whether `Service.Method` was called, with these arguments when given. */
  called(name: string, ...args: unknown[]): boolean {
    return this.callsTo(name).some((a) => args.length === 0 || (a.length === args.length && args.every((x, i) => Object.is(x, a[i]) || JSON.stringify(x) === JSON.stringify(a[i]))))
  }

  /** An event from Go, heard by every listener of that name. */
  emit(name: string, data?: unknown) {
    for (const h of [...(this.#handlers.get(name) ?? [])]) h({ data })
  }

  /** How many listeners an event has. */
  listeners(name: string): number {
    return this.#handlers.get(name)?.size ?? 0
  }

  /** The stand-in for the Wails runtime's Events. */
  events = {
    On: (name: string, handler: Handler) => {
      const set = this.#handlers.get(name) ?? new Set<Handler>()
      set.add(handler)
      this.#handlers.set(name, set)
      return () => void set.delete(handler)
    },
    Emit: (name: string, data?: unknown) => {
      this.emitted.push({ name, data })
    },
  }

  /** Text the window put on the clipboard, and the addresses it asked the system browser to open. */
  copied: string[] = []
  opened: string[] = []

  /** The stand-in for the parts of the Wails runtime the window uses. */
  runtime() {
    return {
      Events: this.events,
      Clipboard: { SetText: async (text: string) => void this.copied.push(text) },
      Browser: { OpenURL: async (url: string) => void this.opened.push(url) },
    }
  }

  /** The stand-in for the bindings module: every service, each method recording its call and giving its answer. */
  bridge(): Record<string, unknown> {
    const out: Record<string, unknown> = {}
    for (const service of SERVICE_NAMES) {
      out[service] = new Proxy(
        {},
        {
          get: (_, method) => {
            if (typeof method !== 'string') return undefined
            const name = `${service}.${method}`
            return (...args: unknown[]) => {
              this.calls.push({ name, args })
              return Promise.resolve().then(() => this.#answers.get(name)?.(...args))
            }
          },
        },
      )
    }
    return out
  }
}

// One instance for the page, so that the module the mocks load and the module a test imports are the same one even
// after vi.resetModules().
const slot = globalThis as { __fakeWails?: FakeWails }
export const fake: FakeWails = (slot.__fakeWails ??= new FakeWails())
