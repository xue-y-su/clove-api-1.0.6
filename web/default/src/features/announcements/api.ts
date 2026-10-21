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
import axios from 'axios'

import type { AnnouncementItem } from '@/features/dashboard/types'
import { api } from '@/lib/api'

import { isManagedAnnouncementImageURL } from './global-announcement'

type ApiResponse<T> = {
  success: boolean
  message?: string
  data?: T
}

export type AnnouncementPopupState = {
  enabled: boolean
  announcement: AnnouncementItem | null
  signature: string
  acknowledged: boolean
}

function responseData<T>(response: ApiResponse<T>, fallback: string): T {
  if (!response.success || response.data === undefined) {
    throw new Error(response.message || fallback)
  }
  return response.data
}

function requestError(error: unknown, fallback: string): Error {
  if (axios.isAxiosError(error)) {
    const message = error.response?.data?.message
    if (typeof message === 'string' && message.trim()) {
      return new Error(message)
    }
  }
  return error instanceof Error ? error : new Error(fallback)
}

export async function getAnnouncementPopupState(): Promise<AnnouncementPopupState> {
  try {
    const response = await api.get<ApiResponse<AnnouncementPopupState>>(
      '/api/announcement/popup-state',
      { skipBusinessError: true, skipErrorHandler: true }
    )
    return responseData(response.data, 'Failed to load announcement')
  } catch (error) {
    throw requestError(error, 'Failed to load announcement')
  }
}

export async function acknowledgeAnnouncement(
  signature: string
): Promise<void> {
  try {
    const response = await api.post<ApiResponse<{ acknowledged: boolean }>>(
      '/api/announcement/acknowledge',
      { signature },
      { skipBusinessError: true, skipErrorHandler: true }
    )
    const result = responseData(
      response.data,
      'Announcement confirmation failed. Please try again.'
    )
    if (!result.acknowledged) {
      throw new Error('Announcement confirmation failed. Please try again.')
    }
  } catch (error) {
    throw requestError(
      error,
      'Announcement confirmation failed. Please try again.'
    )
  }
}

export async function uploadAnnouncementMedia(file: File): Promise<string> {
  const formData = new FormData()
  formData.append('file', file)

  try {
    const response = await api.post<ApiResponse<{ url: string }>>(
      '/api/announcement/media',
      formData,
      { skipBusinessError: true, skipErrorHandler: true }
    )
    const result = responseData(
      response.data,
      'Upload announcement image failed'
    )
    if (!isManagedAnnouncementImageURL(result.url)) {
      throw new Error('Upload announcement image failed')
    }
    return result.url
  } catch (error) {
    throw requestError(error, 'Upload announcement image failed')
  }
}
