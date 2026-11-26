# Frontend Guidelines

Read [Frontend architecture](../architecture/frontend.md),
[Design](design.md), and the shared contracts in
[contract §9.2](../architecture/contract.md#92-cross-feature-contracts).

## Responsibilities

- Put domain pages and workflows in `web/src/features/<area>`.
- Keep shared components, hooks, stores, and `lib` independent of features.
  A feature imports another feature only through the `@/features/…`
  contracts in [contract §9.2](../architecture/contract.md#92-cross-feature-contracts):
  `player/store`, `player/types`, `player/dock`, `player/settings-api`, and
  `library/components/track-actions`. Shared code imports only the player
  contracts and the shell slots listed there. ESLint enforces both lists.
- Code two features need moves to shared code rather than a new deep
  import: generic UI to `web/src/components` (`ResponsiveDialog`,
  `ConfirmDialog`, `FormField`, `PasswordInput`, `Segmented`), hooks to
  `web/src/hooks` (`useLibraries`), and helpers to `web/src/lib`
  (`admin-queries`, `batch`, `rename-tokens`, `query-keys`). A new contract
  is added to contract §9.2 and the allow-list in `web/eslint.config.js`
  together.
- Keep files under 600 lines of code (ESLint warns; 800 is an error). Every
  `eslint-disable` comment states its reason after `--`, and unused
  directives are errors.
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

Text fields that act while the user types (search, filters, pickers) pass
their value through `useImeValue` (`web/src/hooks/use-ime-value.ts`), so an
IME reading such as pinyin or kana stays in the field and only committed text
reaches queries, URLs, and filters.

Every visible string uses i18next and both `en` and `zh` translations.
Tags, lyrics, names, and filenames remain untrusted text rendered through
React. Permission guards in the backend remain authoritative.

## Validation

Use `make ci-style` for lint and `make ci-frontend` for typecheck, helper
tests, and the production build. After changing the lockfile, also run
`make frontend-audit`. For interaction changes, run the
Docker development stack and check the affected browser workflow as described
in [Testing](testing.md#browser-checks). Browser checks are manual; do not
claim a manual check covers every browser.
