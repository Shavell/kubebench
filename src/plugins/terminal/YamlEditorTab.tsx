import Editor from '@monaco-editor/react'
import type { TabDescriptor } from '@/core/plugin/types'

/**
 * Monaco-based YAML tab. Registered under kind "yaml"; any plugin opens
 * one via openTab({kind: "yaml", props: {text, readOnly?}}).
 */
export function YamlEditorTab({ tab }: { tab: TabDescriptor }) {
  const text = (tab.props?.text as string) ?? ''
  const readOnly = (tab.props?.readOnly as boolean) ?? true

  return (
    <Editor
      language="yaml"
      theme="vs-dark"
      value={text}
      options={{
        readOnly,
        minimap: { enabled: true },
        fontSize: 13,
        fontFamily: 'SF Mono, Menlo, Consolas, monospace',
        scrollBeyondLastLine: false,
        renderLineHighlight: 'all',
        automaticLayout: true,
      }}
    />
  )
}
