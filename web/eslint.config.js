import { readdirSync } from 'node:fs'

import js from '@eslint/js'
import { defineConfig, globalIgnores } from 'eslint/config'
import { builtinRules } from 'eslint/use-at-your-own-risk'
import reactHooks from 'eslint-plugin-react-hooks'
import reactRefresh from 'eslint-plugin-react-refresh'
import globals from 'globals'
import tseslint from 'typescript-eslint'

// Feature boundaries (docs/development/frontend-guidelines.md, docs/architecture/contract.md §9.2).
// A feature imports another feature only through these documented contracts; anything else two
// features share lives in src/components, src/hooks, or src/lib.
const FEATURE_CONTRACTS = [
  '@/features/player/store',
  '@/features/player/types',
  '@/features/player/dock',
  '@/features/player/settings-api',
  '@/features/library/components/track-actions',
]
// Shared code (components, hooks, lib, stores, layouts) imports features only for the player
// contracts and the shell slots the app shell mounts. src/router.tsx composes pages and is exempt.
const SHELL_CONTRACTS = [
  '@/features/player/store',
  '@/features/player/types',
  '@/features/player/dock',
  '@/features/player/components/audio-engine',
  '@/features/player/components/mascot-companion',
  '@/features/player/components/mini-player',
  '@/features/player/components/now-playing-sheet',
  '@/features/player/components/player-dock',
  '@/features/library/components/add-to-playlist-host',
  '@/features/library/components/immersive-theme',
  '@/features/manage/tag-editor-host',
  '@/features/settings/components/appearance-section',
]
const FEATURES = readdirSync(new URL('./src/features', import.meta.url), { withFileTypes: true })
  .filter((entry) => entry.isDirectory())
  .map((entry) => entry.name)

const escape = (s) => s.replace(/[.*+?^${}()|[\]\\/]/g, '\\$&')
const anyOf = (list) => `(?:${list.map(escape).join('|')})`

const featureBoundary = (feature) => {
  const others = FEATURES.filter((f) => f !== feature)
  const message =
    `The ${feature} feature must not import another feature's internals. Import one of the contracts ` +
    `in docs/architecture/contract.md §9.2 (${FEATURE_CONTRACTS.join(', ')}), or move code both ` +
    'features need to @/components, @/hooks, or @/lib and import it from there.'
  return {
    files: [`src/features/${feature}/**/*.{ts,tsx}`],
    rules: {
      'no-restricted-imports': [
        'error',
        {
          patterns: [
            {
              regex: `^@/features/(?!${escape(feature)}(?:/|$))(?!${anyOf(FEATURE_CONTRACTS.map((p) => p.slice('@/features/'.length)))}$)`,
              message,
            },
            others.length > 0 && {
              regex: `^(?:\\.\\./)+(?:features/)?${anyOf(others)}(?:/|$)`,
              message: `Import other features through the @/features/… contracts, not relative paths. ${message}`,
            },
          ].filter(Boolean),
        },
      ],
    },
  }
}

const shellBoundary = {
  files: ['src/{components,hooks,lib,stores,layouts}/**/*.{ts,tsx}'],
  rules: {
    'no-restricted-imports': [
      'error',
      {
        patterns: [
          {
            regex: `^@/features/(?!${anyOf(SHELL_CONTRACTS.map((p) => p.slice('@/features/'.length)))}$)`,
            message:
              'Shared code must stay independent of features. Import only the player contracts and shell ' +
              `slots in docs/architecture/contract.md §9.2 (${SHELL_CONTRACTS.join(', ')}); move anything ` +
              'else the shell needs into @/components, @/hooks, or @/lib.',
          },
          {
            regex: '^(?:\\.\\./)+features/',
            message: 'Import features through the @/features/… contracts in docs/architecture/contract.md §9.2, not relative paths.',
          },
        ],
      },
    ],
  },
}

// Every `eslint-disable` comment says why: `// eslint-disable-next-line <rule> -- <reason>`.
const requireDisableReason = {
  meta: {
    type: 'suggestion',
    schema: [],
    messages: {
      missing:
        'Add a reason after `--`: `eslint-disable-next-line <rule> -- <why the rule does not apply here>`, or fix the code instead.',
    },
  },
  create(context) {
    return {
      Program() {
        for (const comment of context.sourceCode.getAllComments()) {
          const text = comment.value.trim()
          if (!/^eslint-disable(?:-next-line|-line)?(?:\s|$)/.test(text)) continue
          const [, reason] = text.split(/\s-{2,}\s/)
          if (!reason?.trim()) context.report({ loc: comment.loc, messageId: 'missing' })
        }
      },
    }
  },
}

const rainy = {
  rules: {
    'require-disable-reason': requireDisableReason,
    // The core max-lines rule under a second name, so one file gets a warning and a hard limit.
    'max-lines-hard': builtinRules.get('max-lines'),
  },
}

const MAX_LINES = { skipBlankLines: true, skipComments: true }

export default defineConfig([
  globalIgnores(['dist', 'dev-dist', 'node_modules']),
  {
    linterOptions: {
      reportUnusedDisableDirectives: 'error',
    },
  },
  {
    files: ['**/*.{ts,tsx,js}'],
    plugins: { rainy },
    rules: {
      'rainy/require-disable-reason': 'error',
    },
  },
  {
    files: ['**/*.{ts,tsx}'],
    extends: [
      js.configs.recommended,
      tseslint.configs.recommended,
      reactHooks.configs.flat['recommended-latest'],
      reactRefresh.configs.vite,
    ],
    languageOptions: {
      ecmaVersion: 2023,
      globals: globals.browser,
    },
    rules: {
      '@typescript-eslint/no-unused-vars': [
        'error',
        { argsIgnorePattern: '^_', varsIgnorePattern: '^_', caughtErrors: 'none', ignoreRestSiblings: true },
      ],
      '@typescript-eslint/consistent-type-imports': ['error', { fixStyle: 'inline-type-imports' }],
    },
  },
  ...FEATURES.map(featureBoundary),
  shellBoundary,
  {
    // Split a file before it grows past 600 lines of code; 800 is a hard limit.
    files: ['src/**/*.{ts,tsx}'],
    ignores: ['src/components/ui/**'],
    rules: {
      'max-lines': ['warn', { max: 600, ...MAX_LINES }],
      'rainy/max-lines-hard': ['error', { max: 800, ...MAX_LINES }],
    },
  },
  {
    // Legacy exception: this page predates the limit. The number may only go down; split the page
    // (for example its dialogs and toolbars into ../components) the next time you change it.
    files: ['src/features/manage/pages/folders-page.tsx'],
    rules: {
      'rainy/max-lines-hard': ['error', { max: 834, ...MAX_LINES }],
    },
  },
  {
    // Generated shadcn/ui components export variants and hooks next to components by design.
    files: ['src/components/ui/**/*.{ts,tsx}'],
    rules: {
      'react-refresh/only-export-components': 'off',
    },
  },
  {
    // Build tooling and unit tests run in Node.
    files: ['*.config.ts', 'vite.config.ts', 'pwa.config.ts', 'pwa-assets.config.ts', 'tests/**/*.ts'],
    languageOptions: {
      globals: globals.node,
    },
  },
])
