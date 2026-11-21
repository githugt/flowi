import type { DataTableView } from './types'

/** What a table chip says it holds, for its tooltip: the file or sheet, the row count and the columns with their types. */
export function tableTitle(t: DataTableView): string {
  const where = t.sheet ? `${t.file}, sheet ${t.sheet}` : t.file
  const rows = `${t.rows} ${t.rows === 1 ? 'row' : 'rows'}`
  const cols = t.columns.map((c) => `${c.name} ${c.type.toLowerCase()}`).join(', ')
  return `${where} · ${rows}\n${cols}`
}

/** Whether a statement holds nothing yet, so a table chip can fill it without asking. */
export const isBlankQuery = (query: string): boolean => query.trim() === ''
