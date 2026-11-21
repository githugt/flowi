// @vitest-environment happy-dom
import { cleanup, fireEvent, render, screen } from '@testing-library/svelte'
import { afterEach, describe, expect, it, vi } from 'vitest'
import Button from './Button.svelte'
import Checkbox from './Checkbox.svelte'
import EmptyState from './EmptyState.svelte'
import Menu from './Menu.svelte'
import Modal from './Modal.svelte'
import RadioGroup from './RadioGroup.svelte'
import Select from './Select.svelte'
import Tabs from './Tabs.svelte'
import Tip from './Tip.svelte'
import { text } from './lib/domtest'

// The first tests that run a component. The rest of the suite reads source text, which cannot tell that a key does
// nothing or that a click is lost; these mount the shared pieces and press them.
// A DOM has no layout, so what a person sees (a tab label at the top of its bar, a bar that stacks) is still the job of
// stylelost.test.ts and of looking at the screen.
afterEach(cleanup)

// The Wails runtime calls out to the app's server when it loads. There is none in a test, and the call only produces noise.
vi.mock('@wailsio/runtime', () => ({ Browser: { OpenURL: vi.fn() } }))

describe('Tabs', () => {
  const items = [{ id: 'a', label: 'Alpha' }, { id: 'b', label: 'Beta', count: 3 }, { id: 'c', label: 'Gamma', disabled: true }, { id: 'd', label: 'Delta' }]

  it('marks the tab in front and gives only it a tab stop', () => {
    render(Tabs, { items, active: 'b', onpick: () => {}, label: 'Sections' })

    expect(screen.getByRole('tablist', { name: 'Sections' })).toBeTruthy()
    expect(screen.getByRole('tab', { name: /Beta/ }).getAttribute('aria-selected')).toBe('true')
    expect(screen.getByRole('tab', { name: /Beta/ }).getAttribute('tabindex')).toBe('0')
    expect(screen.getByRole('tab', { name: 'Alpha' }).getAttribute('tabindex')).toBe('-1')
    expect(screen.getByText('3')).toBeTruthy()
  })

  it('moves with the arrow keys, skipping a disabled tab', async () => {
    const onpick = vi.fn()
    render(Tabs, { items, active: 'b', onpick })

    await fireEvent.keyDown(screen.getByRole('tab', { name: /Beta/ }), { key: 'ArrowRight' })

    expect(onpick).toHaveBeenLastCalledWith('d')
  })

  it('jumps to the ends with Home and End', async () => {
    const onpick = vi.fn()
    render(Tabs, { items, active: 'b', onpick })

    await fireEvent.keyDown(screen.getByRole('tab', { name: /Beta/ }), { key: 'End' })
    expect(onpick).toHaveBeenLastCalledWith('d')
    await fireEvent.keyDown(screen.getByRole('tab', { name: /Beta/ }), { key: 'Home' })

    expect(onpick).toHaveBeenLastCalledWith('a')
  })

  it('reports a click and ignores a disabled tab', async () => {
    const onpick = vi.fn()
    render(Tabs, { items, active: 'a', onpick })

    await fireEvent.click(screen.getByRole('tab', { name: 'Gamma' }))
    await fireEvent.click(screen.getByRole('tab', { name: 'Delta' }))

    expect(onpick).toHaveBeenCalledTimes(1)
    expect(onpick).toHaveBeenCalledWith('d')
  })

  it('has no tab stop on a tab when none is in front but still offers the first that can be chosen', () => {
    render(Tabs, { items, active: null, onpick: () => {} })

    expect(screen.getByRole('tab', { name: 'Alpha' }).getAttribute('tabindex')).toBe('0')
    expect(screen.getByRole('tab', { name: 'Delta' }).getAttribute('tabindex')).toBe('-1')
  })
})

describe('Modal', () => {
  it('is a named dialog with its heading and buttons', () => {
    render(Modal, { title: 'Rename', onclose: () => {}, children: text('body'), actions: text('buttons') })

    expect(screen.getByRole('dialog', { name: 'Rename' })).toBeTruthy()
    expect(screen.getByRole('heading', { name: 'Rename' })).toBeTruthy()
    expect(screen.getByText('buttons')).toBeTruthy()
  })

  it('closes on Escape when it can be closed', async () => {
    const onclose = vi.fn()
    render(Modal, { title: 'Rename', onclose, children: text('body') })

    await fireEvent.keyDown(screen.getByRole('dialog'), { key: 'Escape' })

    expect(onclose).toHaveBeenCalledTimes(1)
  })

  it('stays open on Escape when it has to be answered with a button', async () => {
    render(Modal, { title: 'Sure?', children: text('body') })

    await fireEvent.keyDown(screen.getByRole('dialog'), { key: 'Escape' })

    expect(screen.getByRole('dialog')).toBeTruthy()
  })

  it('is an alert dialog when it says so, and a form when it is submitted', async () => {
    const onsubmit = vi.fn()
    render(Modal, { title: 'Delete', alert: true, onsubmit, children: text('body'), actions: text('x') })

    const box = screen.getByRole('alertdialog')
    await fireEvent.submit(box)

    expect(box.tagName).toBe('FORM')
    expect(onsubmit).toHaveBeenCalledTimes(1)
  })
})

