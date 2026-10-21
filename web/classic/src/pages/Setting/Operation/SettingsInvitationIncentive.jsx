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

import React, { useEffect, useRef, useState } from 'react';
import {
  Banner,
  Button,
  Col,
  Form,
  Row,
  Spin,
  Typography,
} from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';

import {
  getInvitationIncentiveSettings,
  updateInvitationIncentiveSettings,
} from '../../../components/invitations/api';
import { API, showError, showSuccess, showWarning } from '../../../helpers';

const DEFAULT_POLICY = {
  enabled: true,
  inviter_registration_reward_enabled: true,
  invitee_registration_gift_enabled: true,
  cash_commission_enabled: false,
  cash_commission_rate: '0.00',
  admin_balance_redemption_commission_enabled: false,
  admin_balance_redemption_commission_rate: '0.00',
  admin_subscription_redemption_commission_enabled: false,
  admin_subscription_redemption_commission_rate: '0.00',
};

const POLICY_KEYS = Object.keys(DEFAULT_POLICY);
const QUOTA_KEYS = ['QuotaForInviter', 'QuotaForInvitee'];

export default function SettingsInvitationIncentive(props) {
  const { t } = useTranslation();
  const refForm = useRef();
  const [loading, setLoading] = useState(true);
  const [inputs, setInputs] = useState({
    ...DEFAULT_POLICY,
    QuotaForInviter: '0',
    QuotaForInvitee: '0',
  });
  const [savedInputs, setSavedInputs] = useState(inputs);

  useEffect(() => {
    let active = true;

    async function loadSettings() {
      setLoading(true);
      try {
        const policy = await getInvitationIncentiveSettings();
        if (!active) return;
        const nextInputs = {
          ...DEFAULT_POLICY,
          ...policy,
          QuotaForInviter: String(props.options?.QuotaForInviter ?? 0),
          QuotaForInvitee: String(props.options?.QuotaForInvitee ?? 0),
        };
        setInputs(nextInputs);
        setSavedInputs(structuredClone(nextInputs));
        refForm.current?.setValues(nextInputs);
      } catch (error) {
        if (active) {
          showError(
            error?.message || t('Unable to load invitation incentive settings'),
          );
        }
      } finally {
        if (active) setLoading(false);
      }
    }

    void loadSettings();
    return () => {
      active = false;
    };
  }, [props.options, t]);

  const complianceConfirmed =
    (props.options?.['payment_setting.compliance_confirmed'] === true ||
      props.options?.['payment_setting.compliance_confirmed'] === 'true') &&
    props.options?.['payment_setting.compliance_terms_version'] === 'v1';
  const childrenDisabled = !inputs.enabled || loading;

  function setField(key, value) {
    setInputs((current) => ({ ...current, [key]: value }));
  }

  async function updateQuotaOption(key) {
    const response = await API.put('/api/option/', {
      key,
      value: String(inputs[key]),
    });
    if (!response?.data?.success) {
      throw new Error(
        response?.data?.message || t('Save failed, please retry'),
      );
    }
  }

  async function onSubmit() {
    const policyChanged = POLICY_KEYS.some(
      (key) => String(inputs[key]) !== String(savedInputs[key]),
    );
    const changedQuotaKeys = QUOTA_KEYS.filter(
      (key) => String(inputs[key]) !== String(savedInputs[key]),
    );
    if (!policyChanged && changedQuotaKeys.length === 0) {
      showWarning(t('你似乎并没有修改什么'));
      return;
    }

    setLoading(true);
    try {
      const operations = changedQuotaKeys.map(updateQuotaOption);
      if (policyChanged) {
        operations.push(
          updateInvitationIncentiveSettings(
            Object.fromEntries(POLICY_KEYS.map((key) => [key, inputs[key]])),
          ),
        );
      }
      await Promise.all(operations);
      showSuccess(t('Invitation incentive settings updated'));
      setSavedInputs(structuredClone(inputs));
      await props.refresh();
    } catch (error) {
      showError(error?.message || t('Save failed, please retry'));
    } finally {
      setLoading(false);
    }
  }

  return (
    <Spin spinning={loading}>
      {!complianceConfirmed ? (
        <Banner
          type='warning'
          description={t(
            'Non-zero invitation rewards require compliance confirmation in Payment Gateway settings.',
          )}
          closeIcon={null}
          className='!rounded-lg mb-3'
        />
      ) : null}
      <Form
        values={inputs}
        getFormApi={(formAPI) => (refForm.current = formAPI)}
        style={{ marginBottom: 15 }}
      >
        <Form.Section text={t('Invitation incentives')}>
          <Typography.Text
            type='tertiary'
            style={{ marginBottom: 16, display: 'block' }}
          >
            {t(
              'Keep invitation relationships while controlling whether new rewards are created.',
            )}
          </Typography.Text>
          <Row gutter={16}>
            <Col span={24}>
              <Form.Switch
                field='enabled'
                label={t('Enable invitation incentives')}
                checkedText='｜'
                uncheckedText='〇'
                disabled={loading}
                onChange={(value) => setField('enabled', value)}
              />
            </Col>
          </Row>

          <Row gutter={16}>
            <Col xs={24} md={12}>
              <Form.Switch
                field='inviter_registration_reward_enabled'
                label={t('Inviter registration reward')}
                checkedText='｜'
                uncheckedText='〇'
                disabled={childrenDisabled}
                onChange={(value) =>
                  setField('inviter_registration_reward_enabled', value)
                }
              />
            </Col>
            <Col xs={24} md={12}>
              <Form.InputNumber
                field='QuotaForInviter'
                label={t('Inviter reward amount')}
                min={0}
                step={1}
                suffix='Token'
                disabled={
                  childrenDisabled ||
                  !inputs.inviter_registration_reward_enabled
                }
                onChange={(value) =>
                  setField('QuotaForInviter', String(value ?? ''))
                }
              />
            </Col>
          </Row>

          <Row gutter={16}>
            <Col xs={24} md={12}>
              <Form.Switch
                field='invitee_registration_gift_enabled'
                label={t('Invitee registration gift')}
                checkedText='｜'
                uncheckedText='〇'
                disabled={childrenDisabled}
                onChange={(value) =>
                  setField('invitee_registration_gift_enabled', value)
                }
              />
            </Col>
            <Col xs={24} md={12}>
              <Form.InputNumber
                field='QuotaForInvitee'
                label={t('Invitee gift amount')}
                min={0}
                step={1}
                suffix='Token'
                disabled={
                  childrenDisabled || !inputs.invitee_registration_gift_enabled
                }
                onChange={(value) =>
                  setField('QuotaForInvitee', String(value ?? ''))
                }
              />
            </Col>
          </Row>

          <CommissionRow
            switchField='cash_commission_enabled'
            switchLabel={t('Cash payment commission')}
            rateField='cash_commission_rate'
            rateLabel={t('Cash commission rate')}
            inputs={inputs}
            childrenDisabled={childrenDisabled}
            setField={setField}
          />
          <CommissionRow
            switchField='admin_balance_redemption_commission_enabled'
            switchLabel={t('Administrator balance-code commission')}
            rateField='admin_balance_redemption_commission_rate'
            rateLabel={t('Balance-code commission rate')}
            inputs={inputs}
            childrenDisabled={childrenDisabled}
            setField={setField}
          />
          <CommissionRow
            switchField='admin_subscription_redemption_commission_enabled'
            switchLabel={t('Administrator subscription-code commission')}
            rateField='admin_subscription_redemption_commission_rate'
            rateLabel={t('Subscription-code commission rate')}
            inputs={inputs}
            childrenDisabled={childrenDisabled}
            setField={setField}
          />

          <Row>
            <Button size='default' onClick={onSubmit} disabled={loading}>
              {t('Save invitation incentive settings')}
            </Button>
          </Row>
        </Form.Section>
      </Form>
    </Spin>
  );
}

function CommissionRow({
  switchField,
  switchLabel,
  rateField,
  rateLabel,
  inputs,
  childrenDisabled,
  setField,
}) {
  const { t } = useTranslation();
  return (
    <Row gutter={16}>
      <Col xs={24} md={12}>
        <Form.Switch
          field={switchField}
          label={switchLabel}
          checkedText='｜'
          uncheckedText='〇'
          disabled={childrenDisabled}
          onChange={(value) => setField(switchField, value)}
        />
      </Col>
      <Col xs={24} md={12}>
        <Form.InputNumber
          field={rateField}
          label={rateLabel}
          min={0}
          step={0.01}
          precision={2}
          suffix='%'
          extraText={t(
            'Percentage of the eligible amount; values above 100% are allowed',
          )}
          disabled={childrenDisabled || !inputs[switchField]}
          onChange={(value) => setField(rateField, String(value ?? ''))}
        />
      </Col>
    </Row>
  );
}
