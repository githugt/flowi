// The key that sends the open request: Ctrl+Enter, or Cmd+Enter on a Mac. The Send button and the command palette
// advertised it before anything handled it.
export interface KeyLike {
  key: string
  isComposing?: boolean
  ctrlKey: boolean
  metaKey: boolean
  altKey: boolean
  shiftKey: boolean
  repeat: boolean
}

/** True when e is the send key pressed once (a held key repeats, and a repeat must not send again). */
export function isSendKey(e: KeyLike): boolean {
  return (e.ctrlKey || e.metaKey) && !e.altKey && !e.shiftKey && !e.repeat && e.key === 'Enter' && !e.isComposing
}
