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
import { zodResolver } from '@hookform/resolvers/zod'
import { useEffect } from 'react'
import { useForm, type UseFormRegisterReturn } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { z } from 'zod'

import { DateTimePicker } from '@/components/datetime-picker'
import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'
import { Textarea } from '@/components/ui/textarea'

import {
  buildInvitationIssueSpec,
  formatInvitationLocalDateTime,
} from '../lib/invitation-state'
import type {
  InvitationApplicationListItem,
  InvitationCodeKind,
  InvitationCodeListItem,
} from '../types'

export interface ReviewSubmission {
  approve: boolean
  review_reason: string
  kind: InvitationCodeKind
  max_uses: number
  expires_at: number
}

export interface IssueSubmission {
  target_user_id: number
  reason: string
  kind: InvitationCodeKind
  max_uses: number
  expires_at: number
}

const reviewSchema = z.object({
  reviewReason: z.string().trim().max(500),
  kind: z.enum(['single', 'multi']),
  maxUses: z.number().int().min(1).max(100),
  expiresAt: z.string().min(1),
})

const issueSchema = reviewSchema.extend({
  targetUserId: z.number().int().positive(),
  reviewReason: z.string().trim().min(1).max(500),
})

const revokeSchema = z.object({
  reason: z.string().trim().min(1).max(500),
})

type ReviewForm = z.infer<typeof reviewSchema>
type IssueForm = z.infer<typeof issueSchema>
type RevokeForm = z.infer<typeof revokeSchema>

export function ReviewInvitationDialog(props: {
  open: boolean
  onOpenChange: (open: boolean) => void
  application: InvitationApplicationListItem | null
  approve: boolean
  pending: boolean
  onSubmit: (value: ReviewSubmission) => void
}) {
  const { t } = useTranslation()
  const form = useForm<ReviewForm>({
    resolver: zodResolver(reviewSchema),
    defaultValues: defaultReviewForm(),
  })
  const kind = form.watch('kind')
  const expiresAt = form.watch('expiresAt')

  useEffect(() => {
    if (props.open) form.reset(defaultReviewForm())
  }, [form, props.open])

  function submit(value: ReviewForm) {
    if (!props.approve && !value.reviewReason.trim()) {
      form.setError('reviewReason', {
        message: t('A rejection reason is required'),
      })
      return
    }
    const issue = props.approve
      ? buildInvitationIssueSpec(value.kind, value.maxUses, value.expiresAt)
      : {
          kind: 'single' as const,
          max_uses: 1,
          expires_at: 0,
        }
    if (!issue) {
      form.setError('expiresAt', {
        message: t('Use 2 to 100 times and choose an expiry within 365 days.'),
      })
      return
    }
    props.onSubmit({
      approve: props.approve,
      review_reason: value.reviewReason.trim(),
      ...issue,
    })
  }

  return (
    <Dialog
      open={props.open}
      onOpenChange={props.onOpenChange}
      title={props.approve ? t('Approve application') : t('Reject application')}
      description={
        props.application
          ? t('Review the request from {{username}}.', {
              username: props.application.applicant_username,
            })
          : undefined
      }
      contentClassName='sm:max-w-lg'
      footer={
        <Button
          variant={props.approve ? 'default' : 'destructive'}
          disabled={props.pending}
          onClick={form.handleSubmit(submit)}
        >
          {props.approve ? t('Approve and issue') : t('Reject application')}
        </Button>
      }
    >
      <form className='space-y-4' onSubmit={form.handleSubmit(submit)}>
        {props.approve ? (
          <IssueFields
            kind={kind}
            kindField={form.register('kind')}
            maxUsesField={form.register('maxUses', { valueAsNumber: true })}
            expiresAt={expiresAt}
            onExpiresAtChange={(value) =>
              form.setValue('expiresAt', value, { shouldValidate: true })
            }
            maxUsesError={form.formState.errors.maxUses?.message}
            issueError={form.formState.errors.expiresAt?.message}
          />
        ) : null}
        <Field label={props.approve ? t('Review note') : t('Rejection reason')}>
          <Textarea maxLength={500} {...form.register('reviewReason')} />
          <FieldError message={form.formState.errors.reviewReason?.message} />
        </Field>
      </form>
    </Dialog>
  )
}

