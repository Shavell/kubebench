import { useTranslation } from 'react-i18next'
import { Select } from '@/shared/form'
import type { BrowserPort, IngressRoute, Scheme } from '../data/types'
import { NS } from '../i18n'
import { routeKey } from '../model/ingress'
import { portLabel } from '../model/ports'

const SCHEMES: Scheme[] = ['http', 'https']

function SchemePicker({ value, onChange }: { value: Scheme; onChange: (scheme: Scheme) => void }) {
  const { t } = useTranslation(NS)
  return (
    <Select
      className="sb-select"
      aria-label={t('toolbar.scheme')}
      value={value}
      options={SCHEMES.map((s) => ({ value: s, label: s }))}
      onChange={onChange}
    />
  )
}

interface PortPickerProps {
  ports: BrowserPort[]
  port: number
  scheme: Scheme
  onChange: (port: number, scheme: Scheme) => void
}

/** The port of a Service or Pod and the scheme; a new port takes its guessed scheme. */
export function PortPicker({ ports, port, scheme, onChange }: PortPickerProps) {
  const { t } = useTranslation(NS)
  return (
    <>
      <Select
        className="sb-select"
        aria-label={t('toolbar.port')}
        value={port}
        options={ports.map((p) => ({ value: p.port, label: portLabel(p) }))}
        onChange={(next) => onChange(next, ports.find((p) => p.port === next)?.scheme ?? 'http')}
      />
      <SchemePicker value={scheme} onChange={(next) => onChange(port, next)} />
    </>
  )
}

interface RoutePickerProps {
  routes: IngressRoute[]
  route: IngressRoute
  scheme: Scheme
  onChange: (route: IngressRoute, scheme: Scheme) => void
}

/**
 * The route of an Ingress (rule host and path → Service port) and the
 * scheme the Service is reached with; a new route takes its guessed scheme.
 */
export function RoutePicker({ routes, route, scheme, onChange }: RoutePickerProps) {
  const { t } = useTranslation(NS)
  const label = (r: IngressRoute) => {
    const from = r.defaultBackend ? t('ingress.defaultBackend') : `${r.host || '*'}${r.path}`
    return t('ingress.route', { from, service: r.service, port: r.port })
  }
  return (
    <>
      <Select
        className="sb-select sb-select--route"
        aria-label={t('toolbar.route')}
        value={routeKey(route)}
        options={routes.map((r) => ({ value: routeKey(r), label: label(r) }))}
        onChange={(key) => {
          const next = routes.find((r) => routeKey(r) === key) ?? route
          onChange(next, next.scheme)
        }}
      />
      <SchemePicker value={scheme} onChange={(next) => onChange(route, next)} />
    </>
  )
}
