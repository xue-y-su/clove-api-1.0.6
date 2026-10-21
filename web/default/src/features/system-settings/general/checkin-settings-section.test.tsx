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
import { expect, test } from 'bun:test'

const sectionSource = await Bun.file(
  new URL('./checkin-settings-section.tsx', import.meta.url)
).text()
const billingDefaultsSource = await Bun.file(
  new URL('../billing/index.tsx', import.meta.url)
).text()
const billingRegistrySource = await Bun.file(
  new URL('../billing/section-registry.tsx', import.meta.url)
).text()
const settingsTypesSource = await Bun.file(
  new URL('../types.ts', import.meta.url)
).text()
const profileTypesSource = await Bun.file(
  new URL('../../profile/types.ts', import.meta.url)
).text()
const profileCardSource = await Bun.file(
  new URL('../../profile/components/checkin-calendar-card.tsx', import.meta.url)
).text()

test('wires the independent daily expiration option through the billing form', () => {
  expect(sectionSource).toContain('expireUnusedQuota: z.boolean()')
  expect(sectionSource).toContain("key: 'checkin_setting.expire_unused_quota'")
  expect(sectionSource).toContain("name='expireUnusedQuota'")
  expect(sectionSource).toContain("t('Expire unused check-in rewards daily')")
  expect(billingDefaultsSource).toContain(
    "'checkin_setting.expire_unused_quota': false"
  )
  expect(billingRegistrySource).toContain(
    "settings['checkin_setting.expire_unused_quota']"
  )
  expect(settingsTypesSource).toContain(
    "'checkin_setting.expire_unused_quota': boolean"
  )
})

test('shows remaining expiring reward state only for today', () => {
  for (const field of [
    'quota_remaining: number',
    'expires_at: number',
    'reclaimed_quota: number',
  ]) {
    expect(profileTypesSource).toContain(field)
  }
  expect(profileCardSource).toContain('record.checkin_date === todayString')
  expect(profileCardSource).toContain('todayRecord.expires_at > 0')
  expect(profileCardSource).toContain('todayRecord.quota_remaining > 0')
  expect(profileCardSource).toContain(
    "t('Unused check-in reward expires at midnight')"
  )
})
