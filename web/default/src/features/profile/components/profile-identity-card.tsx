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
import { ImagePlus, Loader2, Save, Trash2 } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Dialog } from '@/components/dialog'
import { Avatar, AvatarFallback, AvatarImage } from '@/components/ui/avatar'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'

import { updateUserProfile } from '../api'
import {
  buildProfilePatch,
  formatDisplayNameCooldownDate,
  formatDisplayNameCooldownRemaining,
  isDisplayNameCoolingDown,
  uploadAvatarToImageHost,
} from '../lib/profile-identity'
import type { UserProfile } from '../types'

interface ProfileIdentityDialogProps {
  profile: UserProfile | null
  open: boolean
  onOpenChange: (open: boolean) => void
  onProfileUpdate: () => void
}

export function ProfileIdentityDialog({
  profile,
  open,
  onOpenChange,
  onProfileUpdate,
}: ProfileIdentityDialogProps) {
  const { t, i18n } = useTranslation()
  const fileInputRef = useRef<HTMLInputElement>(null)
  const uploadControllerRef = useRef<AbortController | null>(null)
  const [displayName, setDisplayName] = useState('')
  const [selectedFile, setSelectedFile] = useState<File | null>(null)
  const [selectedPreviewURL, setSelectedPreviewURL] = useState('')
  const [removeAvatar, setRemoveAvatar] = useState(false)
  const [savePhase, setSavePhase] = useState<'idle' | 'uploading' | 'saving'>(
    'idle'
  )
  const saving = savePhase !== 'idle'

  const currentDisplayName = profile?.display_name || profile?.username || ''
  const currentAvatarURL = profile?.avatar_url || ''

  useEffect(() => {
    if (!open) return
    setDisplayName(currentDisplayName)
    setSelectedFile(null)
    setRemoveAvatar(false)
    if (fileInputRef.current) fileInputRef.current.value = ''
  }, [currentAvatarURL, currentDisplayName, open])

  useEffect(() => {
    if (!selectedFile) {
      setSelectedPreviewURL('')
      return
    }
    const previewURL = URL.createObjectURL(selectedFile)
    setSelectedPreviewURL(previewURL)
    return () => URL.revokeObjectURL(previewURL)
  }, [selectedFile])

  useEffect(
    () => () => {
      uploadControllerRef.current?.abort()
    },
    []
  )

  if (!profile) return null

  const currentProfile = profile
  const coolingDown = isDisplayNameCoolingDown(currentProfile)
  const cooldownRemaining = formatDisplayNameCooldownRemaining(
    currentProfile.display_name_next_change_at,
    undefined,
    i18n.resolvedLanguage
  )
  const cooldownDate = formatDisplayNameCooldownDate(
    currentProfile.display_name_next_change_at,
    i18n.resolvedLanguage
  )
  const previewURL =
    selectedPreviewURL || (removeAvatar ? '' : currentAvatarURL)
  const unchanged =
    displayName.trim() === currentDisplayName && !selectedFile && !removeAvatar

  function resetDraft() {
    setDisplayName(currentDisplayName)
    setSelectedFile(null)
    setRemoveAvatar(false)
    if (fileInputRef.current) fileInputRef.current.value = ''
  }

  function handleOpenChange(nextOpen: boolean) {
    if (!nextOpen && savePhase === 'uploading') {
      uploadControllerRef.current?.abort()
      return
    }
    if (!nextOpen && savePhase === 'saving') return
    onOpenChange(nextOpen)
    if (nextOpen) resetDraft()
  }

  async function handleSubmit() {
    const normalizedDisplayName = displayName.trim()
    if (coolingDown && normalizedDisplayName !== currentDisplayName) {
      toast.error(
        t('Display name can be changed {{remaining}} ({{date}})', {
          remaining: cooldownRemaining,
          date: cooldownDate,
        })
      )
      return
    }
    let avatarURL = removeAvatar ? '' : currentAvatarURL
    const uploadController = selectedFile ? new AbortController() : null
    uploadControllerRef.current = uploadController
    try {
      if (selectedFile) {
        if (!currentProfile.avatar_upload) {
          toast.error(t('Avatar uploads are unavailable'))
          return
        }
        setSavePhase('uploading')
        avatarURL = await uploadAvatarToImageHost(
          selectedFile,
          currentProfile.avatar_upload,
          fetch,
          { signal: uploadController?.signal }
        )
      }
      setSavePhase('saving')
      const patch = buildProfilePatch(currentProfile, {
        display_name: displayName,
        avatar_url: avatarURL,
      })
      const response = await updateUserProfile(patch)
      if (!response.success) {
        toast.error(response.message || t('Failed to update profile'))
        return
      }
      toast.success(t('Profile updated successfully'))
      onOpenChange(false)
      onProfileUpdate()
    } catch (error) {
      const message =
        error instanceof Error ? error.message : 'Failed to update profile'
      toast.error(t(message))
    } finally {
      if (uploadControllerRef.current === uploadController) {
        uploadControllerRef.current = null
      }
      setSavePhase('idle')
    }
  }

  return (
    <Dialog
      open={open}
      onOpenChange={handleOpenChange}
      title={t('Edit profile')}
      description={t('Update your display name and avatar')}
      contentClassName='sm:max-w-md'
      footer={
        <>
          <Button
            type='button'
            variant='outline'
            disabled={savePhase === 'saving'}
            onClick={() => handleOpenChange(false)}
          >
            {savePhase === 'uploading' ? t('Cancel upload') : t('Cancel')}
          </Button>
          <Button
            type='button'
            disabled={saving || unchanged}
            onClick={handleSubmit}
          >
            {saving ? (
              <Loader2 className='animate-spin' aria-hidden='true' />
            ) : (
              <Save aria-hidden='true' />
            )}
            {savePhase === 'uploading'
              ? t('Uploading avatar')
              : t('Save profile')}
          </Button>
        </>
      }
    >
      <div className='space-y-5'>
        <div className='space-y-2'>
          <Label htmlFor='profile-display-name'>{t('Display name')}</Label>
          <Input
            id='profile-display-name'
            value={displayName}
            maxLength={40}
            disabled={saving || coolingDown}
            onChange={(event) => setDisplayName(event.target.value)}
          />
          {coolingDown ? (
            <p className='text-muted-foreground text-xs'>
              {t('Display name can be changed {{remaining}} ({{date}})', {
                remaining: cooldownRemaining,
                date: cooldownDate,
              })}
            </p>
          ) : null}
        </div>

        <div className='space-y-3'>
          <Label>{t('Avatar')}</Label>
          <div className='flex items-center gap-3'>
            <Avatar className='size-16'>
              {previewURL ? (
                <AvatarImage src={previewURL} alt={currentDisplayName} />
              ) : null}
              <AvatarFallback>
                {currentDisplayName.slice(0, 2).toUpperCase()}
              </AvatarFallback>
            </Avatar>
            <div className='flex min-w-0 flex-1 flex-wrap gap-2'>
              <input
                ref={fileInputRef}
                className='sr-only'
                type='file'
                accept='image/jpeg,image/png,image/webp,image/gif'
                disabled={saving || !currentProfile.avatar_upload?.enabled}
                onChange={(event) => {
                  const file = event.target.files?.[0] || null
                  setSelectedFile(file)
                  if (file) setRemoveAvatar(false)
                }}
              />
              <Button
                type='button'
                variant='outline'
                disabled={saving || !currentProfile.avatar_upload?.enabled}
                onClick={() => fileInputRef.current?.click()}
              >
                <ImagePlus aria-hidden='true' />
                {t('Choose image')}
              </Button>
              {(currentAvatarURL || selectedFile) && !removeAvatar ? (
                <Button
                  type='button'
                  size='icon'
                  variant='ghost'
                  aria-label={t('Remove avatar')}
                  disabled={saving}
                  onClick={() => {
                    setSelectedFile(null)
                    setRemoveAvatar(true)
                    if (fileInputRef.current) {
                      fileInputRef.current.value = ''
                    }
                  }}
                >
                  <Trash2 aria-hidden='true' />
                </Button>
              ) : null}
            </div>
          </div>
          {selectedFile ? (
            <p className='text-muted-foreground truncate text-xs'>
              {selectedFile.name}
            </p>
          ) : null}
          {!currentProfile.avatar_upload?.enabled ? (
            <p className='text-muted-foreground text-xs'>
              {t('Avatar uploads are unavailable')}
            </p>
          ) : null}
        </div>
      </div>
    </Dialog>
  )
}
