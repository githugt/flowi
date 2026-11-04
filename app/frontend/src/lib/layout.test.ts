import { describe, expect, it } from 'vitest'
import { closeNode, closePane, decodeLayout, edgeOf, encodeLayout, leaf, nodesOf, panesOf, resizeSplit, sharesOf, splitForDrop, splitPane, withNode, type Layout, type Split } from './layout'

const shape = (l: Layout): unknown => (l.kind === 'pane' ? l.node : { [l.dir]: l.children.map(shape) })
const rect = { left: 0, right: 100, top: 0, bottom: 100 }

describe('splitPane', () => {
  it('puts the new pane on the side of the one it splits', () => {
    expect(shape(splitPane(leaf('a', 'A'), 'a', 'B', 'right', 'b'))).toEqual({ row: ['A', 'B'] })
    expect(shape(splitPane(leaf('a', 'A'), 'a', 'B', 'left', 'b'))).toEqual({ row: ['B', 'A'] })
    expect(shape(splitPane(leaf('a', 'A'), 'a', 'B', 'bottom', 'b'))).toEqual({ col: ['A', 'B'] })
    expect(shape(splitPane(leaf('a', 'A'), 'a', 'B', 'top', 'b'))).toEqual({ col: ['B', 'A'] })
  })

  it('adds a third pane beside the two instead of nesting a row in a row', () => {
    const two = splitPane(leaf('a', 'A'), 'a', 'B', 'right', 'b')

    expect(shape(splitPane(two, 'b', 'C', 'right', 'c'))).toEqual({ row: ['A', 'B', 'C'] })
    expect(shape(splitPane(two, 'a', 'C', 'right', 'c'))).toEqual({ row: ['A', 'C', 'B'] })
  })

  it('nests a column in a row when a pane of the row is split the other way', () => {
    const two = splitPane(leaf('a', 'A'), 'a', 'B', 'right', 'b')

    expect(shape(splitPane(two, 'b', 'C', 'bottom', 'c'))).toEqual({ row: ['A', { col: ['B', 'C'] }] })
  })

  it('changes nothing for a pane that is not there', () => {
    const one = leaf('a', 'A')

    expect(splitPane(one, 'zzz', 'B', 'right', 'b')).toEqual(one)
  })

  it('leaves the layout it was given as it was', () => {
    const one = leaf('a', 'A')

    splitPane(one, 'a', 'B', 'right', 'b')

    expect(shape(one)).toBe('A')
  })
})

describe('closePane', () => {
  const three = () => splitPane(splitPane(leaf('a', 'A'), 'a', 'B', 'right', 'b'), 'b', 'C', 'bottom', 'c')

  it('takes the pane away and lets the one beside it have the room', () => {
    expect(shape(closePane(three(), 'c'))).toEqual({ row: ['A', 'B'] })
    expect(shape(closePane(three(), 'a'))).toEqual({ col: ['B', 'C'] })
  })

  it('does not close the last pane', () => {
    expect(shape(closePane(leaf('a', 'A'), 'a'))).toBe('A')
  })

  it('folds a column that is left with one pane into the row around it', () => {
    const l = splitPane(splitPane(leaf('a', 'A'), 'a', 'B', 'right', 'b'), 'b', 'C', 'bottom', 'c')

    expect(shape(closePane(closePane(l, 'c'), 'b'))).toBe('A')
  })
})

describe('closeNode', () => {
  it('closes every pane that showed the tab, and keeps the others', () => {
    const l = splitPane(splitPane(leaf('a', 'A'), 'a', 'B', 'right', 'b'), 'b', 'A', 'right', 'c')

    expect(shape(closeNode(l, 'A', 'X'))).toBe('B')
  })

  it('shows the fallback when the only pane lost its tab', () => {
    expect(shape(closeNode(leaf('a', 'A'), 'A', 'X'))).toBe('X')
  })
})

describe('withNode, panesOf, nodesOf', () => {
  it('shows another tab in a pane', () => {
    const l = splitPane(leaf('a', 'A'), 'a', 'B', 'right', 'b')

    expect(shape(withNode(l, 'b', 'C'))).toEqual({ row: ['A', 'C'] })
    expect(panesOf(l).map((p) => p.id)).toEqual(['a', 'b'])
    expect(nodesOf(l)).toEqual(['A', 'B'])
  })
})

