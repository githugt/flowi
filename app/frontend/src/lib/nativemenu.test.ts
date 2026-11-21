// @vitest-environment happy-dom
import { describe, expect, it } from 'vitest'
import { keepsNativeMenu } from './nativemenu'

describe('keepsNativeMenu', () => {
  const el = (html: string) => {
    document.body.innerHTML = html
    return document.body.firstElementChild!.querySelector('span') ?? document.body.firstElementChild!
  }

  it('is kept in a field, so Cut, Copy and Paste work', () => {
    expect(keepsNativeMenu(el('<input><span></span>'), '')).toBe(true)
    expect(keepsNativeMenu(el('<textarea></textarea>'), '')).toBe(true)
    expect(keepsNativeMenu(el('<div contenteditable="true"><span>x</span></div>'), '')).toBe(true)
  })

  it('is kept on selected text, so it can be copied', () => {
    expect(keepsNativeMenu(el('<p>hello</p>'), 'hello')).toBe(true)
  })

  it('is dropped anywhere else, such as the flow canvas', () => {
    expect(keepsNativeMenu(el('<svg><g></g></svg>'), '')).toBe(false)
    expect(keepsNativeMenu(el('<div contenteditable="false"><span>x</span></div>'), '')).toBe(false)
    expect(keepsNativeMenu(null, '  ')).toBe(false)
  })
})
