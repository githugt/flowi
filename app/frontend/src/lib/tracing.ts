/** "just now", "12 s ago", "5 min ago", "2 h ago": how long before `now` an RFC 3339 time was. */
export function agoText(iso: string, now: number): string {
  const secs = Math.max(0, Math.round((now - Date.parse(iso)) / 1000))
  if (Number.isNaN(secs)) return ''
  if (secs < 5) return 'just now'
  if (secs < 60) return `${secs} s ago`
  if (secs < 3600) return `${Math.floor(secs / 60)} min ago`
  if (secs < 86400) return `${Math.floor(secs / 3600)} h ago`
  return `${Math.floor(secs / 86400)} d ago`
}
