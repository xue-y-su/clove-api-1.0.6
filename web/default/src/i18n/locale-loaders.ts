/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
type LocaleResources = Record<string, string>
type LocaleLoader = () => Promise<LocaleResources>

export const localeLoaders = {
  en: () =>
    import('./locales/en.json').then(
      (module) => module.default.translation as LocaleResources
    ),
  fr: () =>
    import('./locales/fr.json').then(
      (module) => module.default.translation as LocaleResources
    ),
  ja: () =>
    import('./locales/ja.json').then(
      (module) => module.default.translation as LocaleResources
    ),
  ru: () =>
    import('./locales/ru.json').then(
      (module) => module.default.translation as LocaleResources
    ),
  vi: () =>
    import('./locales/vi.json').then(
      (module) => module.default.translation as LocaleResources
    ),
  zhCN: () =>
    import('./locales/zh.json').then(
      (module) => module.default.translation as LocaleResources
    ),
  zhTW: () =>
    import('./locales/zh-TW.json').then(
      (module) => module.default.translation as LocaleResources
    ),
} satisfies Record<string, LocaleLoader>

const resourcePromises = new Map<string, Promise<LocaleResources>>()

export function loadLocaleResources(
  language: string
): Promise<LocaleResources> | undefined {
  if (!Object.hasOwn(localeLoaders, language)) return undefined

  const cached = resourcePromises.get(language)
  if (cached) return cached

  const loader = localeLoaders[language as keyof typeof localeLoaders]
  const promise = loader().catch((error: unknown) => {
    resourcePromises.delete(language)
    throw error
  })
  resourcePromises.set(language, promise)
  return promise
}
