import { describe, expect, it } from 'vitest'
import { credentialsBlocked, moveServerPassword, moveTemplatesToValues, placeholderCount, valuesNote, serverCredentials, serverHasPassword, withServerCredentials } from './dataset'

describe('serverHasPassword', () => {
  it('sees a password written into the URL', () => {
    expect(serverHasPassword('postgres://reader:hunter2@db.example.test:5432/app')).toBe(true)
  })
  it('accepts a password that is a secret reference', () => {
    expect(serverHasPassword('postgres://reader:{{secret.db_password}}@db.example.test/app')).toBe(false)
  })
  it('accepts a URL with no password, a template or a file', () => {
    expect(serverHasPassword('postgres://reader@db.example.test/app')).toBe(false)
    expect(serverHasPassword('{{env.database_url}}')).toBe(false)
    expect(serverHasPassword('shop.db')).toBe(false)
  })
})

describe('moveServerPassword', () => {
  it('replaces the written password with the secret and hands the password back as written', () => {
    expect(moveServerPassword('postgres://reader:p%40ss@host:5432/db?sslmode=disable', 'db_password')).toEqual({
      password: 'p%40ss',
      server: 'postgres://reader:{{secret.db_password}}@host:5432/db?sslmode=disable',
    })
  })
  it('leaves a URL without a written password alone', () => {
    expect(moveServerPassword('postgres://reader:{{secret.x}}@host/db', 'y')).toBeNull()
    expect(moveServerPassword('postgres://host/db', 'y')).toBeNull()
  })
})

describe('server credentials', () => {
  it('reads the user and password of a URL', () => {
    expect(serverCredentials('postgres://ada:{{secret.pw}}@host:5432/db')).toEqual({ user: 'ada', password: '{{secret.pw}}' })
    expect(serverCredentials('postgres://host/db')).toEqual({ user: '', password: '' })
    expect(serverCredentials('data.db')).toBeNull()
  })
  it('puts a user and password into a URL, and takes them out again', () => {
    expect(withServerCredentials('postgres://host:5432/db?x=1', 'ada', '{{secret.pw}}')).toBe('postgres://ada:{{secret.pw}}@host:5432/db?x=1')
    expect(withServerCredentials('postgres://old:pw@host/db', 'new', '')).toBe('postgres://new@host/db')
    expect(withServerCredentials('postgres://old:pw@host/db', '', '')).toBe('postgres://host/db')
  })
})

describe('serverHasPassword for an HTTP URL', () => {
  it('sees a password in a web address, and not one that names a secret', () => {
    // Before: only dataset connection strings showed a "Password in the file" sign; an HTTP URL showed a toast once.
    expect(serverHasPassword('https://ada:hunter2@api.example.com/pets')).toBe(true)
    expect(serverHasPassword('https://ada:{{secret.pw}}@api.example.com/pets')).toBe(false)
    expect(serverHasPassword('{{env.base_url}}/pets')).toBe(false)
  })
})

describe('credentialsBlocked', () => {
  it('allows the boxes only when the server is a connection URL', () => {
    expect(credentialsBlocked('postgres://host/db')).toBe('')
    expect(credentialsBlocked('{{env.database_url}}')).toContain('{{env.database_url}}')
    expect(credentialsBlocked('shop.db')).toContain('not a connection URL')
  })
})

describe('moveTemplatesToValues', () => {
  it('Given a template in a PostgreSQL query Then it becomes the next $n and a value', () => {
    expect(moveTemplatesToValues('SELECT * FROM t WHERE id = {{id}} AND n = $1', ['x'], 'postgres')).toEqual({ query: 'SELECT * FROM t WHERE id = $2 AND n = $1', params: ['x', '{{id}}'] })
  })
  it('Given templates in a ? driver Then each value goes to its position among the placeholders', () => {
    const got = moveTemplatesToValues('SELECT * FROM t WHERE a = ? AND b = {{b}} AND c = ?', ['A', 'C'], 'mysql')
    expect(got).toEqual({ query: 'SELECT * FROM t WHERE a = ? AND b = ? AND c = ?', params: ['A', '{{b}}', 'C'] })
  })
  it('Given a template inside a quoted string Then it is left as it is', () => {
    expect(moveTemplatesToValues("SELECT '{{x}}'", [], 'postgres')).toBeNull()
  })
  it('Given no template Then there is nothing to move', () => {
    expect(moveTemplatesToValues('SELECT 1', [], 'postgres')).toBeNull()
  })
})

describe('placeholderCount', () => {
  it('counts the highest numbered placeholder, however often one is used', () => {
    // Before: adding a $3 to the query said nothing about the Values tab.
    expect(placeholderCount('select * from t where a = $1 and b = $2 and c = $1', 'postgres')).toBe(2)
    expect(placeholderCount('select $3', 'postgres')).toBe(3)
  })
  it('counts question marks for the drivers that use them, but not for PostgreSQL', () => {
    expect(placeholderCount('select * from t where a = ? and b = ?', 'mysql')).toBe(2)
    expect(placeholderCount("select data ? 'k' from t", 'postgres')).toBe(0)
    expect(placeholderCount('select ?, $2', 'sqlite')).toBe(2)
  })
  it('skips quoted text, comments and templates', () => {
    expect(placeholderCount("select '?', \"$9\" from t -- ? $4\nwhere a = ? /* $7 */", 'mysql')).toBe(1)
    expect(placeholderCount('select {{x}}, ?', 'mysql')).toBe(1)
  })
})

describe('valuesNote', () => {
  it('is null when the counts agree', () => {
    expect(valuesNote('select ?', 'mysql', 1)).toBeNull()
  })
  it('says how many values are missing', () => {
    expect(valuesNote('select ?, ?, ?', 'mysql', 2)).toEqual({ text: 'The query has 3 placeholders and 2 values.', missing: 1 })
  })
  it('warns about extra values without offering to add any', () => {
    const n = valuesNote('select ?', 'mysql', 3)
    expect(n?.missing).toBe(0)
    expect(n?.text).toContain('The query has 1 placeholder and 3 values.')
  })
})
