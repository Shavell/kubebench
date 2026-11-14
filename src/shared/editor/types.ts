/** Languages the bundled Monaco ships (design §6.6). */
export type EditorLanguage = 'yaml' | 'json' | 'plaintext' | 'shell'

/** Size and class of the box an editor fills. */
interface EditorBox {
  /** CSS height; defaults to 100%, so the parent then needs a height of its own. */
  height?: number | string
  className?: string
}

export interface CodeEditorProps extends EditorBox {
  value: string
  /**
   * Called with the whole text after every edit. Without it an editable
   * editor keeps its edits until `value` changes.
   */
  onChange?: (value: string) => void
  /** Default yaml. */
  language?: EditorLanguage
  /** Default false. */
  readOnly?: boolean
  /** Accessible name, e.g. the resource the text belongs to. */
  ariaLabel?: string
  /** Default false. */
  minimap?: boolean
  /** Default true. */
  lineNumbers?: boolean
  /** Soft-wrap long lines. Default false. */
  wordWrap?: boolean
  /** Shown while the text is empty. */
  placeholder?: string
  /** Focus the editor once it is ready, e.g. inside a dialog. */
  autoFocus?: boolean
}

export interface DiffEditorProps extends EditorBox {
  /** Left side: the current state (live object, deployed values, older revision). */
  original: string
  /** Right side: the proposed state. */
  modified: string
  /** Default yaml. */
  language?: EditorLanguage
  /** Whether the modified side is read-only. Default true. */
  readOnly?: boolean
  /** Called with the modified text after every edit (when not read-only). */
  onChange?: (modified: string) => void
  /** Side by side (default) or a single inline view. */
  sideBySide?: boolean
  /** Headings over the two sides, e.g. "Live" and "Edited"; side by side only. */
  originalTitle?: string
  modifiedTitle?: string
}

/** What the lazily loaded Monaco components receive; the box is drawn by the public wrapper. */
export type MonacoCodeEditorProps = Omit<CodeEditorProps, keyof EditorBox>
export type MonacoDiffEditorProps = Omit<DiffEditorProps, keyof EditorBox>
