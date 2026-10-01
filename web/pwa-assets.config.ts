import { defineConfig, minimal2023Preset, type Preset } from '@vite-pwa/assets-generator/config'

/**
 * PWA icons, rendered into `public/` by the asset generator in two passes:
 *
 *   pnpm generate-pwa-assets                                  # favicon.ico + pwa-64/192/512 ("any")
 *   RAINY_ICON_SET=adaptive pnpm generate-pwa-assets          # maskable-icon-512 + apple-touch-icon-180
 *
 * The icon is the mascot (docs/development/design.md#mascot), drawn as raster art:
 * - "any" icons come from `public/icon-source.png`: the 512 px rounded app tile with transparent
 *   corners (browser tabs, desktop installs, launchers that show icons unmasked).
 * - Maskable (Android adaptive icons) and Apple touch icons come from
 *   `public/icon-source-maskable.png`: the same art full-bleed — the platform applies its own
 *   mask — with the head inside the 80 % safe zone.
 * Both sources are excluded from the service-worker precache (`pwa.config.ts`).
 */
const adaptive = process.env.RAINY_ICON_SET === 'adaptive'

const anyIcons: Preset = {
  ...minimal2023Preset,
  transparent: { ...minimal2023Preset.transparent, sizes: [64, 192, 512], favicons: [[48, 'favicon.ico']], padding: 0.04 },
  maskable: { ...minimal2023Preset.maskable, sizes: [] },
  apple: { ...minimal2023Preset.apple, sizes: [] },
}

const adaptiveIcons: Preset = {
  ...minimal2023Preset,
  transparent: { ...minimal2023Preset.transparent, sizes: [], favicons: [] },
  maskable: { ...minimal2023Preset.maskable, sizes: [512], padding: 0 },
  apple: { ...minimal2023Preset.apple, sizes: [180], padding: 0 },
}

export default defineConfig({
  headLinkOptions: { preset: '2023' },
  preset: adaptive ? adaptiveIcons : anyIcons,
  images: [adaptive ? 'public/icon-source-maskable.png' : 'public/icon-source.png'],
})
