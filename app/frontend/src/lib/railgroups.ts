export type RailGroup = 'collections' | 'environments' | 'flows'
// What a rail icon shows of the sidebar is chosen by sectionsFor (lib/sidebarview): the Requests section for the
// collections icon, Flows, or Environments. Before: it cut the tree itself, which no longer has a group row to cut by.
