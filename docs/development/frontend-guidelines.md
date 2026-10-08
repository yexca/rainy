# Frontend Guidelines

Read [Frontend architecture](../architecture/frontend.md),
[Design](design.md), and the shared contracts in
[contract §9.2](../architecture/contract.md#92-cross-feature-contracts).

## Responsibilities

- Put domain pages and workflows in `web/src/features/<area>`.
- Keep shared components, hooks, stores, and `lib` independent of features.
  Communicate through the documented small shared contracts instead of deep
  imports into another feature.
- Use TanStack Query for server state and the existing player stores for
  transport and queue state. Avoid a second audio engine or queue identity.
- Keep the player mounted across navigation. Failures in a page or optional
  online service must not discard playback and known local state.
- Update `web/src/lib/api/types.ts` with native API shape changes.

## UI Contract

Use semantic theme tokens and the existing shared components. Check 375 px
and 1280 px widths, iOS safe areas, 44 px touch targets, visible keyboard
focus, reduced motion, and light/dark themes. Reserve space through the
player layout variables so content stays reachable.

Every visible string uses i18next and both `en` and `zh` translations.
Tags, lyrics, names, and filenames remain untrusted text rendered through
React. Permission guards in the backend remain authoritative.

## Validation

Use `make ci-style` for lint and `make ci-frontend` for audit, typecheck,
helper tests, and the production build. For interaction changes, run the
Docker development stack and check the affected browser workflow as described
in [Testing](testing.md#browser-checks). Rainy currently has no automated
browser regression target; do not claim a manual check covers every browser.
