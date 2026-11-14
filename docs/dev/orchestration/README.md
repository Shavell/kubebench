# Agent orchestration for v0.5

v0.5 is built by several Claude Code agents coordinated with the Workflow
tool. This folder keeps their prompts and scripts **verbatim** so another
machine or session can resume the same plan. Status and next steps:
[`../v0.5-handoff.md`](../v0.5-handoff.md).

| File | What | State |
|---|---|---|
| [`w1-foundation.md`](w1-foundation.md) | W1 workflow: backend foundation ‖ frontend platform ‖ i18n conversion → integrate & verify | ran, stopped mid-way |
| [`packaging-agent.md`](packaging-agent.md) | Background agent: Makefile, Dockerfile, chart, manifests, .deb/.rpm, install.sh, offline bundle, GitHub workflows (manual), Dependabot, deployment/releasing docs | ran, stopped early |
| [`w2-features.md`](w2-features.md) | Generic feature workflow: worktree per feature → serial merge with full checks | not run |
| [`feature-specs.md`](feature-specs.md) | Specs F1–F11 fed to the feature workflow | ready |

## Conventions the prompts rely on

- **Contract first.** Every agent implements
  [`../../design-v0.5.md`](../../design-v0.5.md). When the code must
  deviate, the contract is updated in the same change.
- **Shared working tree, disjoint ownership** (W1). Agents running in the
  same tree only edit the files they own. They never change git state, and
  the orchestrator commits.
- **Worktree per feature** (W2). Each agent works in its own worktree, on
  a branch `feat/<id>`, and commits there. An integrator merges the
  branches one at a time into `release/v0.5.0`, resolves conflicts in the
  aggregator files and runs every check.
- **Ports** are unique per agent: backend on `port`, Vite on `port + 1000`.
  **Scratch files** go outside the repo.
- **Checks**:
  - frontend: `npx tsc -b && npm run build && npm run lint && npm run i18n:check`;
  - backend: `cd backend && test -z "$(gofmt -l .)" && go vet ./... && go test ./... && go build ./...`.
- **Cluster safety.** The homelab (`k3s-homelab`) is read-only for agents.
  Mutation tests use fakes, or a disposable k3d/k3s cluster on the new VM
  (handoff §6).
- **Commits** follow Conventional Commits and end with the trailer
  `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.
- **Never** push, touch remotes, build or push images, or deploy, unless
  the user asks.

## Adapting the scripts to a new session

1. Replace the `SCRATCH` constant with the new session's scratch directory.
2. W1 must continue from the WIP rather than start over. Prepend the
   finished/open checklists from handoff §4 to each build prompt.
3. On a machine with fewer than 4 CPUs the Workflow tool runs at most 2
   agents at a time. The scripts queue work and stay correct; they just
   take longer.
