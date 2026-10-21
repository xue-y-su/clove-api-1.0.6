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
import { useCallback, useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { api } from '@/lib/api'
import { getServerErrorMessage } from '@/lib/handle-server-error'

import { SettingsSection } from '../components/settings-section'
import {
  createEmailTemplate,
  listEmailTemplates,
  previewEmailTemplate,
  setDefaultEmailTemplate,
  type EmailTemplate,
} from './email-templates-api'

export function EmailTemplatesSection() {
  const { t } = useTranslation()
  const [items, setItems] = useState<EmailTemplate[]>([])
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const [name, setName] = useState('自定义邮箱验证模板')
  const [subject, setSubject] = useState('{{site_name}} 邮箱验证')
  const [htmlBody, setHtmlBody] = useState(
    '<p>您好，您正在验证 {{site_name}} 的邮箱。</p><p>您的邮箱验证码是：<strong>{{code}}</strong></p><p>验证码有效期为 {{expiration_minutes}} 分钟。如非本人操作，请忽略本邮件。</p>'
  )
  const [preview, setPreview] = useState<{
    subject: string
    html: string
  } | null>(null)

  const reload = useCallback(async () => {
    setError('')
    try {
      const data = await listEmailTemplates(api)
      setItems(data || [])
    } catch (error: unknown) {
      setError(
        getServerErrorMessage(
          error,
          t('failed to load templates'),
          t('Request ID')
        )
      )
    }
  }, [t])

  useEffect(() => {
    void reload()
  }, [reload])

  const defaultId = useMemo(() => items.find((x) => x.is_default)?.id, [items])

  return (
    <SettingsSection
      title={t('Email Templates')}
      description={t(
        'Manage built-in and custom verification email templates.'
      )}
    >
      {error ? <div className='mb-3 text-sm text-red-600'>{error}</div> : null}
      <div className='mb-4 space-y-3'>
        {items.map((item) => (
          <div
            key={item.id}
            className='flex flex-wrap items-start justify-between gap-2 rounded border p-3'
          >
            <div>
              <div className='font-medium'>
                {item.name}{' '}
                {item.is_default ? (
                  <span className='text-xs opacity-70'>({t('Default')})</span>
                ) : null}
              </div>
              <div className='text-xs opacity-70'>
                {item.kind} · {item.enabled ? t('Enabled') : t('Disabled')} ·{' '}
                {item.subject}
              </div>
            </div>
            <div className='flex gap-2'>
              <Button
                type='button'
                variant='outline'
                size='sm'
                disabled={busy}
                onClick={async () => {
                  setBusy(true)
                  try {
                    const p = await previewEmailTemplate(api, item.id)
                    setPreview(p)
                  } catch (error: unknown) {
                    setError(
                      getServerErrorMessage(
                        error,
                        t('preview failed'),
                        t('Request ID')
                      )
                    )
                  } finally {
                    setBusy(false)
                  }
                }}
              >
                {t('Preview')}
              </Button>
              {!item.is_default ? (
                <Button
                  type='button'
                  size='sm'
                  disabled={busy || item.id === defaultId}
                  onClick={async () => {
                    setBusy(true)
                    try {
                      await setDefaultEmailTemplate(api, item.id)
                      await reload()
                    } catch (error: unknown) {
                      setError(
                        getServerErrorMessage(
                          error,
                          t('set default failed'),
                          t('Request ID')
                        )
                      )
                    } finally {
                      setBusy(false)
                    }
                  }}
                >
                  {t('Set Default')}
                </Button>
              ) : null}
            </div>
          </div>
        ))}
      </div>

      <div className='space-y-2 rounded border p-3'>
        <div className='font-medium'>{t('Create custom template')}</div>
        <Input
          value={name}
          onChange={(e) => setName(e.target.value)}
          placeholder={t('Name')}
        />
        <Input
          value={subject}
          onChange={(e) => setSubject(e.target.value)}
          placeholder={t('Subject')}
        />
        <Textarea
          value={htmlBody}
          onChange={(e) => setHtmlBody(e.target.value)}
          rows={6}
        />
        <Button
          type='button'
          disabled={busy}
          onClick={async () => {
            setBusy(true)
            setError('')
            try {
              await createEmailTemplate(api, {
                name,
                subject,
                html_body: htmlBody,
              })
              await reload()
            } catch (error: unknown) {
              setError(
                getServerErrorMessage(
                  error,
                  t('create failed'),
                  t('Request ID')
                )
              )
            } finally {
              setBusy(false)
            }
          }}
        >
          {t('Create')}
        </Button>
      </div>

      {preview ? (
        <div className='mt-4 rounded border p-3'>
          <div className='mb-2 font-medium'>{preview.subject}</div>
          <iframe
            title='email-template-preview'
            sandbox=''
            srcDoc={preview.html}
            className='min-h-48 w-full rounded border bg-white'
          />
        </div>
      ) : null}
    </SettingsSection>
  )
}