describe('edgeOf', () => {
  it('names the side of a pane a point is near', () => {
    expect(edgeOf(10, 50, rect)).toBe('left')
    expect(edgeOf(90, 50, rect)).toBe('right')
    expect(edgeOf(50, 10, rect)).toBe('top')
    expect(edgeOf(50, 90, rect)).toBe('bottom')
  })

  it('names none in the middle, so a drop there does not split', () => {
    expect(edgeOf(50, 50, rect)).toBeNull()
  })

  it('names none outside the pane', () => {
    expect(edgeOf(-1, 50, rect)).toBeNull()
    expect(edgeOf(50, 101, rect)).toBeNull()
  })

  it('takes the nearer edge in a corner', () => {
    expect(edgeOf(5, 20, rect)).toBe('left')
    expect(edgeOf(20, 5, rect)).toBe('top')
  })
})

describe('encodeLayout and decodeLayout', () => {
  const ids = () => {
    let n = 0
    return () => `p${++n}`
  }
  const sample = () => splitPane(splitPane(leaf('a', 'A'), 'a', 'B', 'right', 'b'), 'b', 'C', 'bottom', 'c')

  it('gives back the arrangement of the tabs that are still open', () => {
    const got = decodeLayout(JSON.parse(JSON.stringify(encodeLayout(sample()))), () => true, ids())

    expect(got && shape(got)).toEqual({ row: ['A', { col: ['B', 'C'] }] })
  })

  it('leaves out the tabs that are gone, and the room they had goes to the rest', () => {
    const got = decodeLayout(encodeLayout(sample()), (n) => n !== 'B', ids())

    expect(got && shape(got)).toEqual({ row: ['A', 'C'] })
  })

  it('is null when no tab of it is open', () => {
    expect(decodeLayout(encodeLayout(sample()), () => false, ids())).toBeNull()
  })

  it('is null for anything that is not a layout, whatever storage held', () => {
    for (const junk of [null, 5, 'x', [], {}, { dir: 'row' }, { dir: 'diag', children: [] }, { node: 7 }, { dir: 'row', children: 'no' }]) {
      expect(decodeLayout(junk, () => true, ids())).toBeNull()
    }
  })

  it('gives every pane an id of its own', () => {
    const got = decodeLayout(encodeLayout(sample()), () => true, ids())

    expect(new Set(got && panesOf(got).map((p) => p.id)).size).toBe(3)
  })
})

describe('splitForDrop', () => {
  const nextId = 'new'

  it('puts the dropped tab on the edge of the pane, which keeps its own tab', () => {
    const got = splitForDrop(leaf('a', 'A'), ['A', 'B'], 'a', 'B', 'right', nextId)

    expect(got && shape(got.layout)).toEqual({ row: ['A', 'B'] })
    expect(got?.pane).toBe(nextId)
  })

  it('moves a tab that another pane shows, instead of showing it twice', () => {
    const two = splitPane(leaf('a', 'A'), 'a', 'B', 'right', 'b')

    const got = splitForDrop(two, ['A', 'B'], 'a', 'B', 'left', nextId)

    expect(got && shape(got.layout)).toEqual({ row: ['B', 'A'] })
  })

  it('brings the next tab of the strip along when a tab is dropped on the pane that shows it', () => {
    const got = splitForDrop(leaf('a', 'A'), ['A', 'B', 'C'], 'a', 'A', 'right', nextId)

    expect(got && shape(got.layout)).toEqual({ row: ['B', 'A'] })
  })

  it('takes the tab before it when it is the last in the strip', () => {
    const got = splitForDrop(leaf('a', 'C'), ['A', 'B', 'C'], 'a', 'C', 'left', nextId)

    expect(got && shape(got.layout)).toEqual({ row: ['C', 'B'] })
  })

  it('does nothing when there is no other tab to show', () => {
    expect(splitForDrop(leaf('a', 'A'), ['A'], 'a', 'A', 'right', nextId)).toBeNull()
  })

  it('does nothing for a tab that is not open or a pane that is not there', () => {
    expect(splitForDrop(leaf('a', 'A'), ['A', 'B'], 'a', 'Z', 'right', nextId)).toBeNull()
    expect(splitForDrop(leaf('a', 'A'), ['A', 'B'], 'zzz', 'B', 'right', nextId)).toBeNull()
  })

  it('splits a pane of three the way the edge says', () => {
    const three = splitPane(splitPane(leaf('a', 'A'), 'a', 'B', 'right', 'b'), 'b', 'C', 'right', 'c')

    const got = splitForDrop(three, ['A', 'B', 'C', 'D'], 'b', 'D', 'bottom', nextId)

    expect(got && shape(got.layout)).toEqual({ row: ['A', { col: ['B', 'D'] }, 'C'] })
  })
})

