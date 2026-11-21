/**
 * A font family as CSS takes it: one name is quoted (so "Source Code Pro" and a name that starts with a digit
 * work); a list the person wrote, with commas or quotes, is kept as it is. A name with a quote or a semicolon in it
 * is refused (""), since it could end the declaration.
 * Before: `appearance.font` was written as a CSS family but described as a file, so a path did nothing.
 */
export function quoteFamily(name: string): string {
  const t = name.trim()
  if (!t || /[;{}\\]/.test(t)) return ''
  if (t.includes(',') || /^["'].*["']$/.test(t)) return t
  if (/["']/.test(t)) return ''
  return `"${t}"`
}
