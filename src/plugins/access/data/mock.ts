import { ApiError } from '@/data/apiClient'
import { delay, delayReject } from '@/data/mock/delay'
import { identityOf, RbacEngine } from './mockEngine'
import { mockRbac } from './mockRbac'
import type { AccessProvider } from './provider'
import type { AccessRequest, Grant, ReviewResult, Subject } from './types'

/**
 * Mock access data over the demo cluster's RBAC (mockRbac), evaluated by
 * the same rules as the server. Reviews are answered by that evaluation,
 * with the reason text the API server's RBAC authorizer gives.
 */

const badRequest = (field: string, message: string) => new ApiError(400, 'bad_request', `${field}: ${message}`)

function requestProblem(r: AccessRequest): ApiError | null {
  if (!r.verb) return badRequest('verb', 'is required')
  if (r.path) {
    if (r.resource || r.subresource || r.group || r.namespace || r.name)
      return badRequest('path', 'a non-resource URL takes no group, resource, namespace or name')
    return r.path.startsWith('/') ? null : badRequest('path', 'must start with /')
  }
  return r.resource ? null : badRequest('resource', 'is required (or path, for a non-resource URL)')
}

function subjectProblem(s: Subject): ApiError | null {
  if (!s.name) return badRequest('name', 'is required')
  if (s.kind === 'ServiceAccount' && !s.namespace) return badRequest('namespace', 'is required for a ServiceAccount')
  if (s.kind !== 'ServiceAccount' && s.namespace) return badRequest('namespace', 'only a ServiceAccount has a namespace')
  return null
}

/** The RBAC authorizer's wording, e.g. `RBAC: allowed by RoleBinding "web/shop" of Role "web"`. */
function reasonOf(grant: Grant): string {
  const binding = grant.binding.namespace ? `${grant.binding.name}/${grant.binding.namespace}` : grant.binding.name
  return `RBAC: allowed by ${grant.binding.kind} "${binding}" of ${grant.role.kind} "${grant.role.name}"`
}

export function createMockAccess(): AccessProvider {
  const engine = new RbacEngine(mockRbac)
  return {
    listSubjects: () => delay(engine.subjects()),
    getSubject: (subject) => {
      const problem = subjectProblem(subject)
      return problem ? delayReject(problem) : delay(engine.subject(subject))
    },
    whoCan: (request) => {
      const problem = requestProblem(request)
      return problem ? delayReject(problem) : delay({ request, subjects: engine.whoCan(request) })
    },
    listRiskyRoles: () => delay(engine.risky()),
    graph: (namespace, includeSystem) => delay(engine.graph(namespace, includeSystem)),
    review: (request) => {
      const { subject, ...access } = request
      const problem = subjectProblem(subject) ?? requestProblem(access)
      if (problem) return delayReject(problem)
      const id = identityOf(subject)
      const grants = engine.allows(subject, access)
      const result: ReviewResult = { allowed: grants.length > 0, groups: id.groups, grants }
      if (id.user) result.user = id.user
      if (grants.length) result.reason = reasonOf(grants[0])
      return delay(result)
    },
  }
}