// The menu stays invisible until it has measured itself and a DOM has no layout, and a hidden element has no accessible
// name, so its items are found by their words.
describe('Menu', () => {
  const pick = vi.fn()
  const items = [{ label: 'Open', onselect: () => pick('open') }, { label: 'Gone', onselect: () => pick('gone'), disabled: true }, { label: 'Close', onselect: () => pick('close') }]
  const row = (name: string) => screen.getByText(name).closest('button') as HTMLButtonElement
  const menu = () => screen.getByRole('menu', { hidden: true })

  it('lists its items as menu items, with a disabled one disabled', () => {
    render(Menu, { items, at: { x: 10, y: 10 }, onclose: () => {}, label: 'Actions' })

    expect(menu().getAttribute('aria-label')).toBe('Actions')
    expect(screen.getAllByRole('menuitem', { hidden: true })).toHaveLength(3)
    expect(row('Gone').disabled).toBe(true)
  })

  it('puts focus on the first item that can be chosen and moves with the arrow keys over the disabled one', async () => {
    render(Menu, { items, at: { x: 10, y: 10 }, onclose: () => {} })

    expect(document.activeElement).toBe(row('Open'))
    await fireEvent.keyDown(menu(), { key: 'ArrowDown' })

    expect(document.activeElement).toBe(row('Close'))
  })

  it('closes first and then runs the item chosen', async () => {
    const order: string[] = []
    const onclose = () => order.push('close')
    render(Menu, { items: [{ label: 'Go', onselect: () => order.push('select') }], at: { x: 0, y: 0 }, onclose })

    await fireEvent.click(row('Go'))

    expect(order).toEqual(['close', 'select'])
  })

  it('stays open after an item that is switched on and off', async () => {
    const onclose = vi.fn()
    const onselect = vi.fn()
    render(Menu, { items: [{ label: 'Wrap', checked: false, multi: true, onselect }], at: { x: 0, y: 0 }, onclose })

    await fireEvent.click(row('Wrap'))

    expect(row('Wrap').getAttribute('role')).toBe('menuitemcheckbox')
    expect(onselect).toHaveBeenCalledTimes(1)
    expect(onclose).not.toHaveBeenCalled()
  })

  it('closes on Escape and on a press outside', async () => {
    const onclose = vi.fn()
    const { container } = render(Menu, { items, at: { x: 0, y: 0 }, onclose })

    await fireEvent.keyDown(menu(), { key: 'Escape' })
    await fireEvent.pointerDown(container.querySelector('.layer') as Element)

    expect(onclose).toHaveBeenCalledTimes(2)
  })
})

describe('EmptyState', () => {
  it('names what is missing and offers the way out', () => {
    render(EmptyState, { title: 'No hooks', hint: 'Add one.', children: text('Add hooks…') })

    expect(screen.getByText('No hooks')).toBeTruthy()
    expect(screen.getByText('Add one.')).toBeTruthy()
    expect(screen.getByText('Add hooks…')).toBeTruthy()
  })

  it('says what a filter hides, announces it, and clears it', async () => {
    const onclear = vi.fn()
    render(EmptyState, { kind: 'filtered', query: 'abc', onclear })

    expect(screen.getByRole('status').textContent).toContain('Nothing matches “abc”')
    await fireEvent.click(screen.getByRole('button', { name: 'Clear filter' }))

    expect(onclear).toHaveBeenCalledTimes(1)
  })
})

describe('Button', () => {
  it('runs onclick, and not when it is disabled', async () => {
    const onclick = vi.fn()
    const { rerender } = render(Button, { onclick, children: text('Go') })

    await fireEvent.click(screen.getByRole('button', { name: 'Go' }))
    await rerender({ onclick, disabled: true, children: text('Go') })
    await fireEvent.click(screen.getByRole('button', { name: 'Go' }))

    expect(onclick).toHaveBeenCalledTimes(1)
  })
})

