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
import { Avatar, Button, Input, Modal, Typography } from '@douyinfe/semi-ui';
import { ImagePlus, Save, Trash2 } from 'lucide-react';
import { useTranslation } from 'react-i18next';

import { API, showError, showSuccess } from '../../../../helpers';
import {
  buildProfileIdentityPatch,
  formatDisplayNameCooldownRemaining,
  isDisplayNameCoolingDown,
  uploadAvatarToImageHost,
} from '../profile-identity.js';

function translateProfileError(t, message) {
  switch (message) {
    case '头像必须是 JPEG、PNG、WebP 或 GIF 图片':
      return t('头像必须是 JPEG、PNG、WebP 或 GIF 图片');
    case '头像上传暂不可用':
      return t('头像上传暂不可用');
    case '图床返回了无效响应':
      return t('图床返回了无效响应');
    case '头像上传失败':
      return t('头像上传失败');
    case '图床返回了不受信任的地址':
      return t('图床返回了不受信任的地址');
    case '头像上传超时，请重试':
      return t('头像上传超时，请重试');
    case '头像上传已取消':
      return t('头像上传已取消');
    case '无法连接头像图床，请检查网络或稍后重试':
      return t('无法连接头像图床，请检查网络或稍后重试');
    default:
      return message;
  }
}

