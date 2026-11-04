// Where the playhead is on a flow's finished run, by open flow, in milliseconds from its start. A flow with no entry
// shows the run as it ended. The run timeline strip moves it; the canvas and the variables follow it.
class Playback {
  at = $state<Record<string, number>>({})

  seek(node: string, ms: number) {
    this.at[node] = ms
  }

  /** Back to the run as it ended. */
  live(node: string) {
    delete this.at[node]
  }
}

export const playback = new Playback()
