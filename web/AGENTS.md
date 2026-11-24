# Frontend Agent Guide

The root [AGENTS.md](../AGENTS.md) invariants apply. The
[contract](../docs/architecture/contract.md) is large; open only the section a
change needs.

## Rules

- Follow [Design](../docs/development/design.md): mobile-first at 375px,
  safe-area padding, 44px touch targets, a persistent player and queue, and
  visible keyboard focus.
- Every user-visible string uses i18next with both translations; `en` is the
  base locale and fallback, so write it first. Stored and API text arrives as
  codes or English and is translated here
  ([contract §9.6](../docs/architecture/contract.md#96-i18n)).
- Avoid redundant helper labels and explanatory captions; let layout,
  grouping, and visual hierarchy explain themselves, keeping accessible names
  ([Principles](../docs/development/design.md#principles)).
- A native API shape change updates `web/src/lib/api/types.ts`, the Go models,
  and [contract §8](../docs/architecture/contract.md#8-typescript-contract-websrclibapitypests)
  in the same change.
- Feature boundaries, state ownership, IME-aware inputs, and the UI checklist
  are in [Frontend guidelines](../docs/development/frontend-guidelines.md).
- Browser checks use accessible roles and labels; see
  [Browser checks](../docs/development/testing.md#browser-checks).
- When visible behavior changes, update the matching
  [User guide](../docs/user/index.md) page.

## Which Page To Open

| Change | Page |
| --- | --- |
| Routes and page structure | [Structure](../docs/architecture/frontend.md#structure), [contract §9.1](../docs/architecture/contract.md#91-routes) |
| Shared state, player store, cross-feature hooks | [State](../docs/architecture/frontend.md#state), [contract §9.2](../docs/architecture/contract.md#92-cross-feature-contracts) |
| Theme, layout, typography, icons | [Design](../docs/development/design.md), [contract §9.3](../docs/architecture/contract.md#93-design-system) |
| Player, mini player, PWA | [Player and PWA](../docs/architecture/frontend.md#player-and-pwa), [contract §9.4](../docs/architecture/contract.md#94-ios-style-player) |
| Management pages | [contract §9.5](../docs/architecture/contract.md#95-management-ui), [Library management](../docs/architecture/library-management.md) |
| Listening pages | [contract §9.5a](../docs/architecture/contract.md#95a-listening-ui) |
| Translations | [Internationalization](../docs/architecture/frontend.md#internationalization), [contract §9.6](../docs/architecture/contract.md#96-i18n) |
| Response shapes | [contract §7](../docs/architecture/contract.md#7-native-api-api--json-contract), [§8](../docs/architecture/contract.md#8-typescript-contract-websrclibapitypests) |
| Tests | [Frontend tests](../docs/development/testing.md#frontend), [Browser checks](../docs/development/testing.md#browser-checks) |
