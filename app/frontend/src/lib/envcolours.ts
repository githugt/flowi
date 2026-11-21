// The colours an environment can be marked with (model.EnvColours), so that one is not taken for another.

const swatches: Record<string, string> = {
  red: '#e5534b',
  orange: '#f0883e',
  yellow: '#d4a72c',
  lime: '#84cc16',
  green: '#3fb950',
  aqua: '#2dd4bf',
  blue: '#4f8cf0',
  purple: '#a371f7',
  gray: '#8b949e',
}

/** The CSS colour of a named environment colour; undefined for the default or a name the window cannot draw. */
export function envSwatch(colour: string | undefined): string | undefined {
  return colour ? swatches[colour] : undefined
}

/**
 * The colour of the environment in use, for the band under the tab strip and the line over the footer; undefined for
 * no environment or the default colour. Production safety rested on one small chip at the far right of the tabs.
 */
export function bandColour(tree: { selected?: string; envColours?: Record<string, string> } | null | undefined): string | undefined {
  return envSwatch(tree?.envColours?.[tree?.selected ?? ''])
}
