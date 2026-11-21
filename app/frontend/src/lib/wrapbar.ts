import { plural } from './plural'

export type WrapKind = 'loop' | 'group' | 'route'

/** The containers the bar offers, with what each does, for the buttons' tooltips. Go's vm.WrapKinds names the same three. */
export const wrapKinds: { kind: WrapKind; label: string; hint: string }[] = [
  { kind: 'loop', label: 'Loop', hint: 'Repeat these steps: you say over what next' },
  { kind: 'group', label: 'Group', hint: 'Keep these steps together in one group, with a second lane beside them' },
  { kind: 'route', label: 'Route', hint: 'Run these steps only when a condition holds (it starts as true)' },
]

/** Whether the bar shows: only while editing, and for two or more steps (one step has its own toolbar). */
export function showWrapBar(editing: boolean, selected: number): boolean {
  return editing && selected >= 2
}

/** The words before the buttons: "Wrap 3 steps in". */
export function wrapLabel(selected: number): string {
  return `Wrap ${plural(selected, 'step')} in`
}
