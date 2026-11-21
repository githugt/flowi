// What a screen reader hears when a send finishes. The response pane draws the result silently, so someone who
// cannot see it would not know the send ended, or how.
// Before: nothing announced a finished send; only toasts were in a live region.
export function sendAnnouncement(sending: boolean, response: { title: string; took: string; passed: boolean } | undefined, error: string): string {
  if (sending) return 'Sending'
  if (error && !response) return `Send failed. ${error}`
  if (!response) return ''
  const took = response.took ? `, ${response.took}` : ''
  return `${response.title}${took}${response.passed ? '' : '. Checks failed'}`
}