export function IssueInvitationDialog(props: {
  open: boolean
  onOpenChange: (open: boolean) => void
  pending: boolean
  onSubmit: (value: IssueSubmission) => void
}) {
  const { t } = useTranslation()
  const form = useForm<IssueForm>({
    resolver: zodResolver(issueSchema),
    defaultValues: { ...defaultReviewForm(), targetUserId: 0 },
  })
  const kind = form.watch('kind')
  const expiresAt = form.watch('expiresAt')

  useEffect(() => {
    if (props.open) form.reset({ ...defaultReviewForm(), targetUserId: 0 })
  }, [form, props.open])

  function submit(value: IssueForm) {
    const issue = buildInvitationIssueSpec(
      value.kind,
      value.maxUses,
      value.expiresAt
    )
    if (!issue) {
      form.setError('expiresAt', {
        message: t('Use 2 to 100 times and choose an expiry within 365 days.'),
      })
      return
    }
    props.onSubmit({
      target_user_id: value.targetUserId,
      reason: value.reviewReason.trim(),
      ...issue,
    })
  }

  return (
    <Dialog
      open={props.open}
      onOpenChange={props.onOpenChange}
      title={t('Issue invitation code')}
      description={t('Issue a code directly without an application.')}
      contentClassName='sm:max-w-lg'
      footer={
        <Button disabled={props.pending} onClick={form.handleSubmit(submit)}>
          {t('Issue code')}
        </Button>
      }
    >
      <form className='space-y-4' onSubmit={form.handleSubmit(submit)}>
        <Field label={t('Target user ID')}>
          <Input
            type='number'
            min={1}
            {...form.register('targetUserId', { valueAsNumber: true })}
          />
          <FieldError message={form.formState.errors.targetUserId?.message} />
        </Field>
        <IssueFields
          kind={kind}
          kindField={form.register('kind')}
          maxUsesField={form.register('maxUses', { valueAsNumber: true })}
          expiresAt={expiresAt}
          onExpiresAtChange={(value) =>
            form.setValue('expiresAt', value, { shouldValidate: true })
          }
          maxUsesError={form.formState.errors.maxUses?.message}
          issueError={form.formState.errors.expiresAt?.message}
        />
        <Field label={t('Issue reason')}>
          <Textarea maxLength={500} {...form.register('reviewReason')} />
          <FieldError message={form.formState.errors.reviewReason?.message} />
        </Field>
      </form>
    </Dialog>
  )
}

export function RevokeInvitationDialog(props: {
  open: boolean
  onOpenChange: (open: boolean) => void
  code: InvitationCodeListItem | null
  pending: boolean
  onSubmit: (reason: string) => void
}) {
  const { t } = useTranslation()
  const form = useForm<RevokeForm>({
    resolver: zodResolver(revokeSchema),
    defaultValues: { reason: '' },
  })

  useEffect(() => {
    if (props.open) form.reset({ reason: '' })
  }, [form, props.open])

  return (
    <Dialog
      open={props.open}
      onOpenChange={props.onOpenChange}
      title={t('Revoke invitation code')}
      description={
        props.code
          ? t('Revoke the active code owned by {{username}}.', {
              username: props.code.owner_username,
            })
          : undefined
      }
      contentClassName='sm:max-w-md'
      footer={
        <Button
          variant='destructive'
          disabled={props.pending}
          onClick={form.handleSubmit((value) =>
            props.onSubmit(value.reason.trim())
          )}
        >
          {t('Revoke code')}
        </Button>
      }
    >
      <Field label={t('Revocation reason')}>
        <Textarea maxLength={500} {...form.register('reason')} />
        <FieldError message={form.formState.errors.reason?.message} />
      </Field>
    </Dialog>
  )
}

function IssueFields(props: {
  kind: InvitationCodeKind
  kindField: UseFormRegisterReturn<'kind'>
  maxUsesField: UseFormRegisterReturn<'maxUses'>
  expiresAt: string
  onExpiresAtChange: (value: string) => void
  maxUsesError?: string
  issueError?: string
}) {
  const { t } = useTranslation()
  const now = Date.now()
  return (
    <div className='grid gap-4 sm:grid-cols-2'>
      <Field label={t('Code type')}>
        <NativeSelect className='w-full' {...props.kindField}>
          <NativeSelectOption value='single'>
            {t('Single use')}
          </NativeSelectOption>
          <NativeSelectOption value='multi'>
            {t('Multiple use')}
          </NativeSelectOption>
        </NativeSelect>
      </Field>
      <Field label={t('Maximum uses')}>
        <Input
          type='number'
          min={props.kind === 'single' ? 1 : 2}
          max={100}
          disabled={props.kind === 'single'}
          {...props.maxUsesField}
        />
        <FieldError message={props.maxUsesError} />
      </Field>
      <div className='sm:col-span-2'>
        <Field label={t('Expires at')}>
          <DateTimePicker
            value={props.expiresAt ? new Date(props.expiresAt) : undefined}
            onChange={(value) =>
              props.onExpiresAtChange(
                value ? formatInvitationLocalDateTime(value.getTime()) : ''
              )
            }
            minDate={new Date(now)}
            maxDate={new Date(now + 365 * 24 * 60 * 60 * 1000)}
            aria-label={t('Expires at')}
          />
          <FieldError message={props.issueError} />
        </Field>
      </div>
    </div>
  )
}

function Field(props: { label: string; children: React.ReactNode }) {
  return (
    <label className='block space-y-1.5 text-sm font-medium'>
      <span>{props.label}</span>
      {props.children}
    </label>
  )
}

function FieldError(props: { message?: string }) {
  return props.message ? (
    <span className='text-destructive block text-xs'>{props.message}</span>
  ) : null
}

function defaultReviewForm(): ReviewForm {
  return {
    reviewReason: '',
    kind: 'single',
    maxUses: 1,
    expiresAt: formatInvitationLocalDateTime(
      Date.now() + 30 * 24 * 60 * 60 * 1000
    ),
  }
}
