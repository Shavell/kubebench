import { DiffEditor, type DiffOnMount } from '@monaco-editor/react'
import { useCallback, useLayoutEffect, useMemo, useRef } from 'react'
import { useTranslation } from 'react-i18next'
import { PLATFORM_NS } from '@/core/locales'
import { useResolvedTheme } from '@/core/theme/themeController'
import { EditorLoading } from './EditorLoading'
import { monacoTheme, useEditorFont } from './monaco'
import type { MonacoDiffEditorProps } from './types'

/** Lazily loaded body of DiffEditor; import DiffEditor instead. */
export default function MonacoDiffEditor({
  original,
  modified,
  language = 'yaml',
  readOnly = true,
  onChange,
  sideBySide = true,
  originalTitle,
  modifiedTitle,
}: MonacoDiffEditorProps) {
  const { t } = useTranslation(PLATFORM_NS)
  const theme = useResolvedTheme()

  const onChangeRef = useRef(onChange)
  useLayoutEffect(() => {
    onChangeRef.current = onChange
  })

  const handleMount = useCallback<DiffOnMount>((editor) => {
    const modifiedEditor = editor.getModifiedEditor()
    modifiedEditor.onDidChangeModelContent(() => onChangeRef.current?.(modifiedEditor.getValue()))
    // Monaco reports an error when a model is disposed while a diff editor
    // still shows it, which is the order the wrapper unmounts in. So the
    // wrapper keeps the models (keepCurrent*Model) and they are released
    // here, once the editor itself is gone.
    const models = editor.getModel()
    editor.onDidDispose(() => {
      models?.original.dispose()
      models?.modified.dispose()
    })
  }, [])

  const { fontFamily, fontSize } = useEditorFont()
  const options = useMemo(
    () => ({
      readOnly,
      originalEditable: false,
      renderSideBySide: sideBySide,
      // Indentation is meaningful in YAML: show whitespace-only changes.
      ignoreTrimWhitespace: false,
      originalAriaLabel: originalTitle ?? t('editor.diff.original'),
      modifiedAriaLabel: modifiedTitle ?? t('editor.diff.modified'),
      fontFamily,
      fontSize,
      minimap: { enabled: false },
      scrollBeyondLastLine: false,
      fixedOverflowWidgets: true,
      automaticLayout: true,
    }),
    [readOnly, sideBySide, originalTitle, modifiedTitle, t, fontFamily, fontSize],
  )

  return (
    <div className="kb-diff">
      {sideBySide && (originalTitle || modifiedTitle) && (
        // Decorative: the same titles are the two editors' accessible names.
        <div className="kb-diff__titles" aria-hidden="true">
          <span>{originalTitle}</span>
          <span>{modifiedTitle}</span>
        </div>
      )}
      <div className="kb-diff__editor">
        <DiffEditor
          height="100%"
          original={original}
          modified={modified}
          language={language}
          theme={monacoTheme(theme)}
          options={options}
          loading={<EditorLoading />}
          keepCurrentOriginalModel
          keepCurrentModifiedModel
          onMount={handleMount}
        />
      </div>
    </div>
  )
}
