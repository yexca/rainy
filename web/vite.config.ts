import { mkdirSync, writeFileSync } from 'node:fs'
import path from 'node:path'

import tailwindcss from '@tailwindcss/vite'
import react from '@vitejs/plugin-react'
import { defineConfig, type Plugin } from 'vite'
import { VitePWA } from 'vite-plugin-pwa'

import { pwaOptions } from './pwa.config.ts'

const backend = process.env.RAINY_BACKEND ?? 'http://localhost:7650'
const outDir = path.resolve(import.meta.dirname, 'dist')

/**
 * `emptyOutDir` wipes `dist/`, but the Go side embeds `web/dist` and needs the directory
 * (with its tracked `.gitkeep`) to exist even before the first frontend build.
 * Re-create the placeholder once the bundle has been written.
 */
function keepDistPlaceholder(): Plugin {
  return {
    name: 'rainy:keep-dist-placeholder',
    apply: 'build',
    closeBundle() {
      mkdirSync(outDir, { recursive: true })
      writeFileSync(path.join(outDir, '.gitkeep'), '')
    },
  }
}

// https://vite.dev/config/
export default defineConfig({
  plugins: [react(), tailwindcss(), VitePWA(pwaOptions), keepDistPlaceholder()],
  resolve: {
    alias: {
      '@': path.resolve(import.meta.dirname, 'src'),
    },
  },
  server: {
    port: 5173,
    proxy: {
      '/api': { target: backend, changeOrigin: true, ws: false },
      '/rest': { target: backend, changeOrigin: true, ws: false },
    },
  },
  preview: {
    port: 4173,
    proxy: {
      '/api': { target: backend, changeOrigin: true, ws: false },
      '/rest': { target: backend, changeOrigin: true, ws: false },
    },
  },
  build: {
    outDir,
    emptyOutDir: true,
    chunkSizeWarningLimit: 800,
  },
})
