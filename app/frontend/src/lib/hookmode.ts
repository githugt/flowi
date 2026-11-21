// The hook mode of the next send, as a badge beside Send.
// Before: the mode lived only in the Send chevron's menu, so with hooks switched off the button looked the same and a
// person wondered why the setup request did not run.

export interface HookBadge {
  text: string
  title: string
}

/** The badge for a mode, or null for the usual one (all hooks run). */
export function hookBadge(mode: string): HookBadge | null {
  switch (mode) {
    case 'none':
      return { text: 'No hooks', title: 'The hooks before and after are skipped for the next send. Click to run them again.' }
    case 'skip-pre':
      return { text: 'No pre-hooks', title: 'The hooks before the request are skipped for the next send. Click to run them again.' }
    case 'skip-post':
      return { text: 'No post-hooks', title: 'The hooks after the response are skipped for the next send. Click to run them again.' }
    default:
      return null
  }
}
