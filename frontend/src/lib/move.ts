// Moving requests to another collection or folder, from the Move dialog, a drag onto a tree row,
// or the selection bar. Their tabs close first, since the move gives them new rows. A move takes each
// request's edit slot along, so open edits are stored in it first rather than refusing to move a
// request with unsaved changes (a draft that was never saved always has some).
import { RequestService, WorkspaceService } from '../../bindings/github.com/githugt/flowi/internal/bridge'
import { app } from './app.svelte'
import { ui } from './ui.svelte'
import { tabsAffected } from './menus'

/** Moves the request rows into a collection or request-folder row. */
export async function moveRequests(ids: string[], into: string) {
  const tabs = await tabsAffected(ids)
  if (!(await RequestService.Keep(tabs))) {
    ui.toast('A request has changes only Save can keep (a new version or default). Save it, then move it.', 'error')
    return
  }
  await app.closeAll(tabs)
  app.marked = []
  await WorkspaceService.MoveInto(ids, into)
}

/** The rows a drag of `id` takes: every marked row when id is one of them, else id alone. Rows that are
 *  not requests stay where they are (the bridge skips them). */
export function movingWith(id: string): string[] {
  return app.marked.includes(id) ? [...app.marked] : [id]
}
