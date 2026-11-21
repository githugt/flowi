// What a subflow step can tell about the flow it calls: the inputs it sets, the outputs later steps read.
// Before: the called flow's inputs and outputs were invisible, "with" was free text, and a missing or misspelt input
// was found when the flow ran.
export interface FlowIO {
  name: string
  type?: string
  default?: string
  required?: boolean
}

export interface SubflowNotes {
  /** Inputs the step sets that the flow does not have. */
  unknown: string[]
  /** Inputs with no default that the step does not set. */
  missing: string[]
  /** Each input as it is listed to the person: "user (string, required)", "page (int, default 1)". */
  inputs: string[]
  /** Each output as a later step reads it: "nodes.signin.token". */
  outputs: string[]
}

const label = (i: FlowIO): string => {
  const detail = [i.type, i.required ? 'required' : i.default ? `default ${i.default}` : ''].filter(Boolean).join(', ')
  return detail ? `${i.name} (${detail})` : i.name
}

/** Compares the keys of "with" to the called flow's inputs. Empty lists when the flow is not known. */
export function subflowNotes(stepId: string, withKeys: string[], inputs: FlowIO[] = [], outputs: FlowIO[] = []): SubflowNotes {
  const set = new Set(withKeys.filter((k) => k.trim() !== ''))
  const known = new Set(inputs.map((i) => i.name))
  return {
    unknown: inputs.length || outputs.length ? [...set].filter((k) => !known.has(k)) : [],
    missing: inputs.filter((i) => i.required && !set.has(i.name)).map((i) => i.name),
    inputs: inputs.map(label),
    outputs: outputs.map((o) => `nodes.${stepId}.${o.name}`),
  }
}
