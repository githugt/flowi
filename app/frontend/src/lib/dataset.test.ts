import { describe, expect, it } from 'vitest'
import { moveServerPassword, serverCredentials, serverHasPassword, withServerCredentials } from './dataset'

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
