## What and why

<!-- What does this change do and why? Link issues with "Fixes #123". -->

## How it was tested

<!-- Commands you ran, clusters you tried, screenshots for UI changes. -->

## Checklist

- [ ] Commits follow [Conventional Commits](https://www.conventionalcommits.org/) (`feat(helm): …`, `fix(api): …`)
- [ ] `make test` passes (Go tests, `tsc`, oxlint, `npm run i18n:check`, `npm test`)
- [ ] New UI strings have translations in every UI language (`en`, `tr`, `de`; `npm run i18n:check`)
- [ ] Live mode shows real data or an "unavailable" state with a reason, never mock data
- [ ] User-facing changes have an entry under `[Unreleased]` in `CHANGELOG.md` and updated docs
- [ ] Chart, packaging or workflow changes pass `make chart-lint` and `make lint`, and `make manifests` output is committed
