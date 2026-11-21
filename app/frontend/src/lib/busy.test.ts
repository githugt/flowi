import { describe, expect, it } from 'vitest'
import { BAR_AFTER_MS, PendingCalls, STUCK_AFTER_MS, busyState, callLabel, countingFetch, diagnosticsText } from './busy'
import app from '../App.svelte?raw'
import main from '../main?raw'
import indicator from './busy.svelte.ts?raw'

const call = (id: number) => JSON.stringify({ object: 0, method: 0, args: { 'call-id': 'x', methodID: id, args: ['y'.repeat(2000)] } })

describe('busyState', () => {
  it('is idle with nothing waiting and for a call that has only just gone out', () => {
    const calls = new PendingCalls()
    expect(busyState(calls.list(), 0).level).toBe('idle')
    calls.begin('a', 1000)
    expect(busyState(calls.list(), 1000 + BAR_AFTER_MS - 1).level).toBe('idle')
  })

  it('shows the bar after 400 ms and the notice after 5 s, measured on the oldest call', () => {
    const calls = new PendingCalls()
    calls.begin('old', 0)
    calls.begin('new', 4900)

    expect(busyState(calls.list(), BAR_AFTER_MS)).toMatchObject({ level: 'working', count: 2, oldest: 'old' })
    expect(busyState(calls.list(), STUCK_AFTER_MS)).toMatchObject({ level: 'stuck', seconds: 5, oldest: 'old' })
  })

  it('goes quiet when the slow call comes back', () => {
    const calls = new PendingCalls()
    const done = calls.begin('slow', 0)
    done()
    done()

    expect(busyState(calls.list(), 9000)).toMatchObject({ level: 'idle', count: 0 })
  })
})

describe('callLabel', () => {
  it('names a call to a bound method, reading only the start of its body', () => {
    expect(callLabel(call(1238036098))).toBe('method 1238036098')
    expect(callLabel('{"object":0,"method":0,"args":{"call-id":"x","methodName":"a.b.Open","args":[]}}')).toBe('a.b.Open')
  })

  it('ignores what cannot wait on a Go method: events, a cancel, a body that is not text', () => {
    expect(callLabel('{"object":3,"method":0,"args":{}}')).toBeNull()
    expect(callLabel('{"object":10,"method":0,"args":{"call-id":"x"}}')).toBeNull()
    expect(callLabel(new Uint8Array(2))).toBeNull()
    expect(callLabel(undefined)).toBeNull()
  })
})

describe('countingFetch', () => {
  const never = () => new Promise<Response>(() => {})

  it('counts a runtime call until its response comes, and answers whatever the real fetch answers', async () => {
    const live: string[] = []
    let resolve!: (r: Response) => void
    const real = () => new Promise<Response>((r) => (resolve = r))
    const f = countingFetch(real, (label) => {
      live.push(label)
      return () => void live.splice(live.indexOf(label), 1)
    })

    const answer = f('http://localhost/wails/runtime', { method: 'POST', body: call(7) })
    expect(live).toEqual(['method 7'])
    resolve(new Response('ok'))

    expect(await (await answer).text()).toBe('ok')
    expect(live).toEqual([])
  })

  it('stops counting when the call fails', async () => {
    const live: string[] = []
    const f = countingFetch(() => Promise.reject(new Error('down')), (l) => (live.push(l), () => void live.pop()))

    await expect(f('/wails/runtime', { method: 'POST', body: call(1) })).rejects.toThrow('down')
    expect(live).toEqual([])
  })

  it('leaves other requests alone', () => {
    const live: string[] = []
    const f = countingFetch(never, (l) => (live.push(l), () => {}))

    void f('/assets/app.js')
    void f('/wails/runtime', { method: 'POST', body: '{"object":3,"method":0}' })
    void f('/wails/runtime')

    expect(live).toEqual([])
  })
})

describe('diagnosticsText', () => {
  it('says what waits and for how long, without asking Go', () => {
    const calls = new PendingCalls()
    calls.begin('method 5', 1000)
    const text = diagnosticsText(calls.list(), 8000, 'Flowi 1.2')

    expect(text).toContain('1 call, the oldest 7 s (method 5)')
    expect(text).toContain('method 5: waiting 7 s')
    expect(text.endsWith('Flowi 1.2')).toBe(true)
  })
})

describe('wiring', () => {
  it('installs the counting fetch before the app mounts and shows the bar in the window', () => {
    expect(main.indexOf('busy.install(window)')).toBeGreaterThan(-1)
    expect(main.indexOf('busy.install(window)')).toBeLessThan(main.indexOf('mount(App'))
    expect(app).toContain('<Busy />')
  })
})

describe('busy indicator inside an $effect', () => {
  it('reads and writes its state untracked, so a call made from an effect does not re-run that effect', () => {
    // Before: noting a call read `state` and then changed it. A screen that asks Go from an $effect (RequestService.Missing
    // on every change of the request, and a dozen more) therefore depended on `state`, ran again after its own call,
    // and Svelte stopped it with effect_update_depth_exceeded, which the window reported as a frontend error.
    expect(indicator).toContain("import { untrack } from 'svelte'")
    expect(indicator).toContain('untrack(() => this.update())')
    expect(indicator).toMatch(/private refresh\(\) \{\s*untrack/)
  })
})
