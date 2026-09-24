/**
 * i18next setup (docs/architecture/contract.md §9.6).
 *
 * Namespaces = files in `src/locales/<lng>/<ns>.json`; every namespace is bundled statically.
 * Language detection: saved choice (localStorage `rainy.lang`) → `navigator.language`
 * (`zh*` → `zh`) → `en`. Use `setLanguage()` to switch and persist.
 */
import i18n from 'i18next'
import LanguageDetector from 'i18next-browser-languagedetector'
import { initReactI18next } from 'react-i18next'

import enAdmin from '@/locales/en/admin.json'
import enAuth from '@/locales/en/auth.json'
import enCommon from '@/locales/en/common.json'
import enLibrary from '@/locales/en/library.json'
import enManage from '@/locales/en/manage.json'
import enPlayer from '@/locales/en/player.json'
import enSettings from '@/locales/en/settings.json'
import zhAdmin from '@/locales/zh/admin.json'
import zhAuth from '@/locales/zh/auth.json'
import zhCommon from '@/locales/zh/common.json'
import zhLibrary from '@/locales/zh/library.json'
import zhManage from '@/locales/zh/manage.json'
import zhPlayer from '@/locales/zh/player.json'
import zhSettings from '@/locales/zh/settings.json'

export const NAMESPACES = ['common', 'auth', 'library', 'player', 'settings', 'manage', 'admin'] as const
export type Namespace = (typeof NAMESPACES)[number]

export const LANGUAGES = [
  { code: 'zh', label: '简体中文', htmlLang: 'zh-CN' },
  { code: 'en', label: 'English', htmlLang: 'en' },
] as const
export type Language = (typeof LANGUAGES)[number]['code']

export const LANG_STORAGE_KEY = 'rainy.lang'
const SUPPORTED: readonly string[] = LANGUAGES.map((l) => l.code)

export const resources = {
  en: {
    common: enCommon,
    auth: enAuth,
    library: enLibrary,
    player: enPlayer,
    settings: enSettings,
    manage: enManage,
    admin: enAdmin,
  },
  zh: {
    common: zhCommon,
    auth: zhAuth,
    library: zhLibrary,
    player: zhPlayer,
    settings: zhSettings,
    manage: zhManage,
    admin: zhAdmin,
  },
} as const

function syncDocumentLanguage(lng: string): void {
  if (typeof document === 'undefined') return
  const match = LANGUAGES.find((l) => lng === l.code || lng.startsWith(`${l.code}-`))
  document.documentElement.lang = match?.htmlLang ?? 'en'
}

i18n.on('languageChanged', syncDocumentLanguage)

void i18n
  .use(LanguageDetector)
  .use(initReactI18next)
  .init({
    resources,
    ns: [...NAMESPACES],
    defaultNS: 'common',
    fallbackNS: 'common',
    fallbackLng: 'en',
    supportedLngs: [...SUPPORTED],
    nonExplicitSupportedLngs: true,
    load: 'languageOnly',
    interpolation: { escapeValue: false }, // React escapes
    returnNull: false,
    detection: {
      order: ['localStorage', 'navigator'],
      lookupLocalStorage: LANG_STORAGE_KEY,
      // Only an explicit choice (setLanguage) is persisted, so the browser language keeps
      // being followed until the user picks one.
      caches: [],
    },
    react: { useSuspense: false },
  })

/** The active UI language (`zh` | `en`). */
export function currentLanguage(): Language {
  const lng = i18n.resolvedLanguage ?? i18n.language ?? 'en'
  return lng.startsWith('zh') ? 'zh' : 'en'
}

/** BCP-47 tag for `Intl` formatters (`zh-CN` / `en`). */
export function currentLocale(): string {
  return currentLanguage() === 'zh' ? 'zh-CN' : 'en'
}

/** Switch language and remember the choice. */
export async function setLanguage(lng: Language): Promise<void> {
  try {
    localStorage.setItem(LANG_STORAGE_KEY, lng)
  } catch {
    // storage unavailable (private mode) — the change still applies to this session
  }
  await i18n.changeLanguage(lng)
}

export default i18n
