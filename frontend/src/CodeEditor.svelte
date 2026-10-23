<script lang="ts">
  // A CodeMirror 6 editor for YAML, JSON or plain text. It owns its text while the user types;
  // outside changes to value replace it. It is coloured from the app's theme tokens (tok-* classes,
  // see lib/highlight.ts), not One Dark, so it follows the chosen theme like the rest of the window.
  import { EditorView, basicSetup } from 'codemirror'
  import { Compartment, EditorState } from '@codemirror/state'
  import { Decoration, MatchDecorator, ViewPlugin, type DecorationSet, type ViewUpdate } from '@codemirror/view'
  import { syntaxHighlighting } from '@codemirror/language'
  import { classHighlighter } from '@lezer/highlight'
  import { yaml } from '@codemirror/lang-yaml'
  import { json } from '@codemirror/lang-json'
  import { onMount } from 'svelte'
  import { templatePattern, type CodeLang } from './lib/highlight'

  let {
    value,
    readonly = false,
    lang = 'yaml',
    holdWhileFocused = false,
    onchange,
  }: {
    value: string
    readonly?: boolean
    lang?: CodeLang
    /** Ignore outside changes while the editor has focus: the window echoes each edit back late. */
    holdWhileFocused?: boolean
    onchange?: (text: string) => void
  } = $props()

  let host: HTMLDivElement
  let view: EditorView | undefined
  const language = new Compartment()
  // Read-only can change while the editor is open (a flow's code view follows Edit).
  const access = new Compartment()
  const accessFor = (ro: boolean) => [EditorState.readOnly.of(ro), EditorView.editable.of(!ro)]
  const languageFor = (l: CodeLang) => (l === 'yaml' ? yaml() : l === 'json' ? json() : [])

  // {{templates}} are Flow-I's own, so they are marked in every language.
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

  const theme = EditorView.theme({
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
          language.of(languageFor(lang)),
          syntaxHighlighting(classHighlighter),
          templateMarks,
          theme,
          access.of(accessFor(readonly)),
          EditorView.updateListener.of((u) => {
            if (u.docChanged) onchange?.(u.state.doc.toString())
          }),
        ],
      }),
    })
    return () => view?.destroy()
  })

  $effect(() => {
    view?.dispatch({ effects: language.reconfigure(languageFor(lang)) })
  })
  $effect(() => {
    view?.dispatch({ effects: access.reconfigure(accessFor(readonly)) })
  })

  // Outside changes replace the text; the editor's own changes already match.
  $effect(() => {
    const next = value
    if (!view || view.state.doc.toString() === next) return
    if (holdWhileFocused && view.hasFocus) return
    view.dispatch({ changes: { from: 0, to: view.state.doc.length, insert: next } })
  })

  /** Moves the cursor to a 1-based line and shows it. */
  export function goToLine(line: number) {
    if (!view) return
    const l = view.state.doc.line(Math.min(Math.max(line, 1), view.state.doc.lines))
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
