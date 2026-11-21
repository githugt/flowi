// Time left until a moment, for a sign-in code that expires: "4:32", or "" when there is no deadline and
// "expired" when it has passed.
// Before: the device-code dialog gave no sign that the code would stop working.
export function countdown(expires: string | undefined, now: number): string {
  if (!expires) return ''
  const at = Date.parse(expires)
  if (Number.isNaN(at)) return ''
  const left = Math.ceil((at - now) / 1000)
  if (left <= 0) return 'expired'
  return `${Math.floor(left / 60)}:${String(left % 60).padStart(2, '0')}`
}
