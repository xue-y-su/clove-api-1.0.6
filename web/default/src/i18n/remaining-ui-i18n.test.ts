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

describe('remaining production UI translations', () => {
  test('translates model and channel actions', () => {
    const missingModels = readSource(
      '../features/models/components/dialogs/missing-models-dialog.tsx'
    )
    const fetchModels = readSource(
      '../features/channels/components/dialogs/fetch-models-dialog.tsx'
    )
    const channelsColumns = readSource(
      '../features/channels/components/channels-columns.tsx'
    )
    const codexUsage = readSource(
      '../features/channels/components/dialogs/codex-usage-dialog.tsx'
    )

    expect(missingModels).toContain("{t('Configure')}")
    expect(fetchModels).toContain("t('{{selected}} / {{total}} selected'")
    expect(channelsColumns).toContain("{t('Tag')}：{tag}")
    expect(channelsColumns).toContain("t('{{count}} channels'")
    expect(codexUsage).toContain("{t('Metered feature')}")
  })

  test('translates system, log, wallet, and uptime controls', () => {
    const systemInfo = readSource('../features/system-info/index.tsx')
    const viewLogs = readSource(
      '../features/models/components/dialogs/view-logs-dialog.tsx'
    )
    const recharge = readSource(
      '../features/wallet/components/recharge-form-card.tsx'
    )
    const uptime = readSource(
      '../features/pricing/components/model-details-uptime-sparkline.tsx'
    )

    expect(systemInfo).toContain("{t('Root')}")
    expect(viewLogs).toContain(
      "<SelectItem value='all'>{t('All')}</SelectItem>"
    )
    expect(recharge).toContain("t('Pay {{amount}}'")
    expect(recharge).toContain("t('Save {{amount}}'")
    expect(recharge).toContain("t('Minimum {{amount}}'")
    expect(uptime).toContain("t('30 day uptime {{percent}}%'")
    expect(uptime).toContain("t('{{count}} min outage'")
  })

  test('uses English source keys for sign-up translations', () => {
    const signUp = readSource(
      '../features/auth/sign-up/components/sign-up-form.tsx'
    )

    expect(signUp).not.toContain("t('请输入邀请码')")
    expect(signUp).not.toContain("t('邀请码（选填）')")
    expect(signUp).not.toContain("t('微信登录')")
    expect(signUp).not.toContain(
      "t('扫描二维码关注官方账号，回复“验证码”以接收您的验证码。')"
    )
  })

  test('translates user redemption kinds, statuses, and refunds', () => {
    const table = readSource(
      '../features/user-redemptions/components/user-redemptions-table.tsx'
    )
    const page = readSource('../features/user-redemptions/index.tsx')
    const governance = readSource(
      '../features/user-redemptions/components/admin-governance-dialog.tsx'
    )

    expect(table).toContain('USER_REDEMPTION_KIND_LABELS[item.kind]')
    expect(table).toContain('USER_REDEMPTION_STATUS_LABELS[item.status]')
    expect(table).toContain("t('Refund')")
    expect(page).toContain("label: 'Pending settlement'")
    expect(page).toContain("label: 'Forfeited'")
    expect(page).not.toContain('{t(status)}')
    expect(governance).toContain("translate('Disable')")
    expect(governance).toContain("translate('Terminate')")
    expect(governance).not.toContain("translate('Disable redemption code')")
  })
})
