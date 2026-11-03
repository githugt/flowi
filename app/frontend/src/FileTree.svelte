<script lang="ts">
  import FileTree from './FileTree.svelte'
  import type { FileNode } from './lib/filetree'

  // Folders show open for the first two levels; deeper ones fold until asked for.
  let { nodes, depth = 0 }: { nodes: FileNode[]; depth?: number } = $props()
  const closed = $state<Record<string, boolean>>({})
  const isOpen = (n: FileNode) => closed[n.path] === undefined ? depth < 2 : !closed[n.path]
  const toggle = (n: FileNode) => (closed[n.path] = isOpen(n))
  const plural = (n: number, one: string, many: string) => `${n} ${n === 1 ? one : many}`
  // "2 requests · 1 flow" when the folder holds any, else the plain file count.
  const counts = (n: FileNode): string => {
    const parts = [n.requests && plural(n.requests, 'request', 'requests'), n.flows && plural(n.flows, 'flow', 'flows')].filter(Boolean)
    return parts.length ? parts.join(' · ') : plural(n.files, 'file', 'files')
  }
</script>

<ul class="tree" role={depth === 0 ? 'tree' : 'group'}>
  {#each nodes as n (n.path)}
    <li role="treeitem" aria-selected="false" aria-expanded={n.folder ? isOpen(n) : undefined}>
      {#if n.folder}
        <button class="dir" style:padding-left={`${depth * 14 + 4}px`} onclick={() => toggle(n)}>
          <span class="twist" class:open={isOpen(n)} aria-hidden="true">▸</span>
          <span class="name">{n.name}/</span>
          <span class="n">{counts(n)}</span>
          {#if n.existing}<span class="warn">{n.existing} exist</span>{/if}
        </button>
        {#if isOpen(n)}<FileTree nodes={n.children} depth={depth + 1} />{/if}
      {:else}
        <div class="file" class:exists={n.existing > 0} style:padding-left={`${depth * 14 + 22}px`}>
          <span class="sign" aria-hidden="true">{n.remove ? '−' : '+'}</span><span class="name">{n.name}</span>{#if n.existing}<span class="warn">exists</span>{/if}
        </div>
      {/if}
    </li>
  {/each}
</ul>

<style>
  .tree { list-style: none; margin: 0; padding: 0; font: var(--fs-sm) var(--mono); }
  .dir { all: unset; box-sizing: border-box; display: flex; align-items: center; gap: 6px; width: 100%; height: var(--row-h); border-radius: var(--r-sm); cursor: pointer; color: var(--text); }
  .dir:hover { background: var(--tint-hover); }
  .twist { color: var(--muted); display: inline-block; width: 12px; transition: transform var(--t-fast) var(--ease-out); }
  .twist.open { transform: rotate(90deg); }
  .n { color: var(--muted); font-size: var(--fs-xs); }
  .file { display: flex; align-items: center; gap: 6px; height: var(--row-h); color: var(--text); }
  .sign { width: 10px; color: var(--ok); }
  .file.exists .name { color: var(--warn); }
  .warn { color: var(--warn); font-size: var(--fs-xs); }
</style>
