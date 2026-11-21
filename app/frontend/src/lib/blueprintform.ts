/** A blueprint parameter as the dialog knows it. */
export interface ParamSpec {
  name: string
  default: string
}

/**
 * The parameters the person has to fill in: those with no default whose box is still empty. A flow made with one of
 * them blank has a hole where a request or a value should be.
 * Before: Create worked with every required parameter blank and gave no word about it.
 */
export function missingParams(params: readonly ParamSpec[], values: Readonly<Record<string, string>>): string[] {
  return params.filter((p) => p.default === '' && (values[p.name] ?? '').trim() === '').map((p) => p.name)
}
