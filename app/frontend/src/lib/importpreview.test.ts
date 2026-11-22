import { describe, expect, it } from 'vitest'
import { groupFiles, importLabel, previewState, shownPath, statusNote, supportingFiles, writtenCount, type PreviewFile } from './importpreview'

const file = (over: Partial<PreviewFile>): PreviewFile => ({ rel: '', exists: false, remove: false, kind: 'other', name: '', status: 'new', ...over })

const asyncapi: PreviewFile[] = [
  file({ rel: 'descriptions/Flowi-manual-test-WebSocket.asyncapi.yaml', kind: 'document' }),
  file({ rel: 'requests/demo/ws/echo.req.yaml', kind: 'request', name: 'Echo (send a message, get it back)', exists: true, status: 'differs' }),
  file({ rel: 'requests/demo/ws/receiveTicks.req.yaml', kind: 'request', name: 'Receive ticks' }),
]

describe('the import preview', () => {
  it('should say where a request lands, as a path in the collection and not as a file', () => {
    expect(shownPath(asyncapi[1])).toBe('demo/ws/echo')
    expect(shownPath(file({ rel: 'flows/api/v1/a-to-b.arazzo.yaml', kind: 'flow' }))).toBe('api/v1/a-to-b')
    expect(shownPath(file({ rel: 'descriptions/x.openapi.yaml', kind: 'document' }))).toBe('descriptions/x.openapi.yaml')
  })

  it('should list requests by title, and leave the kept document out of them', () => {
    // Before: the preview was a file tree, where the kept copy of the document looked like a third request.
    const groups = groupFiles(asyncapi, 'skip')

    expect(groups.map((g) => g.title)).toEqual(['Requests'])
    expect(groups[0].entries.map((e) => e.name)).toEqual(['Echo (send a message, get it back)', 'Receive ticks'])
    expect(supportingFiles(asyncapi).map((f) => f.kind)).toEqual(['document'])
  })

  it('should carry the verb of each request, so the preview shows the same pill as the request tree', () => {
    // Before: SNS only publishes, so its queue read is an SQS request; the preview listed two SNS-looking titles and one looked lost.
    const sns = [
      file({ rel: 'requests/snsPublishNews.req.yaml', kind: 'request', name: 'Publish to the topic', verb: 'SNS' }),
      file({ rel: 'requests/snsReceiveFromQueue.req.yaml', kind: 'request', name: 'Receive from the queue', verb: 'SQS' }),
      file({ rel: 'environments/local.yaml', kind: 'environment', name: 'local' }),
    ]

    const groups = groupFiles(sns, 'skip')

    expect(groups[0].entries.map((e) => e.verb)).toEqual(['SNS', 'SQS'])
    expect(groups[1].entries[0].verb).toBe('')
  })

  it('should word an existing request by what the chosen way would do with it', () => {
    expect(groupFiles(asyncapi, 'skip')[0].entries[0]).toMatchObject({ note: 'exists: yours is kept', tone: 'warn' })
    expect(groupFiles(asyncapi, 'overwrite')[0].entries[0]).toMatchObject({ note: 'exists: will be replaced', tone: 'warn' })
    expect(groupFiles(asyncapi, 'refuse')[0].entries[0].note).toBe('exists: stops the import')
    expect(statusNote('same', 'overwrite')).toEqual({ note: 'already the same', tone: 'muted' })
    expect(statusNote('new', 'skip').tone).toBe('ok')
  })

  it('should name the different request that is in the way, so a clash can be found in the sidebar', () => {
    const clash = file({ rel: 'requests/Messaging/receiveTicks.req.yaml', kind: 'request', name: 'Receive ticks', exists: true, status: 'differs', existingName: 'Subscribe to flowi.demo.ticks' })

    expect(groupFiles([clash], 'skip')[0].entries[0].note).toBe('exists (“Subscribe to flowi.demo.ticks”): yours is kept')
    expect(groupFiles([clash], 'overwrite')[0].entries[0].note).toBe('exists (“Subscribe to flowi.demo.ticks”): will be replaced')
    expect(statusNote('differs', 'skip').note).toBe('exists: yours is kept')
  })

  it('should count what the Import button will write, by the way existing files are treated', () => {
    expect(writtenCount(asyncapi, 'skip')).toBe(1)
    expect(writtenCount(asyncapi, 'overwrite')).toBe(2)
    expect(importLabel(asyncapi, 'skip', 0)).toBe('Import 1 item')
    expect(importLabel(asyncapi, 'overwrite', 0)).toBe('Import 2 items')
    expect(importLabel([file({ kind: 'request', status: 'differs', exists: true })], 'skip', 0)).toBe('Import')
    expect(importLabel(asyncapi, 'skip', 2)).toBe('Import 1 item (the rest)')
  })

  it('should choose what the preview area shows, a failure before a stale preview', () => {
    const base = { previewed: false, previewing: false, waiting: '' }

    expect(previewState({ ...base, done: 'Imported 1 request into X' })).toBe('done')
    expect(previewState({ ...base, error: 'FLW-IMPORT-002: not an AsyncAPI document' })).toBe('failed')
    expect(previewState({ ...base, previewed: true })).toBe('ready')
    expect(previewState({ ...base, previewing: true })).toBe('reading')
    expect(previewState({ ...base, waiting: 'Choose a file or paste the text to see what would be imported.' })).toBe('waiting')
  })
})
