import { useSettingsStore } from '@/core/store/settingsStore'
import { dataMode } from '@/data/mode'
import { Toolbar } from '@/shared/Toolbar'
import '@/plugins/workloads/pages/pages.css'
import './settings.css'

interface RowProps {
  title: string
  description: string
  children: React.ReactNode
}

function SettingRow({ title, description, children }: RowProps) {
  return (
    <div className="setting">
      <div className="setting__text">
        <div className="setting__title">{title}</div>
        <div className="setting__description">{description}</div>
      </div>
      <div className="setting__control">{children}</div>
    </div>
  )
}

export function SettingsPage() {
  const settings = useSettingsStore()

  return (
    <div className="page">
      <Toolbar title="Settings" />
      <div className="detail settings">
        <div className="detail__section">Appearance</div>
        <SettingRow title="Color Theme" description="The workbench color theme.">
          <select value={settings.theme} disabled title="More themes coming later">
            <option value="dark">Dark+ (default)</option>
          </select>
        </SettingRow>

        <div className="detail__section">Data</div>
        <SettingRow
          title="Metrics Window"
          description="How much utilization history charts show, in minutes."
        >
          <select
            value={settings.metricsWindow}
            onChange={(e) => settings.set('metricsWindow', Number(e.target.value))}
          >
            <option value={15}>15 minutes</option>
            <option value={30}>30 minutes</option>
            <option value={60}>60 minutes</option>
          </select>
        </SettingRow>
        <SettingRow
          title="Auto Refresh"
          description="Refetch resource lists periodically. Off means manual refresh only."
        >
          <select
            value={settings.autoRefreshSeconds}
            onChange={(e) => settings.set('autoRefreshSeconds', Number(e.target.value))}
          >
            <option value={0}>Off</option>
            <option value={10}>Every 10s</option>
            <option value={30}>Every 30s</option>
            <option value={60}>Every 60s</option>
          </select>
        </SettingRow>

        <div className="detail__section">Terminal & Logs</div>
        <SettingRow
          title="Font Size"
          description="Font size for the terminal, exec and log views, in pixels."
        >
          <select
            value={settings.terminalFontSize}
            onChange={(e) => settings.set('terminalFontSize', Number(e.target.value))}
          >
            {[11, 12, 12.5, 13, 14, 15, 16].map((size) => (
              <option key={size} value={size}>
                {size} px
              </option>
            ))}
          </select>
        </SettingRow>

        <div className="detail__section">About</div>
        <div className="settings__about">
          KubeBench · data source: <strong>{dataMode.mode}</strong> ({dataMode.context})
          <br />
          Settings persist in this browser (localStorage).
        </div>
      </div>
    </div>
  )
}
