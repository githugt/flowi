<script lang="ts">
  // A CodeMirror 6 editor for YAML, JSON or plain text. It owns its text while the user types;
  // outside changes to value replace it. It is coloured from the app's theme tokens (tok-* classes,
  // see lib/highlight.ts), not One Dark, so it follows the chosen theme like the rest of the window.
  import { alreadyShown } from './lib/codeecho'
  import { clamp } from './lib/clamp'
  import { EditorView, basicSetup } from 'codemirror'
  import { Compartment, EditorState, Prec, StateEffect, StateField } from '@codemirror/state'
  import { Decoration, MatchDecorator, ViewPlugin, WidgetType, keymap, type DecorationSet, type ViewUpdate } from '@codemirror/view'
  import { syntaxHighlighting } from '@codemirror/language'
  import { classHighlighter } from '@lezer/highlight'
  import { yaml } from '@codemirror/lang-yaml'
  import { json } from '@codemirror/lang-json'
  import { xmlLanguage } from './lib/xmllang'
  import { sqlLanguage } from './lib/sqllang'
  import { onMount } from 'svelte'
  import { templatePattern, type CodeLang } from './lib/highlight'
  import type { StepNote } from './lib/stepnotes'

  let {
    value,
    readonly = false,
    lang = 'yaml',
    holdWhileFocused = false,
    onchange,
    oncursor,
    notes = [],
  }: {
    value: string
    readonly?: boolean
    lang?: CodeLang
    /** Ignore outside changes while the editor has focus: the window echoes each edit back late. */
    holdWhileFocused?: boolean
    onchange?: (text: string) => void
    /** Called with the 1-based line the caret is on, when the user moves it. */
    oncursor?: (line: number) => void
    /** Lines of text drawn above some lines, read-only and not part of the text (the code view's comment on each step). */
    notes?: StepNote[]
  } = $props()

  let host: HTMLDivElement
  let view: EditorView | undefined
  const language = new Compartment()
  // Read-only can change while the editor is open (a flow's code view follows Edit).
  const access = new Compartment()
  // Read-only text stays focusable, so its caret can be placed (the flow's code view selects the step under it) and its text copied.
  const accessFor = (ro: boolean) => [EditorState.readOnly.of(ro)]
  const languageFor = (l: CodeLang) => (l === 'yaml' ? yaml() : l === 'json' ? json() : l === 'xml' ? xmlLanguage : l === 'sql' ? sqlLanguage : [])

  // {{templates}} are Flowi's own, so they are marked in every language.
  const templates = new MatchDecorator({ regexp: new RegExp(templatePattern.source, 'g'), decoration: Decoration.mark({ class: 'tok-template' }) })
  const templateMarks = ViewPlugin.fromClass(
    class {
      decorations: DecorationSet
      constructor(v: EditorView) {
        this.decorations = templates.createDeco(v)
      }
      update(u: ViewUpdate) {
        this.decorations = templates.updateDeco(u, this.decorations)
      }
    },
    { decorations: (p) => p.decorations },
  )

  // Notes sit between lines as widgets: they are not in the document, so they are not saved, copied or edited.
  class NoteWidget extends WidgetType {
    text: string
    indent: number
    constructor(text: string, indent: number) {
      super()
      this.text = text
      this.indent = indent
    }
    eq(other: NoteWidget) {
      return other.text === this.text && other.indent === this.indent
    }
    toDOM() {
      const el = document.createElement('div')
      el.className = 'cm-stepnote'
      el.style.paddingLeft = `${this.indent}ch`
      el.textContent = '# ' + this.text
      el.title = 'Made from the request each time this is shown. It is not part of the flow.'
      return el
    }
    ignoreEvent() {
      return true
    }
  }
  const setNotes = StateEffect.define<StepNote[]>()
  const noteField = StateField.define<DecorationSet>({
    create: () => Decoration.none,
    update(deco, tr) {
      deco = deco.map(tr.changes)
      for (const e of tr.effects) {
        if (!e.is(setNotes)) continue
        const doc = tr.state.doc
        deco = Decoration.set(
          e.value
            .filter((n) => n.line >= 1 && n.line <= doc.lines)
            .map((n) => Decoration.widget({ widget: new NoteWidget(n.text, n.indent), block: true, side: -1 }).range(doc.line(n.line).from)),
          true,
        )
      }
      return deco
    },
    provide: (f) => EditorView.decorations.from(f),
  })

  const theme = EditorView.theme({
    '.cm-stepnote': { color: 'var(--muted)', fontStyle: 'italic', padding: '1px 0', userSelect: 'none', cursor: 'default' },
    '&': { height: '100%', backgroundColor: 'var(--bg)', color: 'var(--text)' },
    '&.cm-focused': { outline: 'none' },
    '.cm-scroller': { fontFamily: 'var(--mono)', fontSize: '12.5px' },
    '.cm-content': { caretColor: 'var(--text)' },
    '.cm-cursor, .cm-dropCursor': { borderLeftColor: 'var(--text)' },
    '.cm-gutters': { backgroundColor: 'var(--panel)', color: 'var(--muted)', border: 'none', borderRight: '1px solid var(--line)' },
    '.cm-activeLine': { backgroundColor: 'color-mix(in srgb, var(--accent) 6%, transparent)' },
    '.cm-activeLineGutter': { backgroundColor: 'color-mix(in srgb, var(--accent) 10%, transparent)', color: 'var(--text)' },
    '&.cm-focused > .cm-scroller > .cm-selectionLayer .cm-selectionBackground, .cm-selectionBackground, .cm-content ::selection': {
      backgroundColor: 'color-mix(in srgb, var(--accent) 30%, transparent)',
    },
    '.cm-matchingBracket': { backgroundColor: 'color-mix(in srgb, var(--accent) 22%, transparent)', outline: 'none' },
    '.cm-foldPlaceholder': { backgroundColor: 'var(--panel)', border: '1px solid var(--line)', color: 'var(--muted)' },
    '.cm-tooltip': { backgroundColor: 'var(--panel)', border: '1px solid var(--line)', color: 'var(--text)' },
  })

  onMount(() => {
    view = new EditorView({
      parent: host,
      state: EditorState.create({
        doc: value,
        extensions: [
          basicSetup,
          // Mod-Enter sends the request (the page handles it). The stock keymap would insert a blank line with it.
          Prec.highest(keymap.of([{ key: 'Mod-Enter', run: () => true }])),
          // The stock keymap folds on Ctrl-Shift-[ and ] and selects syntax on Mod-I: the window's own shortcuts. Taking them
          // here (and doing nothing) leaves the page its say. Before: they folded code or moved the selection instead.
          Prec.highest(keymap.of(['Ctrl-Shift-[', 'Ctrl-Shift-]', 'Mod-i'].map((key) => ({ key, run: () => true })))),
          language.of(languageFor(lang)),
          syntaxHighlighting(classHighlighter),
          templateMarks,
          noteField,
          theme,
          access.of(accessFor(readonly)),
          EditorView.updateListener.of((u) => {
            if (u.docChanged) {
              lastSeen = u.state.doc.toString()
              onchange?.(lastSeen)
            }
            if (u.selectionSet && u.transactions.some((tr) => tr.isUserEvent('select') || tr.isUserEvent('input'))) oncursor?.(u.state.doc.lineAt(u.state.selection.main.head).number)
          }),
        ],
      }),
    })
    return () => view?.destroy()
  })

  // The text the editor last reported or was set to: the echo of it from the parent is skipped without a comparison.
  let lastSeen: string | null = null
  $effect(() => {
    view?.dispatch({ effects: language.reconfigure(languageFor(lang)) })
  })
  $effect(() => {
    view?.dispatch({ effects: access.reconfigure(accessFor(readonly)) })
  })
  // Set after the text, in the same tick, so the lines the notes name are the ones in the editor.
  $effect(() => {
    const list = notes
    queueMicrotask(() => view?.dispatch({ effects: setNotes.of(list) }))
  })

  // Outside changes replace the text; the editor's own changes already match.
  $effect(() => {
    const next = value
    if (!view || alreadyShown(lastSeen, next, () => view!.state.doc.toString())) return
    if (holdWhileFocused && view.hasFocus) return
    view.dispatch({ changes: { from: 0, to: view.state.doc.length, insert: next } })
  })

  /** Moves the cursor to a 1-based line and shows it. */
  export function goToLine(line: number) {
    if (!view) return
    const l = view.state.doc.line(clamp(line, 1, view.state.doc.lines))
    view.dispatch({ selection: { anchor: l.from }, effects: EditorView.scrollIntoView(l.from, { y: 'center' }) })
    view.focus()
  }

  /** Puts the caret in the editor. */
  export function focus() {
    view?.focus()
  }
</script>

<div class="host" bind:this={host}></div>

<style>
  .host { height: 100%; min-height: 0; overflow: hidden; -webkit-user-select: text; user-select: text; }
  .host :global(.cm-editor) { height: 100%; }
</style>
