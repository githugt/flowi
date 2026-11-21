// What the replayer screen's top bar and tabs say, worked out from the replayer's state and its form.
// Before: the screen was one long form with Start at the bottom, and the faults block vanished while forwarding with
// nothing to say why.

export type ReplayerState = 'stopped' | 'starting' | 'running' | 'stopping'

export interface Headline {
  text: string
  tone: 'ok' | 'busy' | 'off'
}

/** The line at the top: where it listens, or that it is stopped or on its way. */
export function headline(state: ReplayerState, addr: string): Headline {
  switch (state) {
    case 'running':
      return { text: addr ? `Listening on ${addr}` : 'Listening', tone: 'ok' }
    case 'starting':
      return { text: 'Starting…', tone: 'busy' }
    case 'stopping':
      return { text: 'Stopping…', tone: 'busy' }
    default:
      return { text: 'Stopped', tone: 'off' }
  }
}

/** The address to copy for a client: a listen address with no host or an unspecified one (0.0.0.0, [::]) is reached on this machine. */
export function serveUrl(addr: string): string {
  const a = addr.trim()
  if (!a) return ''
  const port = a.slice(a.lastIndexOf(':'))
  const host = a.slice(0, a.lastIndexOf(':'))
  if (a.lastIndexOf(':') < 0) return 'http://' + a
  return 'http://' + (host === '' || host === '0.0.0.0' || host === '[::]' ? '127.0.0.1' : host) + port
}

export interface FaultForm {
  forward: string
  faultErrorPercent: string
  faultDropPercent: string
  faultJitter: string
}

/** Whether the faults can act, and when they cannot, why. They apply to calls a saved response answers, not to forwarded ones. */
export function faultsGate(f: Pick<FaultForm, 'forward'>): { enabled: boolean; reason: string } {
  if (!f.forward) return { enabled: true, reason: '' }
  return { enabled: false, reason: 'Faults only apply when calls are answered from saved responses. Choose Replay only on the Mode tab to use them.' }
}

/** How many of the faults are set, for the tab's badge. */
export function faultCount(f: FaultForm): number {
  return [Number(f.faultErrorPercent) > 0, Number(f.faultDropPercent) > 0, f.faultJitter.trim() !== ''].filter(Boolean).length
}
