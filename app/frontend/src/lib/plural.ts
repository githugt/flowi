// A count with its noun, in the right number: "1 item", "0 items", "2 items".
// Before: 46 inline ternaries, some written `n > 1`, which reads zero as one ("0 send"), and the JSON viewer
// wrote "[1 items]" for every one-item array.
const rules = new Intl.PluralRules('en')

/** `plural(1, 'item')` is "1 item"; `plural(2, 'item')` is "2 items". Give the plural when it is not the noun plus s. */
export function plural(n: number, one: string, many = `${one}s`): string {
  return `${n} ${noun(n, one, many)}`
}

/** The noun alone, in the number n asks for. */
export function noun(n: number, one: string, many = `${one}s`): string {
  return rules.select(n) === 'one' ? one : many
}
