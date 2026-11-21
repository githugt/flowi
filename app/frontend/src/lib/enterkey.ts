// Enter that confirms an action. While an input method (Chinese, Japanese, Korean and others) is composing text,
// Enter picks the candidate and must not also run the action: before, confirming a conversion sent the request.
export interface EnterLike {
  key: string
  isComposing?: boolean
  keyCode?: number
}

/** True when e is an Enter that is not part of text composition. keyCode 229 is how Safari reports composition. */
export function isEnter(e: EnterLike): boolean {
  return e.key === 'Enter' && !e.isComposing && e.keyCode !== 229
}
