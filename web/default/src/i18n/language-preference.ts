import { normalizeInterfaceLanguage } from './languages'

const EXPLICIT_LANGUAGE_STORAGE_KEY = 'clove.explicit-interface-language'

export function markExplicitInterfaceLanguage(language: string): void {
  if (typeof window === 'undefined') return
  try {
    window.localStorage.setItem(
      EXPLICIT_LANGUAGE_STORAGE_KEY,
      normalizeInterfaceLanguage(language)
    )
  } catch {
    // Storage can be unavailable in private browsing; i18next still changes in memory.
  }
}

export function getExplicitInterfaceLanguage(): string | undefined {
  if (typeof window === 'undefined') return undefined
  try {
    const stored = window.localStorage.getItem(EXPLICIT_LANGUAGE_STORAGE_KEY)
    return stored ? normalizeInterfaceLanguage(stored) : undefined
  } catch {
    return undefined
  }
}
