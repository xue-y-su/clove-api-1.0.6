/*
Copyright (C) 2025 QuantumNous

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
import { expect, test } from 'bun:test';
import { existsSync, readFileSync } from 'node:fs';

const source = readFileSync(
  new URL('./UserInvitationCard.jsx', import.meta.url),
  'utf8',
);
const qrModalSource = readFileSync(
  new URL('./InvitationQRModal.jsx', import.meta.url),
  'utf8',
);
const apiSource = readFileSync(new URL('./api.js', import.meta.url), 'utf8');
const adminSource = readFileSync(
  new URL('./AdminInvitations.jsx', import.meta.url),
  'utf8',
);
const rewardAdminSource = readFileSync(
  new URL('./AdminInvitationRewards.jsx', import.meta.url),
  'utf8',
);
const topupSource = readFileSync(
  new URL('../topup/index.jsx', import.meta.url),
  'utf8',
);
const registerSource = readFileSync(
  new URL('../auth/RegisterForm.jsx', import.meta.url),
  'utf8',
);
const operationSource = readFileSync(
  new URL('../settings/OperationSetting.jsx', import.meta.url),
  'utf8',
);
const creditLimitSource = readFileSync(
  new URL(
    '../../pages/Setting/Operation/SettingsCreditLimit.jsx',
    import.meta.url,
  ),
  'utf8',
);
const incentiveSettingsPath = new URL(
  '../../pages/Setting/Operation/SettingsInvitationIncentive.jsx',
  import.meta.url,
);
const incentiveSettingsSource = existsSync(incentiveSettingsPath)
  ? readFileSync(incentiveSettingsPath, 'utf8')
  : '';
const zhCNLocale = JSON.parse(
  readFileSync(
    new URL('../../i18n/locales/zh-CN.json', import.meta.url),
    'utf8',
  ),
);

test('exposes separate copy actions for the code and registration link', () => {
  expect(source).toContain('复制邀请码');
  expect(source).toContain('复制注册链接');
  expect(source).toContain('registrationUrl');
  expect(source).toContain('revokeOwnInvitationCode');
  expect(apiSource).toContain('/api/invitation/codes/');
});

test('validates prefilled invitation codes without racing stale requests', () => {
  expect(registerSource).toContain('validateInvitationCode');
  expect(registerSource).toContain('window.setTimeout');
  expect(registerSource).toContain('AbortController');
  expect(registerSource).toContain("status === 'valid'");
  expect(registerSource).toContain("status === 'invalid'");
  expect(registerSource).toContain("aria-live='polite'");
});

test('uses a status label that does not duplicate the translated state prefix', () => {
  expect(source).toContain("t('Invitation code status: {{status}}'");
  expect(zhCNLocale.translation['Invitation code status: {{status}}']).toBe(
    '邀请码状态：{{status}}',
  );

  const interpolate = (template, status) =>
    template.replace('{{status}}', status);
  expect(
    interpolate(zhCNLocale.translation['邀请码已{{status}}'], '已撤销'),
  ).toBe('邀请码状态：已撤销');
});

test('collects application and revoke reasons only in modals', () => {
  expect(source).toContain('InvitationApplicationModal');
  expect(source).toContain('InvitationRevokeModal');
  expect(source).not.toContain('<TextArea');
  expect(source).not.toContain("t('Cancelled by code owner')");
});

test('keeps the invitation card and reward metrics content-sized', () => {
  expect(source).toContain("className='!self-start");
  expect(source).toContain('max-w-2xl');
  expect(topupSource).toContain('grid grid-cols-1 gap-6');
});

test('shows frozen invitation rewards without changing quota formatting', () => {
  expect(source).toContain("label={t('Frozen')}");
  expect(source).toContain('renderQuota(rewardSummary?.frozen || 0)');
});

test('renders invitation code kinds as user-facing labels', () => {
  expect(source).toContain("invitation.code.kind === 'multi'");
  expect(zhCNLocale.translation['Single use']).toBe('一次性');
  expect(zhCNLocale.translation['Multiple use']).toBe('多次使用');
});

test('refreshes invitation state when the reapply cooldown ends', () => {
  expect(source).toContain('getInvitationReapplyRefreshDelay');
  expect(source).toContain('window.setTimeout');
  expect(source).toContain('loadInvitation()');
});

test('translates the owner revoke confirmation title in Chinese', () => {
  expect(source).toContain("t('Revoke invitation code?')");
  expect(zhCNLocale.translation['Revoke invitation code?']).toBe(
    '撤销邀请码？',
  );
});

test('translates the invitation application dialog in Chinese', () => {
  expect(zhCNLocale.translation['Apply for invitation code']).toBe(
    '申请邀请码',
  );
  expect(zhCNLocale.translation['Application reason']).toBe('申请原因');
  expect(
    zhCNLocale.translation['Describe who you plan to invite and why.'],
  ).toBe('请说明您计划邀请谁以及申请原因。');
  expect(zhCNLocale.translation['Submit application']).toBe('提交申请');
});

test('uses translated labels for every administrator invitation status', () => {
  const labels = {
    Active: '生效',
    Exhausted: '已用尽',
    Expired: '已过期',
    Revoked: '已撤销',
    Pending: '待处理',
    'Pending review': '待审核',
    Approved: '已批准',
    Rejected: '已拒绝',
    Withdrawn: '已撤回',
    Completed: '已完成',
    Skipped: '已跳过',
    Failed: '失败',
  };

  expect(adminSource).toContain('getInvitationStatusTranslationKey');
  expect(source).toContain('getInvitationStatusTranslationKey');
  for (const [key, translation] of Object.entries(labels)) {
    expect(zhCNLocale.translation[key]).toBe(translation);
  }
});

test('translates the wallet redemption entry and user revoke action in Chinese', () => {
  expect(zhCNLocale.translation['Create Redemption Code']).toBe('创建兑换码');
  expect(zhCNLocale.translation['Revoke invitation code']).toBe('撤销邀请码');
  expect(
    zhCNLocale.translation[
      'This code will stop accepting new registrations. Existing registrations remain unchanged.'
    ],
  ).toBe('此邀请码将停止接受新的注册，已有注册不受影响。');
  expect(zhCNLocale.translation.Reason).toBe('原因');
  expect(zhCNLocale.translation['Enter a reason for this action']).toBe(
    '请输入本次操作的原因',
  );
});

test('submits aggregate invitation reward requests for administrator approval', () => {
  expect(apiSource).toContain('/api/invitation/rewards');
  expect(apiSource).toContain('/api/invitation/rewards/payout-requests');
  expect(apiSource).toContain('/api/invitation/admin/reward-payout-requests');
  expect(source).toContain('getInvitationRewardSummary');
  expect(source).toContain('submitInvitationRewardPayout');
  expect(source).toContain('提交提现审核');
  expect(rewardAdminSource).toContain('listInvitationRewardPayoutRequests');
  expect(rewardAdminSource).toContain('reviewInvitationRewardPayoutRequest');
  expect(rewardAdminSource).toContain('Invitation reward approvals');
});

test('keeps reward history, details, freezes and mobile cards available', () => {
  expect(apiSource).toContain('getInvitationRewardPayoutRequestDetail');
  expect(apiSource).toContain('freezeInvitationRewardPayoutRequest');
  expect(apiSource).toContain('unfreezeInvitationRewardPayoutRequest');
  expect(rewardAdminSource).toContain("'frozen'");
  expect(rewardAdminSource).toContain("itemKey: 'pending'");
  expect(rewardAdminSource).toContain("itemKey: 'history'");
  expect(rewardAdminSource).toContain("t('Approval history')");
  expect(rewardAdminSource).toContain("className='space-y-3 lg:hidden'");
  expect(rewardAdminSource).toContain("className='hidden lg:block'");
});

test('formats reward applicant identities through one translated template', () => {
  expect(rewardAdminSource).not.toContain('@{item.username} · #{item.user_id}');
  expect(
    rewardAdminSource.match(/t\('@\{\{username\}\} · #\{\{userId\}\}'/g),
  ).toHaveLength(2);
});

test('provides one dedicated Classic invitation incentive settings card', () => {
  expect(operationSource).toContain('SettingsInvitationIncentive');
  expect(incentiveSettingsSource).toContain('getInvitationIncentiveSettings');
  expect(incentiveSettingsSource).toContain(
    'updateInvitationIncentiveSettings',
  );
  expect(incentiveSettingsSource).toContain('cash_commission_rate');
  expect(incentiveSettingsSource).toContain('step={0.01}');
  expect(incentiveSettingsSource).not.toMatch(
    /cash_commission_rate[\s\S]{0,500}max=\{?100/,
  );
  expect(incentiveSettingsSource).toContain('const childrenDisabled =');
  expect(apiSource).toContain('/api/invitation/incentive-settings');
});

test('moves Classic registration reward amounts out of generic quota settings', () => {
  expect(creditLimitSource).not.toContain('QuotaForInviter');
  expect(creditLimitSource).not.toContain('QuotaForInvitee');
  expect(incentiveSettingsSource).toContain('QuotaForInviter');
  expect(incentiveSettingsSource).toContain('QuotaForInvitee');
});

test('shows the active Classic cash cashback rate without disabled noise', () => {
  expect(source).toContain('rewardSummary?.cash_commission_visible');
  expect(source).toContain('rewardSummary?.cash_commission_rate');
  expect(source).toContain("t('Top-up cashback {{rate}}%'");
  expect(source).not.toContain('充值返现未开启');
});

test('shows complete commission and reversal evidence in Classic details', () => {
  for (const field of [
    'eligible_quota',
    'rate_basis_points',
    'reversed_amount',
    'net_amount',
    'trigger_type',
    'trigger_id',
    'reversal_state',
  ]) {
    expect(rewardAdminSource).toContain(field);
  }
  expect(rewardAdminSource).toContain('renderQuota(reward.amount)');
  expect(rewardAdminSource).toContain("if (value === 'cancelled')");
  expect(rewardAdminSource).toContain('copy(reward.trigger_id)');
});

test('translates Classic commission evidence labels in Chinese', () => {
  const labels = {
    'Save invitation incentive settings': '保存邀请激励设置',
    'Copy trigger ID': '复制触发 ID',
    'Cash top-up': '现金充值',
    'Cash subscription purchase': '现金订阅购买',
    'Administrator balance code': '管理员余额兑换码',
    'Administrator subscription code': '管理员订阅兑换码',
    'Not reversed': '未冲正',
    'Partially reversed': '部分冲正',
    'Fully reversed': '已全部冲正',
  };

  for (const [key, value] of Object.entries(labels)) {
    expect(zhCNLocale.translation[key], key).toBe(value);
  }
});

test('renders and downloads a full invitation poster with Classic controls', () => {
  expect(source).toContain('useContext(UserContext)');
  expect(source).toContain('useContext(StatusContext)');
  expect(source).toContain('code={invitation?.code?.code ||');
  expect(source).toContain('displayName={displayName}');
  expect(qrModalSource).toContain('renderInvitationPoster');
  expect(qrModalSource).toContain('resolveInvitationPosterScene');
  expect(qrModalSource).toContain(
    `logoUrl: resolveInvitationPosterLogoUrl(\n        logo,\n        typeof window === 'undefined' ? '' : window.location.origin,\n      )`,
  );
  expect(qrModalSource).not.toContain('resolveInvitationPosterLogoFallbackUrl');
  expect(qrModalSource).toContain('const controller = new AbortController();');
  expect(qrModalSource).toContain('signal: controller.signal,');
  expect(qrModalSource).toContain('controller.abort();');
  expect(qrModalSource).toContain('canvasToPngBlob');
  expect(qrModalSource).toContain('createInvitationPosterFilename');
  expect(qrModalSource).toContain('saveInvitationPosterState');
  expect(qrModalSource).toContain('<InvitationPosterEditor');
  expect(qrModalSource).toContain('size={360}');
  expect(qrModalSource).toContain("level='H'");
  expect(qrModalSource).toContain('marginSize={4}');
  expect(qrModalSource).not.toContain('toDataURL');
});

test('opens Classic posters in preview and reveals editing from its pencil', () => {
  expect(qrModalSource).toContain(
    "const [mode, setMode] = useState('preview')",
  );
  expect(qrModalSource).toContain("mode === 'preview'");
  expect(qrModalSource).toContain("aria-label={t('Edit poster')}");
  expect(qrModalSource).toContain('<Pencil');
  expect(qrModalSource).toContain('setDraft(structuredClone(activeDesign))');
});

test('reruns Classic poster rendering after the modal mounts the QR canvas', () => {
  expect(qrModalSource).toContain('const [qrCanvas, setQrCanvas] = useState');
  expect(qrModalSource).toContain('ref={setQrCanvas}');
  expect(qrModalSource).toContain('!qrCanvas) return');
  expect(qrModalSource).toContain('qrCanvas,');
  expect(qrModalSource).not.toContain('qrCanvasRef');
});

test('translates Classic invitation poster controls in every locale', async () => {
  const keys = [
    'Invitation poster',
    'Invitation poster preview',
    'Poster size',
    'Show my nickname',
    'Invitation poster title',
    'Invitation poster subtitle',
    'Shown on invitation posters. Use {站点名称} to insert the current site name.',
    'Leave blank to use the localized default copy.',
  ];
  for (const localeFile of [
    'en.json',
    'fr.json',
    'ja.json',
    'ru.json',
    'vi.json',
    'zh-CN.json',
    'zh-TW.json',
    'zh.json',
  ]) {
    const locale = await Bun.file(
      new URL(`../../i18n/locales/${localeFile}`, import.meta.url),
    ).json();
    for (const key of keys) {
      expect(locale.translation[key]).toBeTruthy();
      if (localeFile !== 'en.json') {
        expect(locale.translation[key]).not.toBe(key);
      }
    }
  }
});
