import { registry } from '@/core/plugin/registry'
import { ALL_NAMESPACES, useContextStore } from '@/core/store/contextStore'
import { useProviderQuery } from '@/data/DataContext'
import { dataMode } from '@/data/mode'
import { Icon } from '@/shared/Icon'
import './StatusBar.css'

function ContextItem() {
  const contexts = useContextStore((s) => s.contexts)
  const activeContext = useContextStore((s) => s.activeContext)
  const setActiveContext = useContextStore((s) => s.setActiveContext)

  return (
    <div className="status-bar__item status-bar__item--select" title="Kubernetes Context">
      <Icon name="vm" size={13} />
      <select
        className="status-bar__select"
        value={activeContext}
        onChange={(e) => setActiveContext(e.target.value)}
      >
        {contexts.map((c) => (
          <option key={c} value={c}>
            {c}
          </option>
        ))}
      </select>
    </div>
  )
}

function NamespaceItem() {
  const namespace = useContextStore((s) => s.namespace)
  const setNamespace = useContextStore((s) => s.setNamespace)
  const { data } = useProviderQuery((p) => p.cluster.listNamespaces(), [])

  return (
    <div className="status-bar__item status-bar__item--select" title="Namespace Filter">
      <Icon name="folder" size={13} />
      <select
        className="status-bar__select"
        value={namespace}
        onChange={(e) => setNamespace(e.target.value)}
      >
        <option value={ALL_NAMESPACES}>all namespaces</option>
        {(data ?? []).map((ns) => (
          <option key={ns.name} value={ns.name}>
            {ns.name}
          </option>
        ))}
      </select>
    </div>
  )
}

export function StatusBar() {
  return (
    <div className="status-bar">
      <div className="status-bar__side">
        <ContextItem />
        <NamespaceItem />
        {registry.statusBarItems('left').map((item) => (
          <item.component key={item.id} />
        ))}
      </div>
      <div className="status-bar__side">
        {registry.statusBarItems('right').map((item) => (
          <item.component key={item.id} />
        ))}
        <div
          className="status-bar__item"
          title={
            dataMode.mode === 'live'
              ? `Connected to ${dataMode.server}`
              : 'Mock data — no cluster connected (start the backend)'
          }
        >
          <Icon name={dataMode.mode === 'live' ? 'broadcast' : 'beaker'} size={13} />
          <span>{dataMode.mode}</span>
        </div>
        {dataMode.user && (
          <button
            className="status-bar__item status-bar__item--button"
            title="Sign out"
            onClick={async () => {
              await fetch('/api/auth/logout', { method: 'POST' })
              window.location.reload()
            }}
          >
            <Icon name="account" size={13} />
            <span>{dataMode.user}</span>
            <Icon name="sign-out" size={13} />
          </button>
        )}
      </div>
    </div>
  )
}
