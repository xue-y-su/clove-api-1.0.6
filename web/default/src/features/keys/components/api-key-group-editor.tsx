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
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { updateApiKeyGroup } from '../api'
import { ERROR_MESSAGES, SUCCESS_MESSAGES } from '../constants'
import type { ApiKey } from '../types'
import {
  ApiKeyGroupCombobox,
  type ApiKeyGroupOption,
} from './api-key-group-combobox'
import { useApiKeys } from './api-keys-provider'

type ApiKeyGroupEditorProps = {
  apiKey: ApiKey
  options: ApiKeyGroupOption[]
}

export function ApiKeyGroupEditor(props: ApiKeyGroupEditorProps) {
  const { t } = useTranslation()
  const { triggerRefresh } = useApiKeys()
  const [isUpdating, setIsUpdating] = useState(false)

  const handleValueChange = async (group: string) => {
    if (group === props.apiKey.group || isUpdating) return

    setIsUpdating(true)
    try {
      const result = await updateApiKeyGroup(props.apiKey.id, group)
      if (!result.success) {
        toast.error(result.message || t(ERROR_MESSAGES.UPDATE_FAILED))
        return
      }
      toast.success(t(SUCCESS_MESSAGES.API_KEY_UPDATED))
      triggerRefresh()
    } catch (error) {
      toast.error(
        error instanceof Error && error.message
          ? error.message
          : t(ERROR_MESSAGES.UPDATE_FAILED)
      )
    } finally {
      setIsUpdating(false)
    }
  }

  return (
    <ApiKeyGroupCombobox
      options={props.options}
      value={props.apiKey.group ?? ''}
      onValueChange={handleValueChange}
      disabled={isUpdating || props.options.length === 0}
      compact
    />
  )
}