const ProfileIdentity = ({ t, userState, refreshUser, open, onOpenChange }) => {
  const { i18n } = useTranslation();
  const fileInputRef = useRef(null);
  const uploadControllerRef = useRef(null);
  const profile = userState?.user;
  const currentDisplayName = profile?.display_name || profile?.username || '';
  const currentAvatarURL = profile?.avatar_url || '';
  const [displayName, setDisplayName] = useState(currentDisplayName);
  const [selectedFile, setSelectedFile] = useState(null);
  const [selectedPreviewURL, setSelectedPreviewURL] = useState('');
  const [removeAvatar, setRemoveAvatar] = useState(false);
  const [savePhase, setSavePhase] = useState('idle');
  const saving = savePhase !== 'idle';

  useEffect(() => {
    if (!open) return;
    setDisplayName(currentDisplayName);
    setSelectedFile(null);
    setRemoveAvatar(false);
    if (fileInputRef.current) fileInputRef.current.value = '';
  }, [currentAvatarURL, currentDisplayName, open]);

  useEffect(() => {
    if (!selectedFile) {
      setSelectedPreviewURL('');
      return undefined;
    }
    const previewURL = URL.createObjectURL(selectedFile);
    setSelectedPreviewURL(previewURL);
    return () => URL.revokeObjectURL(previewURL);
  }, [selectedFile]);

  useEffect(
    () => () => {
      uploadControllerRef.current?.abort();
    },
    [],
  );

  if (!profile) return null;

  const coolingDown = isDisplayNameCoolingDown(
    profile.display_name_next_change_at,
  );
  const cooldownRemaining = formatDisplayNameCooldownRemaining(
    profile.display_name_next_change_at,
    undefined,
    i18n.resolvedLanguage,
  );
  const cooldownDate = profile.display_name_next_change_at
    ? new Date(profile.display_name_next_change_at * 1000).toLocaleString(
        i18n.resolvedLanguage,
      )
    : '';
  const avatarPreview =
    selectedPreviewURL || (removeAvatar ? '' : currentAvatarURL);
  const unchanged =
    displayName.trim() === currentDisplayName && !selectedFile && !removeAvatar;

  const saveProfile = async () => {
    const normalizedDisplayName = displayName.trim();
    if (coolingDown && normalizedDisplayName !== currentDisplayName) {
      showError(
        t('昵称可在 {{remaining}}修改（{{date}}）', {
          remaining: cooldownRemaining,
          date: cooldownDate,
        }),
      );
      return;
    }
    let avatarURL = removeAvatar ? '' : currentAvatarURL;
    const uploadController = selectedFile ? new AbortController() : null;
    uploadControllerRef.current = uploadController;
    try {
      if (selectedFile) {
        if (!profile.avatar_upload) {
          showError(t('头像上传暂不可用'));
          return;
        }
        setSavePhase('uploading');
        avatarURL = await uploadAvatarToImageHost(
          selectedFile,
          profile.avatar_upload,
          fetch,
          { signal: uploadController?.signal },
        );
      }
      setSavePhase('saving');
      const patch = buildProfileIdentityPatch(
        {
          displayName: currentDisplayName,
          avatarURL: currentAvatarURL,
        },
        { displayName, avatarURL },
      );
      const res = await API.put('/api/user/self', patch);
      if (!res.data.success) {
        showError(res.data.message || t('资料保存失败'));
        return;
      }
      showSuccess(t('个人资料已更新'));
      onOpenChange(false);
      await refreshUser?.();
    } catch (error) {
      showError(translateProfileError(t, error?.message || t('资料保存失败')));
    } finally {
      if (uploadControllerRef.current === uploadController) {
        uploadControllerRef.current = null;
      }
      setSavePhase('idle');
    }
  };

  const cooldownNotice = coolingDown
    ? t('昵称可在 {{remaining}}修改（{{date}}）', {
        remaining: cooldownRemaining,
        date: cooldownDate,
      })
    : '';

  return (
    <Modal
      title={t('编辑个人资料')}
      visible={open}
      centered
      confirmLoading={saving}
      okText={savePhase === 'uploading' ? t('正在上传头像') : t('保存资料')}
      cancelText={savePhase === 'uploading' ? t('取消上传') : t('取消')}
      okButtonProps={{ disabled: saving || unchanged }}
      onOk={saveProfile}
      onCancel={() => {
        if (savePhase === 'uploading') {
          uploadControllerRef.current?.abort();
          return;
        }
        if (savePhase === 'saving') return;
        onOpenChange(false);
      }}
      icon={<Save size={18} />}
    >
      <div className='space-y-5'>
        <div>
          <Typography.Text strong>{t('昵称')}</Typography.Text>
          <Input
            className='mt-2'
            value={displayName}
            maxLength={40}
            disabled={saving || coolingDown}
            onChange={setDisplayName}
          />
          {cooldownNotice && (
            <div className='mt-2 text-xs text-gray-500'>{cooldownNotice}</div>
          )}
        </div>

        <div>
          <Typography.Text strong>{t('头像')}</Typography.Text>
          <div className='mt-2 flex items-center gap-3'>
            <Avatar
              size='extra-large'
              src={avatarPreview || undefined}
              color='blue'
            >
              {currentDisplayName.slice(0, 2).toUpperCase()}
            </Avatar>
            <div className='flex min-w-0 flex-1 flex-wrap gap-2'>
              <input
                ref={fileInputRef}
                type='file'
                accept='image/jpeg,image/png,image/webp,image/gif'
                style={{ display: 'none' }}
                disabled={saving || !profile.avatar_upload?.enabled}
                onChange={(event) => {
                  const file = event.target.files?.[0] || null;
                  setSelectedFile(file);
                  if (file) setRemoveAvatar(false);
                }}
              />
              <Button
                icon={<ImagePlus size={16} />}
                disabled={saving || !profile.avatar_upload?.enabled}
                onClick={() => fileInputRef.current?.click()}
              >
                {t('选择图片')}
              </Button>
              {(currentAvatarURL || selectedFile) && !removeAvatar && (
                <Button
                  theme='borderless'
                  type='danger'
                  icon={<Trash2 size={16} />}
                  aria-label={t('移除头像')}
                  disabled={saving}
                  onClick={() => {
                    setSelectedFile(null);
                    setRemoveAvatar(true);
                    if (fileInputRef.current) {
                      fileInputRef.current.value = '';
                    }
                  }}
                />
              )}
            </div>
          </div>
          {selectedFile && (
            <div className='mt-2 truncate text-xs text-gray-500'>
              {selectedFile.name}
            </div>
          )}
          {!profile.avatar_upload?.enabled && (
            <div className='mt-2 text-xs text-gray-500'>
              {t('头像上传暂不可用')}
            </div>
          )}
        </div>
      </div>
    </Modal>
  );
};

export default ProfileIdentity;
