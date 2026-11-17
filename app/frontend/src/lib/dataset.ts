/** The drivers that read data files (a file or a folder of them) rather than connect to a database server. */
export const FILE_DRIVERS = ['csv', 'xlsx', 'parquet']
export const isFileDriver = (driver: string): boolean => FILE_DRIVERS.includes(driver)

/** Whether a connection URL spells out a password (scheme://user:password@host) rather than naming a secret. */
export function serverHasPassword(server: string): boolean {
  const m = /^[a-z][a-z0-9+.-]*:\/\/[^/@\s:]*:([^@\s/]*)@/i.exec(server.trim())
  return !!m && m[1] !== '' && !m[1].startsWith('{{')
}

/** The password written in a connection URL, and the URL with `{{secret.<name>}}` in its place; null when none is written. */
export function moveServerPassword(server: string, name: string): { password: string; server: string } | null {
  if (!serverHasPassword(server)) return null
  const m = /^(\s*[a-z][a-z0-9+.-]*:\/\/[^/@\s:]*:)([^@\s/]*)(@.*)$/is.exec(server)
  if (!m) return null
  return { password: m[2], server: `${m[1]}{{secret.${name}}}${m[3]}` }
}

const URL_PARTS = /^(\s*[a-z][a-z0-9+.-]*:\/\/)([^/@\s]*@)?(.*)$/is

/** The user and password written in a connection URL's userinfo; null when the server is not a URL. */
export function serverCredentials(server: string): { user: string; password: string } | null {
  const m = URL_PARTS.exec(server)
  if (!m) return null
  const info = (m[2] ?? '').slice(0, -1)
  const i = info.indexOf(':')
  return i < 0 ? { user: info, password: '' } : { user: info.slice(0, i), password: info.slice(i + 1) }
}

/** The connection URL with the given user and password in place of its own; unchanged when it is not a URL. */
export function withServerCredentials(server: string, user: string, password: string): string {
  const m = URL_PARTS.exec(server)
  if (!m) return server
  const info = user || password ? user + (password ? ':' + password : '') + '@' : ''
  return m[1] + info + m[3]
}
