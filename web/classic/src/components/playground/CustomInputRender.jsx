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

import React, { useRef, useEffect, useCallback } from 'react';
import { Button, Dropdown, Toast, Tooltip } from '@douyinfe/semi-ui';
import {
  Camera,
  FileText,
  Image as ImageIcon,
  MonitorUp,
  Paperclip,
  Upload,
  X,
} from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { usePlayground } from '../../contexts/PlaygroundContext';
import { formatAttachmentSize } from './attachment-utils';

const CustomInputRender = (props) => {
  const { t } = useTranslation();
  const {
    attachments,
    attachmentLoading,
    attachmentsDisabled,
    onFilesSelected,
    onRemoveAttachment,
    onCaptureScreenshot,
  } = usePlayground();
  const { detailProps } = props;
  const { clearContextNode, uploadNode, inputNode, sendNode, onClick } =
    detailProps;
  const containerRef = useRef(null);
  const fileInputRef = useRef(null);
  const photoInputRef = useRef(null);
  const cameraInputRef = useRef(null);

  const handlePaste = useCallback(
    async (e) => {
      const items = e.clipboardData?.items;
      if (!items) return;

      for (let i = 0; i < items.length; i++) {
        const item = items[i];

        if (item.type.indexOf('image') !== -1) {
          e.preventDefault();
          const file = item.getAsFile();

          if (file) {
            await onFilesSelected([file]);
          }
          break;
        }
      }
    },
    [onFilesSelected],
  );

  const handleFiles = async (event) => {
    const files = event.target.files;
    if (files?.length) await onFilesSelected(files);
    event.target.value = '';
  };

  const attachmentMenu = (
    <Dropdown
      trigger='click'
      position='topLeft'
      render={
        <Dropdown.Menu>
          <Dropdown.Item onClick={() => fileInputRef.current?.click()}>
            <Upload size={15} className='mr-2' /> {t('上传文件')}
          </Dropdown.Item>
          <Dropdown.Item onClick={() => photoInputRef.current?.click()}>
            <ImageIcon size={15} className='mr-2' /> {t('上传照片')}
          </Dropdown.Item>
          <Dropdown.Item onClick={onCaptureScreenshot}>
            <MonitorUp size={15} className='mr-2' /> {t('截图')}
          </Dropdown.Item>
          <Dropdown.Item onClick={() => cameraInputRef.current?.click()}>
            <Camera size={15} className='mr-2' /> {t('拍照')}
          </Dropdown.Item>
        </Dropdown.Menu>
      }
    >
      <span className='inline-flex'>
        <Tooltip content={t('添加附件')}>
          <Button
            icon={<Paperclip size={16} />}
            aria-label={t('添加附件')}
            disabled={attachmentLoading || attachmentsDisabled}
            loading={attachmentLoading}
            theme='borderless'
            type='tertiary'
            className='!rounded-full flex-shrink-0'
          />
        </Tooltip>
      </span>
    </Dropdown>
  );

  useEffect(() => {
    const container = containerRef.current;
    if (!container) return;

    container.addEventListener('paste', handlePaste);
    return () => {
      container.removeEventListener('paste', handlePaste);
    };
  }, [handlePaste]);

  // 清空按钮
  const styledClearNode = clearContextNode
    ? React.cloneElement(clearContextNode, {
        className: `!rounded-full !bg-gray-100 hover:!bg-red-500 hover:!text-white flex-shrink-0 transition-all ${clearContextNode.props.className || ''}`,
        style: {
          ...clearContextNode.props.style,
          width: '32px',
          height: '32px',
          minWidth: '32px',
          padding: 0,
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
        },
      })
    : null;

  // 发送按钮
  const styledSendNode = React.cloneElement(sendNode, {
    className: `!rounded-full !bg-purple-500 hover:!bg-purple-600 flex-shrink-0 transition-all ${sendNode.props.className || ''}`,
    style: {
      ...sendNode.props.style,
      width: '32px',
      height: '32px',
      minWidth: '32px',
      padding: 0,
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
    },
  });

  return (
    <div className='p-2 sm:p-4' ref={containerRef}>
      <input
        ref={fileInputRef}
        type='file'
        multiple
        hidden
        onChange={handleFiles}
      />
      <input
        ref={photoInputRef}
        type='file'
        accept='image/*'
        multiple
        hidden
        onChange={handleFiles}
      />
      <input
        ref={cameraInputRef}
        type='file'
        accept='image/*'
        capture='environment'
        hidden
        onChange={handleFiles}
      />
      {attachments.length > 0 && (
        <div className='mb-2 flex max-h-28 flex-wrap gap-2 overflow-y-auto'>
          {attachments.map((attachment) => (
            <div
              key={attachment.id}
              className='flex max-w-full items-center gap-2 rounded-md border px-2 py-1 text-xs'
            >
              {attachment.kind === 'image' ? (
                <img
                  src={attachment.dataUrl}
                  alt={attachment.name}
                  className='h-8 w-8 rounded object-cover'
                />
              ) : (
                <FileText size={16} />
              )}
              <span className='max-w-40 truncate'>{attachment.name}</span>
              <span className='text-semi-color-text-2'>
                {formatAttachmentSize(attachment.size)}
              </span>
              <Button
                icon={<X size={13} />}
                aria-label={t('移除附件')}
                onClick={() => onRemoveAttachment(attachment.id)}
                theme='borderless'
                type='danger'
                size='small'
              />
            </div>
          ))}
        </div>
      )}
      <div
        className='flex items-center gap-2 sm:gap-3 p-2 bg-gray-50 rounded-xl sm:rounded-2xl shadow-sm hover:shadow-md transition-shadow'
        style={{ border: '1px solid var(--semi-color-border)' }}
        onClick={onClick}
        title={t('支持 Ctrl+V 粘贴图片')}
      >
        {/* 清空对话按钮 - 左边 */}
        {styledClearNode}
        {attachmentMenu}
        <div className='flex-1'>{inputNode}</div>
        {/* 发送按钮 - 右边 */}
        {styledSendNode}
      </div>
    </div>
  );
};

export default CustomInputRender;
