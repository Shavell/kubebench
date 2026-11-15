import type { ReactNode } from 'react'
import './tabPage.css'

/** Frame of the plugin's editor tabs: the bottom panel's spacing around a full-height view. */
export function TabPage({ children }: { children: ReactNode }) {
  return <div className="term-tab-page">{children}</div>
}