describe('Checkbox', () => {
  it('is named by its words, and clicking the words ticks the box', async () => {
    const onchange = vi.fn()
    render(Checkbox, { checked: false, onchange, children: text('Only secrets') })

    await fireEvent.click(screen.getByText('Only secrets'))

    expect((screen.getByRole('checkbox', { name: 'Only secrets' }) as HTMLInputElement).checked).toBe(true)
    expect(onchange).toHaveBeenCalledWith(true)
  })

  it('is named by its label when it has no words, as in a table cell', () => {
    render(Checkbox, { checked: true, label: 'Send X-Trace' })

    expect((screen.getByRole('checkbox', { name: 'Send X-Trace' }) as HTMLInputElement).checked).toBe(true)
  })

  it('reports each change, off as well as on', async () => {
    const onchange = vi.fn()
    render(Checkbox, { checked: true, label: 'x', onchange })

    await fireEvent.click(screen.getByRole('checkbox'))

    expect(onchange).toHaveBeenLastCalledWith(false)
  })

  it('does nothing when it is disabled', async () => {
    const onchange = vi.fn()
    render(Checkbox, { checked: false, label: 'x', disabled: true, onchange })

    await fireEvent.click(screen.getByRole('checkbox'))

    expect(onchange).not.toHaveBeenCalled()
  })
})

describe('Select', () => {
  const options = text('<option value="a">Alpha</option><option value="b">Beta</option>')
  // The options are plain HTML inside a snippet, so the select is read as the browser does.
  const box = () => screen.getByRole('combobox') as HTMLSelectElement

  it('is a native select named by its label, with its value chosen', () => {
    render(Select, { value: 'b', label: 'Letter', children: options })

    expect(screen.getByRole('combobox', { name: 'Letter' })).toBeTruthy()
    expect(box().tagName).toBe('SELECT')
  })

  it('reports the event, so the handler reads the new value from it', async () => {
    const onchange = vi.fn((e: Event & { currentTarget: HTMLSelectElement }) => e.currentTarget.value)
    render(Select, { value: 'a', label: 'x', onchange, children: options })

    await fireEvent.change(box(), { target: { value: 'b' } })

    expect(onchange).toHaveBeenCalledTimes(1)
    expect(onchange.mock.results[0].value).toBe('b')
  })

  it('does nothing when it is disabled', () => {
    render(Select, { value: 'a', label: 'x', disabled: true, children: options })

    expect(box().disabled).toBe(true)
  })

  it('takes its look from the size, tone and fill it is given', () => {
    render(Select, { label: 'x', size: 'sm', tone: 'raised', fill: true, children: options })

    expect(box().classList.contains('sm')).toBe(true)
    expect(box().classList.contains('raised')).toBe(true)
    expect(box().classList.contains('fill')).toBe(true)
  })
})

describe('RadioGroup', () => {
  const options = [{ value: 'a', label: 'Alpha', hint: 'First' }, { value: 'b', label: 'Beta' }]

  it('is a named group of radio buttons with the chosen one checked', () => {
    render(RadioGroup, { value: 'b', options, legend: 'Pick one' })

    expect(screen.getByRole('group', { name: 'Pick one' })).toBeTruthy()
    expect((screen.getByRole('radio', { name: 'Alpha' }) as HTMLInputElement).checked).toBe(false)
    expect((screen.getByRole('radio', { name: 'Beta' }) as HTMLInputElement).checked).toBe(true)
  })

  it('reports the value picked, and clicking the words picks it', async () => {
    const onchange = vi.fn()
    render(RadioGroup, { value: 'a', options, label: 'Pick', onchange })

    await fireEvent.click(screen.getByText('Beta'))

    expect(onchange).toHaveBeenCalledWith('b')
    expect((screen.getByRole('radio', { name: 'Beta' }) as HTMLInputElement).checked).toBe(true)
  })

  it('cannot be changed when it is disabled', () => {
    render(RadioGroup, { value: 'a', options, label: 'Pick', disabled: true })

    expect((screen.getByRole('radio', { name: 'Beta' }) as HTMLInputElement).disabled).toBe(true)
  })
})

describe('Tip', () => {
  it('shows its words while the pointer rests on it', async () => {
    const { container } = render(Tip, { text: 'What this does', children: text('?') })

    await fireEvent.mouseEnter(container.querySelector('.tip') as Element)
    expect(screen.getByRole('tooltip').textContent).toBe('What this does')
    await fireEvent.mouseLeave(container.querySelector('.tip') as Element)

    expect(screen.queryByRole('tooltip')).toBeNull()
  })

  // Before: the Escape only closed the tip when the focus was on its trigger; with the pointer resting on it (the usual
  // way a hint is read) the Escape went on to close the popover the hint sits in, and the tip stayed.
  it('takes the first Escape from anywhere, so the dialog around it stays open', async () => {
    const outer = vi.fn()
    window.addEventListener('keydown', outer)
    const { container } = render(Tip, { text: 'What this does', children: text('?') })
    await fireEvent.mouseEnter(container.querySelector('.tip') as Element)

    await fireEvent.keyDown(document.body, { key: 'Escape' })
    expect(screen.queryByRole('tooltip')).toBeNull()
    expect(outer).not.toHaveBeenCalled()

    await fireEvent.keyDown(document.body, { key: 'Escape' })
    expect(outer).toHaveBeenCalledTimes(1)
    window.removeEventListener('keydown', outer)
  })
})
