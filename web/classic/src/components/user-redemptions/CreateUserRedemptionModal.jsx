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
import React, { useEffect, useMemo, useState } from 'react';
import {
  Banner,
  Button,
  ButtonGroup,
  Form,
  Input,
  InputNumber,
  Modal,
  Select,
  Typography,
} from '@douyinfe/semi-ui';
import { Copy, Gift } from 'lucide-react';
import { useTranslation } from 'react-i18next';

import { copy, renderQuota, showError, showSuccess } from '../../helpers';
import {
  displayAmountToQuota,
  getQuotaPerUnit,
  quotaToDisplayAmount,
} from '../../helpers/quota';
import { formatSubscriptionPrice } from '../../helpers/subscriptionFormat';
import { useIsMobile } from '../../hooks/common/useIsMobile';
import { createUserRedemption } from './api.js';
import {
  buildCreateUserRedemptionRequest,
  calculateSubscriptionRedemptionCost,
  getMinimumRedemptionAmount,
  getRedemptionAmountValidation,
  getUserRedemptionExpiryOptions,
  getUserRedemptionExpiryTranslation,
  normalizeRedemptionAmount,
  sanitizeRedemptionAmountInput,
} from './user-redemption-state.js';

export default function CreateUserRedemptionModal({
  visible,
  onClose,
  onCreated,
  singleEnabled = true,
  randomEnabled = true,
  subscriptionEnabled = true,
  subscriptionPlans = [],
  maxClaimsPerCode = 100,
  maxExpiryHours = 720,
}) {
  const { t } = useTranslation();
  const isMobile = useIsMobile();
  const quotaPerUnit = getQuotaPerUnit();
  const minimumUnitAmount = quotaToDisplayAmount(quotaPerUnit);
  const balanceEnabled = singleEnabled || randomEnabled;
  const defaultRewardType = balanceEnabled ? 'balance' : 'subscription';
  const defaultKind = singleEnabled ? 'single' : 'random';
  const expiryOptions = useMemo(
    () => getUserRedemptionExpiryOptions(maxExpiryHours),
    [maxExpiryHours],
  );
  const eligiblePlans = useMemo(
    () =>
      subscriptionPlans.filter(
        ({ plan }) => plan?.enabled && plan?.allow_balance_pay !== false,
      ),
    [subscriptionPlans],
  );
  const [busy, setBusy] = useState(false);
  const [rewardType, setRewardType] = useState(defaultRewardType);
  const [kind, setKind] = useState(defaultKind);
  const [amount, setAmount] = useState(() => minimumUnitAmount);
  const [maxClaims, setMaxClaims] = useState(2);
  const [planId, setPlanId] = useState(0);
  const [units, setUnits] = useState(1);
  const [hours, setHours] = useState(24);
  const [createdCode, setCreatedCode] = useState(null);
  const selectedPlan = eligiblePlans.find(
    ({ plan }) => plan?.id === planId,
  )?.plan;
  const subscriptionCost = calculateSubscriptionRedemptionCost(
    Number(selectedPlan?.price_amount || 0),
    units,
    quotaPerUnit,
  );
  const minimumAmount = getMinimumRedemptionAmount(
    kind,
    maxClaims,
    minimumUnitAmount,
  );
  const amountValidation = getRedemptionAmountValidation(
    kind,
    amount,
    maxClaims,
    minimumUnitAmount,
  );

  useEffect(() => {
    if (!visible) return;
    setRewardType((current) => {
      if (current === 'balance' && balanceEnabled) return current;
      if (current === 'subscription' && subscriptionEnabled) return current;
      return defaultRewardType;
    });
    setKind((current) => {
      if (current === 'single' && singleEnabled) return current;
      if (current === 'random' && randomEnabled) return current;
      return defaultKind;
    });
    setAmount(minimumUnitAmount);
    setMaxClaims((current) => Math.min(Math.max(current, 2), maxClaimsPerCode));
    setUnits((current) => Math.min(Math.max(current, 1), maxClaimsPerCode));
    setHours((current) => {
      if (current <= maxExpiryHours) return current;
      return expiryOptions.at(-1) || 1;
    });
    setCreatedCode(null);
  }, [
    balanceEnabled,
    defaultKind,
    defaultRewardType,
    expiryOptions,
    maxClaimsPerCode,
    maxExpiryHours,
    minimumUnitAmount,
    randomEnabled,
    singleEnabled,
    subscriptionEnabled,
    visible,
  ]);

  useEffect(() => {
    if (rewardType !== 'subscription' || eligiblePlans.length === 0) return;
    if (eligiblePlans.some(({ plan }) => plan?.id === planId)) return;
    setPlanId(eligiblePlans[0].plan.id);
  }, [eligiblePlans, planId, rewardType]);

  async function submit() {
    if (rewardType === 'subscription' && !subscriptionEnabled) {
      showError(t('This redemption mode is currently disabled'));
      return;
    }
    if (
      rewardType === 'balance' &&
      ((kind === 'single' && !singleEnabled) ||
        (kind === 'random' && !randomEnabled))
    ) {
      showError(t('This redemption mode is currently disabled'));
      return;
    }
    setBusy(true);
    try {
      const expiresAt = Math.floor(Date.now() / 1000) + hours * 3600;
      const request =
        rewardType === 'subscription'
          ? buildCreateUserRedemptionRequest({
              rewardType: 'subscription',
              planId,
              units,
              expiresAt,
            })
          : buildCreateUserRedemptionRequest({
              rewardType: 'balance',
              kind,
              totalQuota: String(
                displayAmountToQuota(
                  normalizeRedemptionAmount(
                    kind,
                    amount,
                    maxClaims,
                    minimumUnitAmount,
                    maxClaimsPerCode,
                  ),
                ),
              ),
              maxClaims: kind === 'random' ? maxClaims : 1,
              expiresAt,
            });
      const code = await createUserRedemption(request);
      setCreatedCode(code);
      showSuccess(t('Created'));
      await onCreated?.(code);
    } catch (error) {
      showError(error?.message || t('create failed'));
    } finally {
      setBusy(false);
    }
  }

  async function copyCreatedCode() {
    if (!createdCode?.code) return;
    if (await copy(createdCode.code)) {
      showSuccess(t('Copied to clipboard'));
    } else {
      showError(t('Failed to copy to clipboard'));
    }
  }

  const creationDisabled =
    busy ||
    (rewardType === 'balance'
      ? amountValidation !== null
      : !selectedPlan || units < 1);

  return (
    <Modal
      title={t('Create Redemption Code')}
      visible={visible}
      onCancel={onClose}
      width={isMobile ? 'calc(100vw - 24px)' : 560}
      bodyStyle={{
        maxHeight: isMobile ? 'calc(100vh - 160px)' : '75vh',
        overflowX: 'hidden',
        overflowY: 'auto',
      }}
      footer={
        <div className='flex justify-end gap-2'>
          <Button onClick={onClose}>{t('Close')}</Button>
          {!createdCode ? (
            <Button
              theme='solid'
              type='primary'
              loading={busy}
              disabled={creationDisabled}
              onClick={() => void submit()}
            >
              {t('Create')}
            </Button>
          ) : null}
        </div>
      }
    >
      {createdCode ? (
        <div className='space-y-4'>
          <Banner
            type='success'
            title={t('Redemption code created')}
            description={t('Copy it now and share it only with the recipient.')}
            closeIcon={null}
          />
          <Input
            value={createdCode.code}
            readonly
            className='font-mono'
            suffix={
              <Button
                theme='borderless'
                icon={<Copy size={16} />}
                aria-label={t('Copy redemption code')}
                onClick={() => void copyCreatedCode()}
              />
            }
          />
          <Button onClick={() => setCreatedCode(null)}>
            {t('Create another code')}
          </Button>
        </div>
      ) : (
        <div className='space-y-5'>
          <Form layout='vertical'>
            <Form.Slot label={t('Reward type')}>
              <ButtonGroup className='w-full'>
                {balanceEnabled ? (
                  <Button
                    className='flex-1'
                    theme={rewardType === 'balance' ? 'solid' : 'light'}
                    type={rewardType === 'balance' ? 'primary' : 'tertiary'}
                    onClick={() => setRewardType('balance')}
                  >
                    {t('Balance')}
                  </Button>
                ) : null}
                {subscriptionEnabled ? (
                  <Button
                    className='flex-1'
                    theme={rewardType === 'subscription' ? 'solid' : 'light'}
                    type={
                      rewardType === 'subscription' ? 'primary' : 'tertiary'
                    }
                    onClick={() => setRewardType('subscription')}
                  >
                    {t('Subscription')}
                  </Button>
                ) : null}
              </ButtonGroup>
            </Form.Slot>

            {rewardType === 'balance' ? (
              <>
                <Form.Slot label={t('Redemption mode')}>
                  <ButtonGroup className='w-full'>
                    {singleEnabled ? (
                      <Button
                        className='flex-1'
                        theme={kind === 'single' ? 'solid' : 'light'}
                        type={kind === 'single' ? 'primary' : 'tertiary'}
                        onClick={() => {
                          setKind('single');
                          setAmount((current) =>
                            Math.max(minimumUnitAmount, current),
                          );
                        }}
                      >
                        {t('Single')}
                      </Button>
                    ) : null}
                    {randomEnabled ? (
                      <Button
                        className='flex-1'
                        theme={kind === 'random' ? 'solid' : 'light'}
                        type={kind === 'random' ? 'primary' : 'tertiary'}
                        onClick={() => {
                          setKind('random');
                          setAmount((current) =>
                            Math.max(
                              getMinimumRedemptionAmount(
                                'random',
                                maxClaims,
                                minimumUnitAmount,
                              ),
                              current,
                            ),
                          );
                        }}
                      >
                        {t('Red packet')}
                      </Button>
                    ) : null}
                  </ButtonGroup>
                </Form.Slot>
                <Form.Slot label={t('Amount')}>
                  <div className='space-y-1'>
                    <InputNumber
                      min={minimumAmount}
                      step={0.01}
                      value={amount}
                      onChange={(value) =>
                        setAmount(sanitizeRedemptionAmountInput(Number(value)))
                      }
                      style={{ width: '100%' }}
                    />
                    {amountValidation ? (
                      <Typography.Text type='danger' size='small' role='alert'>
                        {t(amountValidation.key, amountValidation.values)}
                      </Typography.Text>
                    ) : null}
                  </div>
                </Form.Slot>
                {kind === 'random' ? (
                  <Form.Slot label={t('Recipients')}>
                    <InputNumber
                      min={2}
                      max={maxClaimsPerCode}
                      value={maxClaims}
                      onChange={(value) => {
                        const nextClaims = Number(value);
                        setMaxClaims(
                          Math.min(Math.max(nextClaims, 2), maxClaimsPerCode),
                        );
                        setAmount((current) =>
                          normalizeRedemptionAmount(
                            'random',
                            current,
                            nextClaims,
                            minimumUnitAmount,
                            maxClaimsPerCode,
                          ),
                        );
                      }}
                      style={{ width: '100%' }}
                    />
                  </Form.Slot>
                ) : null}
              </>
            ) : (
              <>
                <Form.Slot label={t('Plan')}>
                  <Select
                    value={planId || undefined}
                    placeholder={t('Select a subscription plan')}
                    onChange={(value) => setPlanId(Number(value) || 0)}
                    style={{ width: '100%' }}
                  >
                    {eligiblePlans.map(({ plan }) => (
                      <Select.Option key={plan.id} value={plan.id}>
                        {plan.title} (
                        {formatSubscriptionPrice(plan.price_amount)})
                      </Select.Option>
                    ))}
                  </Select>
                  {eligiblePlans.length === 0 ? (
                    <Typography.Text type='tertiary' size='small'>
                      {t(
                        'No balance-purchasable subscription plans are available.',
                      )}
                    </Typography.Text>
                  ) : null}
                </Form.Slot>
                <Form.Slot label={t('Units')}>
                  <InputNumber
                    min={1}
                    max={maxClaimsPerCode}
                    step={1}
                    value={units}
                    onChange={(value) =>
                      setUnits(
                        Math.min(
                          Math.max(Math.trunc(Number(value)) || 1, 1),
                          maxClaimsPerCode,
                        ),
                      )
                    }
                    style={{ width: '100%' }}
                  />
                </Form.Slot>
                {selectedPlan ? (
                  <div className='mb-4 grid grid-cols-2 gap-3 rounded-md border border-[var(--semi-color-border)] p-3'>
                    <div>
                      <Typography.Text type='tertiary' size='small'>
                        {t('Per unit')}
                      </Typography.Text>
                      <div className='font-semibold tabular-nums'>
                        {renderQuota(subscriptionCost.unitQuota)}
                      </div>
                    </div>
                    <div>
                      <Typography.Text type='tertiary' size='small'>
                        {t('Total cost')}
                      </Typography.Text>
                      <div className='font-semibold tabular-nums'>
                        {renderQuota(subscriptionCost.totalQuota)}
                      </div>
                    </div>
                  </div>
                ) : null}
              </>
            )}

            <Form.Slot label={t('Validity')}>
              <Select
                value={hours}
                onChange={(value) => setHours(Number(value))}
                style={{ width: '100%' }}
              >
                {expiryOptions.map((optionHours) => {
                  const translation =
                    getUserRedemptionExpiryTranslation(optionHours);
                  return (
                    <Select.Option key={optionHours} value={optionHours}>
                      {t(translation.key, translation.values)}
                    </Select.Option>
                  );
                })}
              </Select>
            </Form.Slot>
          </Form>

          <div className='rounded-md border border-[var(--semi-color-border)] p-3'>
            <div className='mb-2 flex items-center gap-2 font-medium'>
              <Gift size={16} />
              {t('Important')}
            </div>
            <ul className='list-disc space-y-1 pl-5 text-xs text-[var(--semi-color-text-2)]'>
              <li>
                {t(
                  rewardType === 'subscription'
                    ? 'The full subscription cost is reserved from your wallet immediately.'
                    : 'The full amount is reserved from your wallet immediately.',
                )}
              </li>
              {rewardType === 'subscription' ? (
                <>
                  <li>
                    {t('Each user can claim one subscription from this code.')}
                  </li>
                  <li>
                    {t(
                      'Unused subscription units are refunded automatically after expiry or cancellation.',
                    )}
                  </li>
                </>
              ) : (
                <>
                  <li>
                    {t(
                      'A single code gives all remaining balance to the first recipient.',
                    )}
                  </li>
                  <li>
                    {t(
                      'Each user can claim a red packet only once; the final recipient receives the remainder.',
                    )}
                  </li>
                  <li>
                    {t(
                      'Unused balance is submitted for administrator settlement after expiry or when you cancel an active code.',
                    )}
                  </li>
                </>
              )}
              <li>
                {t(
                  'Administrators may terminate a code with or without a refund.',
                )}
              </li>
              <li>
                {t(
                  'Treat the code as a transferable credential and share it carefully.',
                )}
              </li>
            </ul>
          </div>
          {rewardType === 'balance' ? (
            <Typography.Text type='tertiary' size='small'>
              {t('Amount uses the same display unit as your wallet.')}
            </Typography.Text>
          ) : null}
        </div>
      )}
    </Modal>
  );
}
