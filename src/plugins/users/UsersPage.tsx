import { useState } from 'react'
import { refreshData, useProviderQuery, useProviders } from '@/data/DataContext'
import type { AppUser, ClusterEntry } from '@/data/types'
import { Badge } from '@/shared/Badge'
import { ConfirmButton } from '@/shared/ConfirmButton'
import { Icon } from '@/shared/Icon'
import { Toolbar } from '@/shared/Toolbar'
import '@/plugins/workloads/pages/pages.css'
import '@/plugins/clusters/inventory.css'
import './users.css'

export function UsersPage() {
  const providers = useProviders()
  const { data: users, loading } = useProviderQuery((p) => p.users.listUsers(), [])
  const { data: clusters } = useProviderQuery((p) => p.inventory.listClusters(), [])
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [role, setRole] = useState<'admin' | 'user'>('user')
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  const run = async (op: () => Promise<void>) => {
    if (busy) return
    setBusy(true)
    setError(null)
    try {
      await op()
      refreshData()
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    } finally {
      setBusy(false)
    }
  }

  const createUser = () =>
    run(async () => {
      await providers.users.createUser(username, password, role)
      setUsername('')
      setPassword('')
      setRole('user')
    })

  const toggleCluster = (user: AppUser, cluster: ClusterEntry, on: boolean) =>
    run(async () => {
      const current = (clusters ?? [])
        .filter((c) => user.clusters.includes(c.name))
        .map((c) => c.id!)
      const next = on
        ? [...new Set([...current, cluster.id!])]
        : current.filter((id) => id !== cluster.id)
      await providers.users.assignClusters(user.id, next)
    })

  return (
    <div className="page">
      <Toolbar title="Users" />
      <div className="detail">
        {error && (
          <div className="detail__error">
            <Icon name="error" size={14} /> {error}
          </div>
        )}

        <table className="kb-table detail__env">
          <thead>
            <tr>
              <th>User</th>
              <th>Role</th>
              <th>Assigned clusters</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {(users ?? []).map((u: AppUser) => (
              <tr key={u.id}>
                <td>
                  <Icon name="account" size={14} style={{ marginRight: 6, verticalAlign: 'middle' }} />
                  {u.username}
                </td>
                <td>
                  <Badge tone={u.role === 'admin' ? 'info' : 'neutral'}>{u.role}</Badge>
                </td>
                <td>
                  {u.role === 'admin' ? (
                    <span className="users__all">all clusters</span>
                  ) : (
                    <div className="users__clusters">
                      {(clusters ?? []).map((c) => (
                        <label key={c.name} className="users__cluster">
                          <input
                            type="checkbox"
                            checked={u.clusters.includes(c.name)}
                            disabled={busy}
                            onChange={(e) => toggleCluster(u, c, e.target.checked)}
                          />
                          {c.name}
                        </label>
                      ))}
                      {(clusters ?? []).length === 0 && <span className="users__all">no clusters</span>}
                    </div>
                  )}
                </td>
                <td>
                  <ConfirmButton
                    icon="trash"
                    label="Delete"
                    confirmLabel="Confirm?"
                    onConfirm={() => run(() => providers.users.deleteUser(u.id))}
                  />
                </td>
              </tr>
            ))}
          </tbody>
        </table>
        {loading && <div className="page-loading">Loading users…</div>}

        <div className="detail__section">Add User</div>
        <div className="inventory__form">
          <label>
            Username
            <input value={username} onChange={(e) => setUsername(e.target.value)} spellCheck={false} />
          </label>
          <label>
            Password
            <input
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              placeholder="at least 4 characters"
            />
          </label>
          <label>
            Role
            <select value={role} onChange={(e) => setRole(e.target.value as 'admin' | 'user')}>
              <option value="user">user</option>
              <option value="admin">admin</option>
            </select>
          </label>
          <button
            className="kb-button"
            disabled={busy || !username || password.length < 4}
            onClick={createUser}
          >
            <Icon name="add" size={14} /> Add User
          </button>
        </div>
      </div>
    </div>
  )
}
