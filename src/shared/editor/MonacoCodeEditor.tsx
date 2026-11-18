import Editor, { type OnMount } from '@monaco-editor/react'
import { useCallback, useLayoutEffect, useMemo, useRef } from 'react'
import { useResolvedTheme } from '@/core/theme/themeController'
import { EditorLoading } from './EditorLoading'
import { monacoTheme, useEditorFont } from './monaco'
import type { MonacoCodeEditorProps } from './types'

const focusEditor: OnMount = (editor) => editor.focus()

/** Lazily loaded body of CodeEditor; import CodeEditor instead. */
export default function MonacoCodeEditor({
  value,
  onChange,
  language = 'yaml',
  readOnly = false,
  ariaLabel,
  minimap = false,
  lineNumbers = true,
  wordWrap = false,
  placeholder,
  autoFocus = false,
}: MonacoCodeEditorProps) {
  const theme = useResolvedTheme()

  // A stable change handler, so the wrapper does not re-subscribe on every render.
  const onChangeRef = useRef(onChange)
  useLayoutEffect(() => {
    onChangeRef.current = onChange
  })
  const handleChange = useCallback((text: string | undefined) => onChangeRef.current?.(text ?? ''), [])

  const { fontFamily, fontSize } = useEditorFont()
  const options = useMemo(
    () => ({
      readOnly,
      ariaLabel,
      placeholder,
      minimap: { enabled: minimap },
      lineNumbers: lineNumbers ? ('on' as const) : ('off' as const),
      wordWrap: wordWrap ? ('on' as const) : ('off' as const),
      fontFamily,
      fontSize,
      tabSize: 2,
      insertSpaces: true,
      detectIndentation: false,
      scrollBeyondLastLine: false,
      renderLineHighlight: 'all' as const,
      // Hovers and suggestions may extend beyond clipping containers such as dialogs.
      fixedOverflowWidgets: true,
      automaticLayout: true,
    }),
    [readOnly, ariaLabel, placeholder, minimap, lineNumbers, wordWrap, fontFamily, fontSize],
  )

  return (
    <Editor
      height="100%"
      value={value}
      language={language}
      theme={monacoTheme(theme)}
      options={options}
      loading={<EditorLoading />}
      onChange={onChange ? handleChange : undefined}
      // Read once by the wrapper, on the first mount.
      onMount={autoFocus ? focusEditor : undefined}
    />
  )
}
