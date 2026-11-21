export interface WelcomeInput {
  collections: string[]
  selectedEnv: string
  environments: number
  firstRequest?: string
  firstFlow?: string
}

export interface WelcomeStep {
  done: boolean
  title: string
  detail: string
}

/** The model the product is built on, said once on the welcome screen. */
export const MODEL_LINE = 'Request (one call) → Flow (calls in order, passing values along) → Check (fails the run when something is off)'

/** The first-run checklist. The last step adds a check: before, the list never mentioned checks at all. */
export function welcomeSteps(i: WelcomeInput): WelcomeStep[] {
  const example = i.firstRequest ? `For example ${i.firstRequest}${i.firstFlow ? ` or the ${i.firstFlow} flow` : ''}.` : 'Once a collection is open.'
  return [
    {
      done: i.collections.length > 0,
      title: 'Open a collection',
      // Before: "Mount a folder of requests and flows" never said what a collection is.
      detail: i.collections.length ? i.collections.join(', ') : 'A collection is a folder of YAML files, ideally in your git repo. Open one, or create a new one.',
    },
    { done: !!i.selectedEnv, title: 'Choose an environment', detail: i.selectedEnv ? `Using ${i.selectedEnv}.` : i.environments ? 'Requests read {{env.…}} values from it.' : 'Create one for values like base_url.' },
    { done: false, title: 'Send a request or run a flow', detail: example },
    { done: false, title: 'Add a check', detail: 'Open a request, go to the Assert tab, add response.status == 200, and send again. A failed check fails the run.' },
  ]
}
