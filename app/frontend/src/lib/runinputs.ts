// The values typed in the Run flow dialog: remembered for the next run of the same flow, and an input can be sent
// empty on purpose (a negative test such as "an empty email gives 422").
// Before: the dialog started blank each time, and an empty box was dropped, so an input could not be sent empty.

export interface LastInputs {
  values: Record<string, string>
  /** Inputs to send as the empty text, not leave unset. */
  empty: string[]
}

const last = new Map<string, LastInputs>()

/** What was used the last time this flow ran with inputs, or nothing. Only inputs the flow still has are kept. */
export function lastInputs(node: string, keys: string[]): LastInputs {
  const got = last.get(node)
  if (!got) return { values: {}, empty: [] }
  const has = new Set(keys)
  return {
    values: Object.fromEntries(Object.entries(got.values).filter(([k]) => has.has(k))),
    empty: got.empty.filter((k) => has.has(k)),
  }
}

export function rememberInputs(node: string, values: Record<string, string>, empty: string[]) {
  last.set(node, { values: { ...values }, empty: [...empty] })
}

/**
 * The inputs to hand to the run: each typed value, and an empty text for each input marked empty. A blank box that is
 * not marked is left out, so the flow's default applies. A marked input wins over what is typed in its box.
 */
export function inputsToSend(values: Record<string, string>, empty: string[]): Record<string, string> {
  const out = Object.fromEntries(Object.entries(values).filter(([, v]) => v !== ''))
  for (const k of empty) out[k] = ''
  return out
}
