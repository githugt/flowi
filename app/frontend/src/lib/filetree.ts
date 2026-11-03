export interface PlannedFile {
  rel: string
  exists: boolean
  remove: boolean
}

export interface FileNode {
  name: string
  path: string
  /** A folder holds children; a file does not. */
  folder: boolean
  /** Files at or below this node (1 for a file). */
  files: number
  /** Of those, the ones that already exist and would be replaced. */
  existing: number
  /** Requests and flows at or below this node, told by the requests or flows folder a file sits in. */
  requests: number
  flows: number
  remove: boolean
  children: FileNode[]
}

const byFoldersThenName = (a: FileNode, b: FileNode): number => Number(b.folder) - Number(a.folder) || a.name.localeCompare(b.name)

function sortDeep(nodes: FileNode[]): FileNode[] {
  nodes.sort(byFoldersThenName)
  nodes.forEach((n) => sortDeep(n.children))
  return nodes
}

function folderAt(level: FileNode[], name: string, path: string): FileNode {
  let node = level.find((n) => n.folder && n.name === name)
  if (!node) {
    node = { name, path, folder: true, files: 0, existing: 0, requests: 0, flows: 0, remove: false, children: [] }
    level.push(node)
  }
  return node
}

/** The files an import would write as a tree of folders, each folder counting what lies below it. */
export function buildFileTree(files: PlannedFile[]): FileNode[] {
  const roots: FileNode[] = []
  for (const f of files) {
    const parts = f.rel.split('/').filter(Boolean)
    const trail: FileNode[] = []
    let level = roots
    parts.slice(0, -1).forEach((part, i) => {
      const dir = folderAt(level, part, parts.slice(0, i + 1).join('/'))
      trail.push(dir)
      level = dir.children
    })
    const isYaml = /\.ya?ml$/.test(f.rel)
    const request = isYaml && parts.slice(0, -1).includes('requests') ? 1 : 0
    const flow = isYaml && parts.slice(0, -1).includes('flows') ? 1 : 0
    level.push({ name: parts[parts.length - 1], path: parts.join('/'), folder: false, files: 1, existing: f.exists ? 1 : 0, requests: request, flows: flow, remove: f.remove, children: [] })
    for (const dir of trail) {
      dir.files += 1
      dir.existing += f.exists ? 1 : 0
      dir.requests += request
      dir.flows += flow
    }
  }
  return sortDeep(roots)
}
