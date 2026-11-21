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

/**
 * Why the User and Password boxes cannot write into the server field, or "" when they can.
 * Before: with a template server such as `{{env.database_url}}` both boxes took text, stored nothing and reset to
 * empty.
 */
export function credentialsBlocked(server: string): string {
  if (serverCredentials(server)) return ''
  const t = server.trim()
  if (t.includes('{{')) return `The login comes from ${t.length > 60 ? 'the server template' : t}. Edit that value in Environments.`
  return 'The server is not a connection URL (scheme://host), so there is no place for a login.'
}

/**
 * Moves each `{{expr}}` of a statement into the Values list and puts a placeholder in its place, so the value
 * travels apart from the statement. `$1, $2 …` drivers (PostgreSQL, SQLite and the file drivers) get the next
 * number; `?` drivers get a `?` and the value at that position. Returns null when there is nothing to move.
 * Before: `{{id}}` in a query was coloured as a template and then refused at send time as a dataset request error.
 */
export function moveTemplatesToValues(query: string, params: string[], driver: string): { query: string; params: string[] } | null {
  const numbered = driver === 'postgres' || driver === 'sqlite' || isFileDriver(driver)
  const values = [...params]
  let seen = 0 // `?` placeholders so far, written or moved
  let out = ''
  let quote = ''
  let moved = false
  for (let i = 0; i < query.length; i++) {
    const c = query[i]
    if (quote) {
      out += c
      if (c === quote) quote = ''
    } else if (c === "'" || c === '"') {
      quote = c
      out += c
    } else if (c === '{' && query[i + 1] === '{' && query.indexOf('}}', i) >= 0) {
      const end = query.indexOf('}}', i)
      const expr = query.slice(i, end + 2)
      if (numbered) {
        values.push(expr)
        out += '$' + values.length
      } else {
        values.splice(seen, 0, expr)
        seen++
        out += '?'
      }
      moved = true
      i = end + 1
    } else {
      if (c === '?' && !numbered) seen++
      out += c
    }
  }
  return moved ? { query: out, params: values } : null
}

/**
 * How many values a statement needs: the highest `$N` of a numbered placeholder, or the count of `?` for the drivers
 * that use it (SQLite and the file drivers take either, so the larger of the two). Quoted text, comments and
 * `{{templates}}` are not placeholders. PostgreSQL's `?` is an operator and is not counted.
 * Before: adding or removing a `$3` said nothing about the Values tab, and the mismatch showed as a driver error on send.
 */
export function placeholderCount(query: string, driver: string): number {
  const questions = driver !== 'postgres'
  let highest = 0
  let marks = 0
  let quote = ''
  for (let i = 0; i < query.length; i++) {
    const c = query[i]
    if (quote) {
      if (c === quote) quote = ''
    } else if (c === "'" || c === '"') quote = c
    else if (c === '-' && query[i + 1] === '-') i = query.indexOf('\n', i) < 0 ? query.length : query.indexOf('\n', i)
    else if (c === '/' && query[i + 1] === '*') i = query.indexOf('*/', i) < 0 ? query.length : query.indexOf('*/', i) + 1
    else if (c === '{' && query[i + 1] === '{' && query.indexOf('}}', i) >= 0) i = query.indexOf('}}', i) + 1
    else if (c === '$') {
      const m = /^\d+/.exec(query.slice(i + 1))
      if (m) {
        highest = Math.max(highest, Number(m[0]))
        i += m[0].length
      }
    } else if (c === '?' && questions) marks++
  }
  return Math.max(highest, marks)
}

/** What to say when the number of values is not the number of placeholders, and how many are missing; null when they agree. */
export function valuesNote(query: string, driver: string, values: number): { text: string; missing: number } | null {
  const need = placeholderCount(query, driver)
  if (need === values) return null
  const word = (n: number, w: string) => `${n} ${w}${n === 1 ? '' : 's'}`
  const text = `The query has ${word(need, 'placeholder')} and ${word(values, 'value')}.`
  return { text: need > values ? text : text + ' The extra values are sent and may be refused.', missing: Math.max(0, need - values) }
}
