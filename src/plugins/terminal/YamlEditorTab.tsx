import { useTranslation } from 'react-i18next'
import { PLATFORM_NS } from '@/core/locales'
import type { TabDescriptor } from '@/core/plugin/types'
import { CodeEditor } from '@/shared/editor/CodeEditor'

/**
 * YAML tab, registered under kind "yaml". Any plugin opens one with
 * openTab({ kind: 'yaml', props: { text, readOnly? } }). `readOnly`
 * defaults to true; `readOnly: false` allows local edits that are never
 * saved or applied.
 */
export function YamlEditorTab({ tab }: { tab: TabDescriptor }) {
  const { t } = useTranslation(PLATFORM_NS)
  const text = typeof tab.props?.text === 'string' ? tab.props.text : ''
  const readOnly = tab.props?.readOnly !== false

  return (
    <CodeEditor
      value={text}
      language="yaml"
      readOnly={readOnly}
      minimap
      ariaLabel={t('editor.yamlTab.ariaLabel', { title: tab.title })}
    />
  )
}
