// The value trace a flow's canvas is showing, by open flow: the Inspector starts one, the canvas dims to it.
import { FlowService } from '../../bindings/github.com/githugt/flowi/app/internal/bridge'
import type { TraceView } from './types'

class Tracing {
  byNode = $state<Record<string, TraceView>>({})

  async start(node: string, name: string) {
    this.byNode[node] = (await FlowService.Trace(node, name)) as unknown as TraceView
  }

  clear(node: string) {
    delete this.byNode[node]
  }
}

export const tracing = new Tracing()
