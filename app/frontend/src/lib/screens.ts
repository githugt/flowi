// The screens that open as tabs: the sidebar's tool rail, the window's menu and the tab strip all
// name and draw them from this list.

export interface Screen {
  /** The tab's node: screen:<name>. */
  id: string
  title: string
  /** An Icon name. */
  icon: string
  /** The window menu's action that opens it. */
  menu: string
  /** Shown in the sidebar's tool rail. */
  rail: boolean
}

export const screens: Screen[] = [
  { id: 'screen:import', title: 'Import', icon: 'import', menu: 'import', rail: true },
  { id: 'screen:manager', title: 'Collections', icon: 'folder', menu: 'collections', rail: true },
  { id: 'screen:secrets', title: 'Secrets', icon: 'key', menu: 'secrets', rail: true },
  { id: 'screen:schedules', title: 'Schedules', icon: 'clock', menu: 'schedules', rail: true },
  { id: 'screen:replayer', title: 'Replayer', icon: 'replay', menu: 'replayer', rail: true },
  { id: 'screen:checks', title: 'Checks', icon: 'checks', menu: 'checks', rail: true },
  { id: 'screen:blueprints', title: 'Blueprints', icon: 'blueprint', menu: 'blueprints', rail: true },
  { id: 'screen:trash', title: 'Trash', icon: 'trash', menu: 'trash', rail: true },
  { id: 'screen:settings', title: 'Settings', icon: 'settings', menu: 'settings', rail: true },
  { id: 'screen:welcome', title: 'Welcome', icon: 'home', menu: 'welcome', rail: false },
  { id: 'screen:telemetry', title: 'Telemetry', icon: 'chart', menu: 'telemetry', rail: false },
]

/** The screen a tab shows, if it is one. */
export const screenOf = (node: string): Screen | undefined => screens.find((s) => s.id === node)