describe('resizeSplit', () => {
  const row = (): Split => splitPane(splitPane(leaf('a', 'A'), 'a', 'B', 'right', 'b'), 'b', 'C', 'right', 'c') as Split
  const shares = (l: Layout) => sharesOf(l as Split).map((w) => Math.round(w * 1000) / 1000)

  it('gives equal shares until a divider is moved', () => {
    expect(shares(row())).toEqual([0.333, 0.333, 0.333])
  })

  it('moves the divider to where the pointer is, and the pane beyond it keeps what the other two do not use', () => {
    const got = resizeSplit(row(), [], 0, 0.2)

    expect(shares(got)).toEqual([0.2, 0.467, 0.333])
  })

  it('measures the second divider from the start of the split, not of the pane before it', () => {
    const got = resizeSplit(row(), [], 1, 0.8)

    expect(shares(got)).toEqual([0.333, 0.467, 0.2])
  })

  it('stops a pane at its least share instead of squeezing it away', () => {
    expect(shares(resizeSplit(row(), [], 0, 0))).toEqual([0.1, 0.567, 0.333])
    expect(shares(resizeSplit(row(), [], 0, 5))).toEqual([0.567, 0.1, 0.333])
  })

  it('moves a divider of a split inside another by its path', () => {
    const nested = splitPane(row(), 'c', 'D', 'bottom', 'd')

    const got = resizeSplit(nested, [2], 0, 0.25) as Split

    expect(shares(got)).toEqual([0.333, 0.333, 0.333])
    expect(shares(got.children[2])).toEqual([0.25, 0.75])
  })

  it('changes nothing for a path, a divider or a position that is not there', () => {
    const l = row()

    expect(resizeSplit(l, [5], 0, 0.5)).toBe(l)
    expect(resizeSplit(l, [], 2, 0.5)).toEqual(l)
    expect(resizeSplit(l, [], -1, 0.5)).toEqual(l)
    expect(resizeSplit(l, [], 0, Number.NaN)).toEqual(l)
    expect(resizeSplit(leaf('a', 'A'), [], 0, 0.5)).toEqual(leaf('a', 'A'))
  })
})

describe('shares across other changes', () => {
  const sized = () => resizeSplit(splitPane(splitPane(leaf('a', 'A'), 'a', 'B', 'right', 'b'), 'b', 'C', 'right', 'c'), [], 0, 0.5) as Split
  const shares = (l: Layout) => sharesOf(l as Split).map((w) => Math.round(w * 1000) / 1000)

  it('gives a pane that is split in beside the others an equal share, taken in proportion from each', () => {
    const got = splitPane(sized(), 'c', 'D', 'right', 'd')

    expect(shares(got)).toEqual([0.375, 0.125, 0.25, 0.25])
  })

  it('shares what a closed pane had among the others in proportion to theirs', () => {
    const got = closePane(sized(), 'c')

    expect(shares(got)).toEqual([0.75, 0.25])
  })

  it('keeps the shares of a split through a pane showing another tab', () => {
    expect(shares(withNode(sized(), 'a', 'Z'))).toEqual(shares(sized()))
  })

  it('stores no shares for a split that was never resized, and keeps them for one that was', () => {
    const plain = splitPane(leaf('a', 'A'), 'a', 'B', 'right', 'b')

    expect(encodeLayout(plain)).toEqual({ dir: 'row', children: [{ node: 'A' }, { node: 'B' }] })
    expect((encodeLayout(sized()) as { sizes: number[] }).sizes).toHaveLength(3)
  })

  it('reads the shares back, and shares out those of a pane whose tab is gone', () => {
    const data = encodeLayout(sized())
    let n = 0
    const all = decodeLayout(data, () => true, () => `p${n++}`) as Split
    const without = decodeLayout(data, (node) => node !== 'C', () => `q${n++}`) as Split

    expect(shares(all)).toEqual(shares(sized()))
    expect(shares(without)).toEqual([0.75, 0.25])
  })

  it('ignores shares that are not one positive number for each pane', () => {
    const bad = { dir: 'row', sizes: [1, -1], children: [{ node: 'A' }, { node: 'B' }] }
    const short = { dir: 'row', sizes: [1], children: [{ node: 'A' }, { node: 'B' }] }

    expect(shares(decodeLayout(bad, () => true, () => 'x') as Split)).toEqual([0.5, 0.5])
    expect(shares(decodeLayout(short, () => true, () => 'x') as Split)).toEqual([0.5, 0.5])
  })

  it('keeps the room of a split that joins its parent: a row beside a pane, closed to one pane, gives its panes back their share', () => {
    const inner = splitPane(leaf('a', 'A'), 'a', 'B', 'right', 'b')
    const outer = resizeSplit(splitPane(inner, 'b', 'C', 'bottom', 'c'), [], 0, 0.5)

    const got = closePane(outer, 'c')

    expect(shape(got)).toEqual({ row: ['A', 'B'] })
  })
})
