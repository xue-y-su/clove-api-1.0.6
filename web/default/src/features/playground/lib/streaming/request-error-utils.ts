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
import { ERROR_MESSAGES } from '../../constants'

type RequestErrorLike = {
  message?: string
  response?: {
    status?: number
    data?: {
      error?: {
        code?: string
        message?: string
      }
      message?: string
    }
  }
}

export type RequestErrorDetails = {
  errorCode?: string
  errorMessage: string
}

export function parseRequestErrorDetails(error: unknown): RequestErrorDetails {
  const requestError = error as RequestErrorLike
  const providerMessage =
    requestError?.response?.data?.error?.message?.trim() || ''
  const status = requestError?.response?.status

  if (
    providerMessage.toLowerCase() === 'openai_error' &&
    Number.isInteger(status) &&
    Number(status) >= 400
  ) {
    return {
      errorCode: requestError.response?.data?.error?.code || undefined,
      errorMessage: `HTTP ${status}: ${ERROR_MESSAGES.CONNECTION_CLOSED}`,
    }
  }

  return {
    errorCode: requestError?.response?.data?.error?.code || undefined,
    errorMessage:
      providerMessage ||
      requestError?.response?.data?.message ||
      requestError?.message ||
      ERROR_MESSAGES.API_REQUEST_ERROR,
  }
}
