// The Help menu's links: the documentation and the place to report a problem.
// Before: Help held a welcome page, usage reporting and About, with no way to the documentation or to a bug report.
const REPO = 'https://github.com/githugt/flowi'

const LINKS: Record<string, string> = {
  docs: `${REPO}#readme`,
  report: `${REPO}/issues/new`,
}

/** The web address a Help action opens, or undefined for an action that is not a link. */
export const helpUrl = (action: string): string | undefined => LINKS[action]
