// A size as "512 B", "1.2 KB" or "3.4 MB", the same way Go writes it (vm.SizeText): units of 1000, one decimal.
// Before: the message feed and the stored responses divided by 1024, so one body had two sizes on one screen, and
// the feed said "5120.0 KB" for a 5 MB message.
export function sizeText(n: number): string {
  if (n < 0 || !Number.isFinite(n)) return ''
  if (n < 1000) return `${Math.trunc(n)} B`
  if (n < 1_000_000) return `${(n / 1000).toFixed(1)} KB`
  return `${(n / 1_000_000).toFixed(1)} MB`
}
