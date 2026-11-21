// The replayer form as the person last left it, shared by the Replayer screen and the replay bar.
// Before: the bar started the server with the form Go had read at start-up, so faults, forwarding and the other fields
// set on the screen were lost, and a tab switch dropped what was typed.
import type { ReplayerView } from './types'

type Form = ReplayerView['form']

/** The draft, or null before the screen has shown a form. */
export const replayerDraft = $state<{ form: Form | null }>({ form: null })
