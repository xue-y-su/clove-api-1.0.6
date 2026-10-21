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
import { describe, expect, test } from 'bun:test'
import { readFileSync } from 'node:fs'

function readSource(relativePath: string): string {
  return readFileSync(new URL(relativePath, import.meta.url), 'utf8')
}

describe('shared control translations', () => {
  test('translates theme option labels and accessible descriptions', () => {
    const source = readSource('../components/config-drawer.tsx')

    expect(source).toContain("t('Select {{label}}'")
    expect(source).toContain("t('{{label}} option preview'")
    expect(source).toContain("aria-label={t('Reset')}")
    expect(source).toContain('t(option.label)')
  })

  test('translates reusable component accessibility labels', () => {
    const conversation = readSource(
      '../components/ai-elements/conversation.tsx'
    )
    const dateTimePicker = readSource('../components/datetime-picker.tsx')
    const jsonEditor = readSource('../components/json-editor.tsx')
    const passwordInput = readSource('../components/password-input.tsx')
    const sonner = readSource('../components/ui/sonner.tsx')
    const tagInput = readSource('../components/tag-input.tsx')
    const pagination = readSource('../components/ui/pagination.tsx')
    const breadcrumb = readSource('../components/ui/breadcrumb.tsx')
    const spinner = readSource('../components/ui/spinner.tsx')
    const sidebar = readSource('../components/ui/sidebar.tsx')

    expect(conversation).toContain("aria-label={t('Scroll to bottom')}")
    expect(dateTimePicker).toContain("aria-label={t('Clear')}")
    expect(jsonEditor).toContain("aria-label={t('Delete row')}")
    expect(passwordInput).toContain(
      "aria-label={t('Toggle password visibility')}"
    )
    expect(sonner).toContain(
      "containerAriaLabel={containerAriaLabel ?? t('Notifications')}"
    )
    expect(sonner).toContain(
      "toastOptions?.closeButtonAriaLabel ?? t('Close toast')"
    )
    expect(tagInput).toContain("aria-label={t('Remove tag')}")
    expect(pagination).toContain("aria-label={t('Pagination')}")
    expect(pagination).toContain("t('Go to previous page')")
    expect(pagination).toContain("t('Go to next page')")
    expect(pagination).toContain("t('More pages')")
    expect(breadcrumb).toContain("aria-label={t('Breadcrumb')}")
    expect(breadcrumb).toContain("t('More')")
    expect(spinner).toContain("aria-label={t('Loading')}")
    expect(sidebar).toContain("<SheetTitle>{t('Sidebar')}</SheetTitle>")
    expect(sidebar).toContain("t('Displays the mobile sidebar.')")
  })

  test('translates common admin labels and selection controls', () => {
    const forgotPassword = readSource(
      '../features/auth/forgot-password/components/forgot-password-form.tsx'
    )
    const apiKeysColumns = readSource(
      '../features/keys/components/api-keys-columns.tsx'
    )
    const usersColumns = readSource(
      '../features/users/components/users-columns.tsx'
    )
    const channelSelector = readSource(
      '../features/system-settings/models/channel-selector-dialog.tsx'
    )
    const performance = readSource(
      '../features/system-settings/maintenance/performance-section.tsx'
    )
    const tieredPricing = readSource(
      '../features/system-settings/models/tiered-pricing-editor.tsx'
    )
    const subscriptions = readSource(
      '../features/subscriptions/components/subscriptions-mutate-drawer.tsx'
    )

    expect(forgotPassword).toContain("<FormLabel>{t('Email')}</FormLabel>")
    expect(apiKeysColumns).toContain("aria-label={t('Select all')}")
    expect(apiKeysColumns).toContain("aria-label={t('Select row')}")
    expect(usersColumns).toContain("aria-label={t('Select all')}")
    expect(usersColumns).toContain("aria-label={t('Select row')}")
    expect(channelSelector).toContain('label: t(option.label)')
    expect(channelSelector).toContain('{t(option.label)}')
    expect(performance).toContain("{t('Goroutines')}:")
    expect(tieredPricing).toContain("aria-label={t('Remove')}")
    expect(tieredPricing).toContain("placeholder={t('tokens')}")
    expect(subscriptions).toContain(
      "<FormLabel>{t('Stripe Price ID')}</FormLabel>"
    )
    expect(subscriptions).toContain(
      "<FormLabel>{t('Creem Product ID')}</FormLabel>"
    )
    expect(subscriptions).toContain("t('Waffo Pancake Product ID')")
  })
})
