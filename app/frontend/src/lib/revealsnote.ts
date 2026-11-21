/** "Revealed 3 times this session": a secret shown on screen is something the person should be able to count. */
export function revealsNote(n: number): string {
  return n === 1 ? 'Revealed 1 time this session.' : `Revealed ${n} times this session.`
}
