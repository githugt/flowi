import type { SecretUseView, SecretsView } from './types'

type Secret = SecretsView['secrets'][number]

/** The secrets to list: all of them, or only the ones with no value (the ones a run would stop at). */
export const visibleSecrets = (secrets: Secret[], onlyMissing: boolean): Secret[] => (onlyMissing ? secrets.filter((s) => s.state === 'missing') : secrets)

/** "Used by 3: 2 requests, 1 flow" for the line under a secret; "Not used by any file" when nothing writes it. */
export function usedByText(uses: SecretUseView[] | undefined): string {
  const list = uses ?? []
  if (!list.length) return 'Not used by any file'
  const count = (kind: string, one: string, many: string) => {
    const n = list.filter((u) => u.kind === kind).length
    return n ? [`${n} ${n === 1 ? one : many}`] : []
  }
  const parts = [...count('request', 'request', 'requests'), ...count('flow', 'flow', 'flows'), ...count('auth', 'authentication', 'authentications')]
  return `Used by ${list.length}: ${parts.join(', ')}`
}
