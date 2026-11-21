/** What the footer needs to know about the services that keep working after their screen is closed. */
export interface ServiceState {
  replayer: { state: string; addr: string }
  schedules: { state: string; triggerAddr: string; upcoming: number }
}

/** A service shown in the footer: what it says, what it means, and the screen it opens. */
export interface ServiceChip {
  id: 'replayer' | 'schedules'
  text: string
  title: string
  screen: string
  /** starting and stopping are in between: the dot blinks. */
  busy: boolean
}

export const noServices: ServiceState = { replayer: { state: 'stopped', addr: '' }, schedules: { state: 'stopped', triggerAddr: '', upcoming: 0 } }

const running = (state: string) => state === 'running' || state === 'starting' || state === 'stopping'

/**
 * The chips for the services that are doing something, and none for a stopped one. Before: starting the replayer or the
 * schedules and then leaving their screen left nothing on screen to say they were still answering calls and firing runs.
 */
export function serviceChips(s: ServiceState): ServiceChip[] {
  const out: ServiceChip[] = []
  if (running(s.replayer.state)) {
    const where = s.replayer.addr ? ` ${s.replayer.addr}` : ''
    out.push({
      id: 'replayer',
      text: s.replayer.state === 'running' ? `Replayer${where}` : `Replayer ${s.replayer.state}…`,
      title: s.replayer.state === 'running' ? `The replayer is answering calls${where ? ' on' + where : ''}. Click to open it.` : `The replayer is ${s.replayer.state}.`,
      screen: 'screen:replayer',
      busy: s.replayer.state !== 'running',
    })
  }
  if (running(s.schedules.state)) {
    const n = s.schedules.upcoming
    out.push({
      id: 'schedules',
      text: s.schedules.state === 'running' ? `Schedules${n ? ` · ${n} due` : ''}` : `Schedules ${s.schedules.state}…`,
      title: s.schedules.state === 'running' ? `Schedules are firing while the window is open${s.schedules.triggerAddr ? '; triggers listen on ' + s.schedules.triggerAddr : ''}. Click to open them.` : `The schedules are ${s.schedules.state}.`,
      screen: 'screen:schedules',
      busy: s.schedules.state !== 'running',
    })
  }
  return out
}
