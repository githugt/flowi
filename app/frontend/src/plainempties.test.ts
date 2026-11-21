import { describe, expect, it } from 'vitest'

const files = import.meta.glob('./{Footer,RequestExtras,MessageFeed,StepsPanel,TrafficLog,DataTab,OutputDock,ChecksScreen}.svelte', { query: '?raw', import: 'default', eager: true }) as Record<string, string>

describe('plain empty lines that became EmptyState', () => {
  it('has no "nothing here" sentence left as a muted line in the panes that had one', () => {
    // Before: these panes said "No problems", "The jar is empty", "No calls yet" and the like as one muted line, with no
    // icon and a different size in each place.
    expect(Object.keys(files)).toHaveLength(8)
    const lines: Record<string, RegExp> = {
      './Footer.svelte': /<p class="muted">No problems\.<\/p>/,
      './RequestExtras.svelte': /<p class="muted">(Nothing uses this request yet|The jar is empty)\.<\/p>/,
      './MessageFeed.svelte': /<p class="muted none">Nothing yet\.<\/p>/,
      './StepsPanel.svelte': /<p class="muted">This flow has no steps yet\.<\/p>/,
      './TrafficLog.svelte': /<p class="none">No calls yet\.<\/p>/,
      './DataTab.svelte': /has not run yet\.<\/p>|Select a step to see what the last run did there\.<\/p>/,
      './OutputDock.svelte': /has nothing to show\.<\/p>|has not run yet\.`}<\/p>/,
      './ChecksScreen.svelte': /<p class="muted">No operation to measure/,
    }
    for (const [path, text] of Object.entries(files)) {
      expect(text, path).toContain('<EmptyState')
      expect(text, path).not.toMatch(lines[path])
    }
  })
})
