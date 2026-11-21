// Whether an event carries what the app already holds. Go sends a whole view on every change, and often the same one
// again (a save that changed nothing, a refresh): replacing a deep $state object with an equal copy still wakes every
// derived value and effect that reads it.
//
// Compared as JSON: the views are plain data from Go, whose struct fields keep one order. A value JSON cannot hold
// (a function, a cycle) counts as changed, so the worst case is the old behaviour.
export function sameData(held: unknown, incoming: unknown): boolean {
  if (held === incoming) return true
  if (held === undefined || held === null || incoming === undefined || incoming === null) return false
  try {
    return JSON.stringify(held) === JSON.stringify(incoming)
  } catch {
    return false
  }
}
