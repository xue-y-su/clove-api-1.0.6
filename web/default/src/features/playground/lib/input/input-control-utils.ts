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
import type { GroupOption, ModelOption } from '../../types'

type InputControlStateOptions = {
  disabled?: boolean
  groups: GroupOption[]
  hasAttachments?: boolean
  hasStopHandler: boolean
  isGenerating?: boolean
  isModelLoading?: boolean
  models: ModelOption[]
  text: string
}

type InputControlState = {
  canSubmit: boolean
  isSelectorDisabled: boolean
  shouldShowStop: boolean
  submitDisabledReason: SubmitDisabledReason
}

type SubmittableInputMessage = {
  files?: unknown[] | null
  text?: string | null
}

export type SubmitDisabledReason =
  | 'disabled'
  | 'empty'
  | 'loadingModels'
  | 'noModel'
  | null

function getSubmitDisabledReason({
  canSubmit,
  disabled,
  hasModels,
  isModelLoading,
}: {
  canSubmit: boolean
  disabled?: boolean
  hasModels: boolean
  isModelLoading?: boolean
}): SubmitDisabledReason {
  if (canSubmit) {
    return null
  }

  if (disabled) {
    return 'disabled'
  }

  if (isModelLoading) {
    return 'loadingModels'
  }

  if (!hasModels) {
    return 'noModel'
  }

  return 'empty'
}

export function getSubmittableInputText(
  message: SubmittableInputMessage,
  disabled?: boolean
): string | null {
  if (disabled) {
    return null
  }

  const text = message.text ?? ''
  if (text.trim() || (message.files?.length ?? 0) > 0) {
    return text
  }

  return null
}

export function getInputControlState({
  disabled,
  groups,
  hasAttachments = false,
  hasStopHandler,
  isGenerating,
  isModelLoading,
  models,
  text,
}: InputControlStateOptions): InputControlState {
  const hasModels = models.length > 0
  const hasContent = text.trim().length > 0 || hasAttachments
  const canSubmit = !disabled && !isModelLoading && hasModels && hasContent

  return {
    canSubmit,
    isSelectorDisabled: disabled || isModelLoading || groups.length === 0,
    shouldShowStop: Boolean(isGenerating && hasStopHandler),
    submitDisabledReason: getSubmitDisabledReason({
      canSubmit,
      disabled,
      hasModels,
      isModelLoading,
    }),
  }
}
