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
import React, { useEffect, useState } from 'react';
import {
  Button,
  Card,
  Input,
  TextArea,
  Space,
  Spin,
  Tag,
  Typography,
} from '@douyinfe/semi-ui';
import { API, showError, showSuccess } from '../../../helpers';
import { useTranslation } from 'react-i18next';

const { Text, Title } = Typography;

const EMAIL_TEMPLATE_KIND_LABELS = {
  builtin: 'Built-in',
  custom: 'Custom',
};

function getEmailTemplateName(item, t) {
  if (item.builtin_key === 'verification_default') {
    return t('Default verification');
  }
  return item.name;
}

export default function SettingsEmailTemplates() {
  const { t } = useTranslation();
  const [items, setItems] = useState([]);
  const [loading, setLoading] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [name, setName] = useState('自定义邮箱验证模板');
  const [subject, setSubject] = useState('{{site_name}} 邮箱验证');
  const [htmlBody, setHtmlBody] = useState(
    '<p>您好，您正在验证 {{site_name}} 的邮箱。</p><p>您的邮箱验证码是：<strong>{{code}}</strong></p><p>验证码有效期为 {{expiration_minutes}} 分钟。如非本人操作，请忽略本邮件。</p>',
  );
  const [preview, setPreview] = useState(null);

  async function reload() {
    setLoading(true);
    setError('');
    try {
      const res = await API.get('/api/email-template/');
      setItems(res.data?.data || []);
    } catch (e) {
      setError(e?.message || t('failed to load templates'));
      showError(e?.message || t('failed to load templates'));
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    void reload();
  }, []);

  return (
    <Spin spinning={loading}>
      <div style={{ marginBottom: 12 }}>
        <Title heading={5} style={{ margin: 0 }}>
          {t('Email Templates')}
        </Title>
        <Text type='tertiary'>
          {t('Manage built-in and custom verification email templates.')}
        </Text>
      </div>
      {error ? <Text type='danger'>{error}</Text> : null}

      <Space vertical style={{ width: '100%' }} spacing={12}>
        {items.map((item) => (
          <Card key={item.id} style={{ width: '100%' }}>
            <div
              style={{
                display: 'flex',
                justifyContent: 'space-between',
                gap: 12,
                flexWrap: 'wrap',
              }}
            >
              <div>
                <div>
                  <Text strong>{getEmailTemplateName(item, t)}</Text>{' '}
                  {item.is_default ? (
                    <Tag color='green'>{t('Default')}</Tag>
                  ) : null}{' '}
                  <Tag>
                    {t(EMAIL_TEMPLATE_KIND_LABELS[item.kind] || item.kind)}
                  </Tag>
                </div>
                <Text type='tertiary' size='small'>
                  {item.subject}
                </Text>
              </div>
              <Space>
                <Button
                  disabled={busy}
                  onClick={async () => {
                    setBusy(true);
                    try {
                      const res = await API.get(
                        `/api/email-template/${item.id}/preview`,
                      );
                      setPreview(res.data?.data || null);
                    } catch (e) {
                      showError(e?.message || t('preview failed'));
                    } finally {
                      setBusy(false);
                    }
                  }}
                >
                  {t('Preview')}
                </Button>
                {!item.is_default ? (
                  <Button
                    theme='solid'
                    type='primary'
                    disabled={busy}
                    onClick={async () => {
                      setBusy(true);
                      try {
                        await API.post(
                          `/api/email-template/${item.id}/default`,
                        );
                        showSuccess(t('Default updated'));
                        await reload();
                      } catch (e) {
                        showError(e?.message || t('set default failed'));
                      } finally {
                        setBusy(false);
                      }
                    }}
                  >
                    {t('Set Default')}
                  </Button>
                ) : null}
              </Space>
            </div>
          </Card>
        ))}
      </Space>

      <Card style={{ marginTop: 16 }}>
        <Title heading={6}>{t('Create custom template')}</Title>
        <Space vertical style={{ width: '100%' }} spacing={8}>
          <Input value={name} onChange={setName} placeholder={t('Name')} />
          <Input
            value={subject}
            onChange={setSubject}
            placeholder={t('Subject')}
          />
          <TextArea value={htmlBody} onChange={setHtmlBody} rows={6} />
          <Button
            theme='solid'
            type='primary'
            disabled={busy}
            onClick={async () => {
              setBusy(true);
              try {
                await API.post('/api/email-template/', {
                  name,
                  subject,
                  html_body: htmlBody,
                });
                showSuccess(t('Created'));
                await reload();
              } catch (e) {
                showError(e?.message || t('create failed'));
              } finally {
                setBusy(false);
              }
            }}
          >
            {t('Create')}
          </Button>
        </Space>
      </Card>

      {preview ? (
        <Card style={{ marginTop: 16 }}>
          <Title heading={6}>{preview.subject}</Title>
          <iframe
            title='email-template-preview'
            sandbox=''
            srcDoc={preview.html}
            style={{
              width: '100%',
              minHeight: 180,
              border: '1px solid var(--semi-color-border)',
              borderRadius: 6,
              background: '#fff',
            }}
          />
        </Card>
      ) : null}
    </Spin>
  );
}
