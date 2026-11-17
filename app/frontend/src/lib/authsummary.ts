import type { AuthField } from './types'

/** What an authentication adds to the request, in a sentence: the Auth section shows it under the form. "" when nothing is known. */
export function authSummary(kind: string, value: string, fields: AuthField[]): string {
  const p = (name: string) => fields.find((f) => f.name === name)?.value.trim() ?? ''
  const q = (s: string) => '`' + s + '`'
  if (kind === 'bearer') return 'adds the header ' + q('Authorization: Bearer …')
  if (kind === 'basic') return 'adds the header ' + q('Authorization: Basic …')
  if (kind === 'header') return 'adds the header ' + q('Authorization')
  if (!kind.startsWith('scheme:')) return ''
  switch (kind.slice('scheme:'.length)) {
    case 'apikey': {
      const name = p('name') || 'X-Api-Key'
      const where = p('in') || 'header'
      return where === 'query' ? `adds the query parameter ${q(name)}` : where === 'cookie' ? `adds the cookie ${q(name)}` : `adds the header ${q(name)}`
    }
    case 'digest':
      return 'answers the server\'s Digest challenge: the request is sent twice, the second time with an ' + q('Authorization: Digest …') + ' header'
    case 'oauth2':
      return `gets a token${p('grant') ? ' (' + p('grant') + ')' : ''}${p('token_url') ? ' from ' + p('token_url') : ''} and adds ${q('Authorization: Bearer …')}`
    case 'oauth1':
      return `signs the request with OAuth 1.0${p('signature_method') ? ' (' + p('signature_method') + ')' : ''} in the ${q('Authorization')} header`
    case 'aws_sigv4':
      return `signs the request with AWS SigV4${p('region') ? ' for ' + p('region') : ''}${p('service') ? ' / ' + p('service') : ''}`
    case 'jwt':
      return `signs a JWT${p('algorithm') ? ' (' + p('algorithm') + ')' : ''} and adds it as ${q(p('header') || 'Authorization: Bearer …')}`
    case 'hawk':
      return 'signs the request with Hawk in the ' + q('Authorization') + ' header'
    case 'mtls':
      return 'presents your client certificate when it connects (no header is added)'
    case 'ntlm':
      return 'logs in with NTLM: the request is sent more than once while the server and Flowi agree'
    case 'httpsig':
      return 'signs the request (HTTP Message Signatures) and adds ' + q('Signature') + ' and ' + q('Signature-Input') + ' headers'
    case 'dpop':
      return 'adds a ' + q('DPoP') + ' proof header that is tied to this request'
  }
  return ''
}
