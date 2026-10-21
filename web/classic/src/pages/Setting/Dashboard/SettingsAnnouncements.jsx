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

import React, { useEffect, useState, useRef } from 'react';
import {
  Button,
  Space,
  Table,
  Form,
  InputNumber,
  Typography,
  Empty,
  Divider,
  Modal,
  Tag,
  Switch,
  TextArea,
  Tooltip,
} from '@douyinfe/semi-ui';
import {
  IllustrationNoResult,
  IllustrationNoResultDark,
} from '@douyinfe/semi-illustrations';
import {
  Plus,
  Edit,
  Trash2,
  Save,
  Bell,
  Maximize2,
  ImagePlus,
  LoaderCircle,
  RefreshCw,
} from 'lucide-react';
import {
  API,
  showError,
  showSuccess,
  getRelativeTime,
  formatDateTimeString,
} from '../../../helpers';
import { useTranslation } from 'react-i18next';
import { useIsMobile } from '../../../hooks/common/useIsMobile';
import {
  isSafeAnnouncementImageURL,
  uploadAnnouncementMedia,
} from '../../../components/layout/global-announcement-api';

const { Text } = Typography;

const SettingsAnnouncements = ({ options, refresh }) => {
  const { t } = useTranslation();
  const isMobile = useIsMobile();

  const [announcementsList, setAnnouncementsList] = useState([]);
  const [showAnnouncementModal, setShowAnnouncementModal] = useState(false);
  const [showDeleteModal, setShowDeleteModal] = useState(false);
  const [showContentModal, setShowContentModal] = useState(false);
  const [deletingAnnouncement, setDeletingAnnouncement] = useState(null);
  const [editingAnnouncement, setEditingAnnouncement] = useState(null);
  const [modalLoading, setModalLoading] = useState(false);
  const [loading, setLoading] = useState(false);
  const [hasChanges, setHasChanges] = useState(false);
  const [announcementForm, setAnnouncementForm] = useState({
    title: '',
    content: '',
    publishDate: new Date(),
    type: 'default',
    extra: '',
    imageUrl: '',
    popup: false,
    published: true,
  });
  const [currentPage, setCurrentPage] = useState(1);
  const [pageSize, setPageSize] = useState(10);
  const [selectedRowKeys, setSelectedRowKeys] = useState([]);
  const [isUploadingImage, setIsUploadingImage] = useState(false);
  const [imageUploadError, setImageUploadError] = useState('');

  // 面板启用状态
  const [panelEnabled, setPanelEnabled] = useState(true);
  const [imageMaxMiB, setImageMaxMiB] = useState(5);
  const [announcementSettingsLoading, setAnnouncementSettingsLoading] =
    useState(true);
  const [announcementSettingsLoaded, setAnnouncementSettingsLoaded] =
    useState(false);
  const [announcementSettingsError, setAnnouncementSettingsError] =
    useState('');
  const [announcementSettingsSaving, setAnnouncementSettingsSaving] =
    useState(false);

  const formApiRef = useRef(null);
  const imageInputRef = useRef(null);

  const loadAnnouncementSettings = async () => {
    setAnnouncementSettingsLoading(true);
    setAnnouncementSettingsLoaded(false);
    setAnnouncementSettingsError('');
    try {
      const res = await API.get('/api/announcement/settings');
      if (!res.data?.success || !res.data?.data) {
        const message =
          res.data?.message || t('Failed to load announcement settings');
        setAnnouncementSettingsError(message);
        showError(message);
        return;
      }
      const settings = res.data.data;
      setPanelEnabled(Boolean(settings.enabled));
      setImageMaxMiB(
        Math.max(
          1,
          Math.min(5, Math.round(settings.image_max_bytes / (1024 * 1024))),
        ),
      );
      setAnnouncementSettingsLoaded(true);
    } catch (error) {
      const message =
        error?.message || t('Failed to load announcement settings');
      setAnnouncementSettingsError(message);
      showError(message);
    } finally {
      setAnnouncementSettingsLoading(false);
    }
  };

  const saveAnnouncementPolicy = async () => {
    if (!announcementSettingsLoaded) return;
    setAnnouncementSettingsSaving(true);
    try {
      const res = await API.put('/api/announcement/settings', {
        enabled: panelEnabled,
        image_max_bytes: imageMaxMiB * 1024 * 1024,
      });
      if (!res.data?.success || !res.data?.data) {
        showError(
          res.data?.message || t('Failed to update announcement settings'),
        );
        return;
      }
      const settings = res.data.data;
      setPanelEnabled(Boolean(settings.enabled));
      setImageMaxMiB(
        Math.max(
          1,
          Math.min(5, Math.round(settings.image_max_bytes / (1024 * 1024))),
        ),
      );
      showSuccess(t('Announcement settings saved'));
      refresh?.();
    } catch (error) {
      showError(error?.message || t('Failed to update announcement settings'));
    } finally {
      setAnnouncementSettingsSaving(false);
    }
  };

  const typeOptions = [
    { value: 'default', label: t('默认') },
    { value: 'ongoing', label: t('进行中') },
    { value: 'success', label: t('成功') },
    { value: 'warning', label: t('警告') },
    { value: 'error', label: t('错误') },
  ];

  const getTypeColor = (type) => {
    const colorMap = {
      default: 'grey',
      ongoing: 'blue',
      success: 'green',
      warning: 'orange',
      error: 'red',
    };
    return colorMap[type] || 'grey';
  };

  const columns = [
    {
      title: t('内容'),
      dataIndex: 'title',
      key: 'content',
      render: (title, record) => (
        <Tooltip content={record.content} position='topLeft' showArrow>
          <div
            style={{
              maxWidth: '300px',
              overflow: 'hidden',
              textOverflow: 'ellipsis',
              whiteSpace: 'nowrap',
            }}
          >
            {title ? <div className='font-medium'>{title}</div> : null}
            <div className='text-xs text-semi-color-text-2'>
              {record.content}
            </div>
          </div>
        </Tooltip>
      ),
    },
    {
      title: t('发布状态'),
      dataIndex: 'published',
      key: 'published',
      width: 100,
      render: (published) => (
        <Tag color={published === false ? 'grey' : 'green'} shape='circle'>
          {published === false ? t('草稿') : t('已发布')}
        </Tag>
      ),
    },
    {
      title: t('发布时间'),
      dataIndex: 'publishDate',
      key: 'publishDate',
      width: 180,
      render: (publishDate) => (
        <div>
          <div style={{ fontWeight: 'bold' }}>
            {getRelativeTime(publishDate)}
          </div>
          <div
            style={{
              fontSize: '12px',
              color: 'var(--semi-color-text-2)',
              marginTop: '2px',
            }}
          >
            {publishDate ? formatDateTimeString(new Date(publishDate)) : '-'}
          </div>
        </div>
      ),
    },
    {
      title: t('类型'),
      dataIndex: 'type',
      key: 'type',
      width: 100,
      render: (type) => (
        <Tag color={getTypeColor(type)} shape='circle'>
          {typeOptions.find((opt) => opt.value === type)?.label || type}
        </Tag>
      ),
    },
    {
      title: t('说明'),
      dataIndex: 'extra',
      key: 'extra',
      render: (text) => (
        <Tooltip content={text || '-'} showArrow>
          <div
            style={{
              maxWidth: '200px',
              overflow: 'hidden',
              textOverflow: 'ellipsis',
              whiteSpace: 'nowrap',
              color: 'var(--semi-color-text-2)',
            }}
          >
            {text || '-'}
          </div>
        </Tooltip>
      ),
    },
    {
      title: t('操作'),
      key: 'action',
      fixed: 'right',
      width: 150,
      render: (text, record) => (
        <Space>
          <Button
            icon={<Edit size={14} />}
            theme='light'
            type='tertiary'
            size='small'
            onClick={() => handleEditAnnouncement(record)}
          >
            {t('编辑')}
          </Button>
          <Button
            icon={<Trash2 size={14} />}
            type='danger'
            theme='light'
            size='small'
            onClick={() => handleDeleteAnnouncement(record)}
          >
            {t('删除')}
          </Button>
        </Space>
      ),
    },
  ];

  const updateOption = async (key, value) => {
    const res = await API.put('/api/option/', {
      key,
      value,
    });
    const { success, message } = res.data;
    if (success) {
      showSuccess(t('Announcements saved successfully'));
      if (refresh) refresh();
    } else {
      showError(message);
    }
  };

  const submitAnnouncements = async () => {
    try {
      setLoading(true);
      const announcementsJson = JSON.stringify(announcementsList);
      await updateOption('console_setting.announcements', announcementsJson);
      setHasChanges(false);
    } catch (error) {
      console.error('系统公告更新失败', error);
      showError(t('Failed to save announcements'));
    } finally {
      setLoading(false);
    }
  };

  const handleAddAnnouncement = () => {
    setEditingAnnouncement(null);
    setImageUploadError('');
    setAnnouncementForm({
      title: '',
      content: '',
      publishDate: new Date(),
      type: 'default',
      extra: '',
      imageUrl: '',
      popup: false,
      published: true,
    });
    setShowAnnouncementModal(true);
  };

  const handleEditAnnouncement = (announcement) => {
    setEditingAnnouncement(announcement);
    setImageUploadError('');
    setAnnouncementForm({
      title: announcement.title || '',
      content: announcement.content,
      publishDate: announcement.publishDate
        ? new Date(announcement.publishDate)
        : new Date(),
      type: announcement.type || 'default',
      extra: announcement.extra || '',
      imageUrl: announcement.imageUrl || '',
      popup: announcement.popup === true,
      published: announcement.published !== false,
    });
    setShowAnnouncementModal(true);
  };

  const handleDeleteAnnouncement = (announcement) => {
    setDeletingAnnouncement(announcement);
    setShowDeleteModal(true);
  };

  const confirmDeleteAnnouncement = () => {
    if (deletingAnnouncement) {
      const newList = announcementsList.filter(
        (item) => item.id !== deletingAnnouncement.id,
      );
      setAnnouncementsList(newList);
      setHasChanges(true);
      showSuccess(t('Announcement deleted. Click "Save Settings" to apply.'));
    }
    setShowDeleteModal(false);
    setDeletingAnnouncement(null);
  };

  const handleSaveAnnouncement = async () => {
    if (isUploadingImage) return;
    if (!announcementForm.content || !announcementForm.publishDate) {
      showError(t('Please complete all announcement fields'));
      return;
    }
    if (
      new TextEncoder().encode(announcementForm.content).byteLength >
      64 * 1024
    ) {
      showError(t('Announcement content must not exceed 64 KiB'));
      return;
    }

    try {
      setModalLoading(true);

      // 将publishDate转换为ISO字符串保存
      const formData = {
        ...announcementForm,
        publishDate: announcementForm.publishDate.toISOString(),
      };

      let newList;
      if (editingAnnouncement) {
        newList = announcementsList.map((item) =>
          item.id === editingAnnouncement.id ? { ...item, ...formData } : item,
        );
      } else {
        const newId =
          Math.max(...announcementsList.map((item) => item.id), 0) + 1;
        const newAnnouncement = {
          id: newId,
          ...formData,
        };
        newList = [...announcementsList, newAnnouncement];
      }

      setAnnouncementsList(newList);
      setHasChanges(true);
      setShowAnnouncementModal(false);
      showSuccess(
        editingAnnouncement
          ? t('Announcement updated. Click "Save Settings" to apply.')
          : t('Announcement added. Click "Save Settings" to apply.'),
      );
    } catch (error) {
      showError(`${t('操作失败')}: ${error.message}`);
    } finally {
      setModalLoading(false);
    }
  };

  const handleImageUpload = async (file) => {
    const supportedTypes = new Set([
      'image/png',
      'image/jpeg',
      'image/gif',
      'image/webp',
    ]);
    if (!supportedTypes.has(file.type)) {
      const message = t('公告图片必须是 PNG、JPEG、GIF 或 WebP');
      setImageUploadError(message);
      showError(message);
      return;
    }
    if (file.size > imageMaxMiB * 1024 * 1024) {
      const message = t('Announcement image exceeds the configured size limit');
      setImageUploadError(message);
      showError(message);
      return;
    }

    setIsUploadingImage(true);
    setImageUploadError('');
    try {
      const imageUrl = await uploadAnnouncementMedia(API, file);
      setAnnouncementForm((previous) => ({ ...previous, imageUrl }));
    } catch (error) {
      const message =
        error instanceof Error && error.message
          ? t(error.message)
          : t('公告图片上传失败');
      setImageUploadError(message);
      showError(message);
    } finally {
      setIsUploadingImage(false);
    }
  };

  const parseAnnouncements = (announcementsStr) => {
    if (!announcementsStr) {
      setAnnouncementsList([]);
      return;
    }

    try {
      const parsed = JSON.parse(announcementsStr);
      const list = Array.isArray(parsed) ? parsed : [];
      // 确保每个项目都有id
      const listWithIds = list.map((item, index) => ({
        id: item.id || index + 1,
        title: item.title,
        content: item.content,
        publishDate: item.publishDate,
        type: item.type || 'default',
        extra: item.extra,
        imageUrl: item.imageUrl,
        popup: item.popup === true,
        published: item.published !== false,
      }));
      setAnnouncementsList(listWithIds);
    } catch (error) {
      console.error('解析系统公告失败:', error);
      setAnnouncementsList([]);
    }
  };

  useEffect(() => {
    const annStr =
      options['console_setting.announcements'] ?? options.Announcements;
    if (annStr !== undefined) {
      parseAnnouncements(annStr);
    }
  }, [options['console_setting.announcements'], options.Announcements]);

  useEffect(() => {
    loadAnnouncementSettings();
  }, []);

  useEffect(() => {
    const enabledStr = options['console_setting.announcements_enabled'];
    if (!announcementSettingsLoaded && enabledStr !== undefined) {
      setPanelEnabled(enabledStr === 'true' || enabledStr === true);
    }
  }, [
    options['console_setting.announcements_enabled'],
    announcementSettingsLoaded,
  ]);

  const handleBatchDelete = () => {
    if (selectedRowKeys.length === 0) {
      showError(t('Please select items to delete'));
      return;
    }

    const newList = announcementsList.filter(
      (item) => !selectedRowKeys.includes(item.id),
    );
    setAnnouncementsList(newList);
    setSelectedRowKeys([]);
    setHasChanges(true);
    showSuccess(
      t('{{count}} announcements deleted. Click "Save Settings" to apply.', {
        count: selectedRowKeys.length,
      }),
    );
  };

  const renderHeader = () => (
    <div className='flex flex-col w-full'>
      <div className='mb-2'>
        <div className='flex items-center text-blue-500'>
          <Bell size={16} className='mr-2' />
          <Text>
            {t(
              '系统公告管理，可以发布系统通知和重要消息（最多100个，前端显示最新20条）',
            )}
          </Text>
        </div>
      </div>

      <Divider margin='12px' />

      <div className='mb-4 grid gap-3 rounded-md border border-semi-color-border bg-semi-color-fill-0 p-3 md:grid-cols-2 xl:grid-cols-4'>
        <div className='flex items-center gap-2'>
          <Switch
            checked={panelEnabled}
            onChange={setPanelEnabled}
            disabled={
              announcementSettingsLoading || !announcementSettingsLoaded
            }
          />
          <Text>{t('Enable announcements')}</Text>
        </div>
        <div className='flex min-w-0 items-center gap-2'>
          <Text className='shrink-0'>
            {t('Maximum announcement image size (MiB)')}
          </Text>
          <InputNumber
            value={imageMaxMiB}
            min={1}
            max={5}
            step={1}
            onChange={(value) =>
              setImageMaxMiB(Math.max(1, Math.min(5, Number(value) || 1)))
            }
            disabled={
              announcementSettingsLoading || !announcementSettingsLoaded
            }
            style={{ width: 90 }}
          />
        </div>
        <Button
          icon={<Save size={14} />}
          type='primary'
          theme='solid'
          loading={announcementSettingsSaving}
          disabled={!announcementSettingsLoaded}
          onClick={saveAnnouncementPolicy}
        >
          {t('Save announcement settings')}
        </Button>
      </div>

      {announcementSettingsError ? (
        <div
          className='mb-4 flex flex-wrap items-center justify-between gap-3 rounded-md border border-red-200 bg-red-50 p-3 dark:border-red-900 dark:bg-red-950/30'
          role='alert'
        >
          <Text type='danger' className='min-w-0 break-words'>
            {announcementSettingsError}
          </Text>
          <Button
            icon={<RefreshCw size={14} />}
            theme='light'
            type='danger'
            onClick={() => void loadAnnouncementSettings()}
          >
            {t('Retry')}
          </Button>
        </div>
      ) : null}

      <div className='flex flex-col md:flex-row justify-start items-center gap-4 w-full'>
        <div className='flex gap-2 w-full md:w-auto order-2 md:order-1'>
          <Button
            theme='light'
            type='primary'
            icon={<Plus size={14} />}
            className='w-full md:w-auto'
            onClick={handleAddAnnouncement}
          >
            {t('添加公告')}
          </Button>
          <Button
            icon={<Trash2 size={14} />}
            type='danger'
            theme='light'
            onClick={handleBatchDelete}
            disabled={selectedRowKeys.length === 0}
            className='w-full md:w-auto'
          >
            {t('批量删除')}{' '}
            {selectedRowKeys.length > 0 && `(${selectedRowKeys.length})`}
          </Button>
          <Button
            icon={<Save size={14} />}
            onClick={submitAnnouncements}
            loading={loading}
            disabled={!hasChanges}
            type='secondary'
            className='w-full md:w-auto'
          >
            {t('保存设置')}
          </Button>
        </div>
      </div>
    </div>
  );

  // 计算当前页显示的数据（按发布时间倒序排序，最新优先显示）
  const getCurrentPageData = () => {
    const sortedList = [...announcementsList].sort((a, b) => {
      const dateA = new Date(a.publishDate).getTime();
      const dateB = new Date(b.publishDate).getTime();
      return dateB - dateA; // 倒序，最新的排在前面
    });

    const startIndex = (currentPage - 1) * pageSize;
    const endIndex = startIndex + pageSize;
    return sortedList.slice(startIndex, endIndex);
  };

  const rowSelection = {
    selectedRowKeys,
    onChange: (selectedRowKeys, selectedRows) => {
      setSelectedRowKeys(selectedRowKeys);
    },
    onSelect: (record, selected, selectedRows) => {
      console.log(`选择行: ${selected}`, record);
    },
    onSelectAll: (selected, selectedRows) => {
      console.log(`全选: ${selected}`, selectedRows);
    },
    getCheckboxProps: (record) => ({
      disabled: false,
      name: record.id,
    }),
  };

  return (
    <>
      <Form.Section text={renderHeader()}>
        <Table
          columns={columns}
          dataSource={getCurrentPageData()}
          rowSelection={rowSelection}
          rowKey='id'
          scroll={{ x: 'max-content' }}
          pagination={{
            currentPage: currentPage,
            pageSize: pageSize,
            total: announcementsList.length,
            showSizeChanger: true,
            showQuickJumper: true,
            pageSizeOptions: ['5', '10', '20', '50'],
            onChange: (page, size) => {
              setCurrentPage(page);
              setPageSize(size);
            },
            onShowSizeChange: (current, size) => {
              setCurrentPage(1);
              setPageSize(size);
            },
          }}
          size='middle'
          loading={loading}
          empty={
            <Empty
              image={
                <IllustrationNoResult style={{ width: 150, height: 150 }} />
              }
              darkModeImage={
                <IllustrationNoResultDark style={{ width: 150, height: 150 }} />
              }
              description={t('暂无系统公告')}
              style={{ padding: 30 }}
            />
          }
          className='overflow-hidden'
        />
      </Form.Section>

      <Modal
        title={editingAnnouncement ? t('编辑公告') : t('添加公告')}
        visible={showAnnouncementModal}
        width={isMobile ? 'calc(100vw - 24px)' : 520}
        onOk={handleSaveAnnouncement}
        onCancel={() => {
          if (!isUploadingImage) setShowAnnouncementModal(false);
        }}
        okText={t('保存')}
        cancelText={t('取消')}
        confirmLoading={modalLoading}
        okButtonProps={{ disabled: isUploadingImage }}
      >
        <Form
          layout='vertical'
          initValues={announcementForm}
          key={editingAnnouncement ? editingAnnouncement.id : 'new'}
          getFormApi={(api) => (formApiRef.current = api)}
        >
          <Form.Input
            field='title'
            label={t('公告标题')}
            placeholder={t('可选，填写简洁的公告标题')}
            maxLength={200}
            onChange={(value) =>
              setAnnouncementForm({ ...announcementForm, title: value })
            }
          />
          <Form.TextArea
            field='content'
            label={t('公告内容')}
            placeholder={t('请输入公告内容（支持 Markdown/HTML）')}
            rows={3}
            rules={[{ required: true, message: t('请输入公告内容') }]}
            onChange={(value) =>
              setAnnouncementForm({ ...announcementForm, content: value })
            }
          />
          <div className='mb-4 rounded-md border border-semi-color-border p-3'>
            <div className='flex items-center gap-2'>
              <Form.Switch
                field='published'
                onChange={(published) =>
                  setAnnouncementForm({ ...announcementForm, published })
                }
              />
              <Text>{t('发布此公告')}</Text>
            </div>
            <div className='mt-1 text-xs text-semi-color-text-2'>
              {t('未发布的公告仅管理员可见。')}
            </div>
          </div>
          <div className='mb-4 flex items-center gap-2'>
            <Switch
              checked={announcementForm.popup}
              onChange={(popup) =>
                setAnnouncementForm({ ...announcementForm, popup })
              }
            />
            <Text>{t('此公告以弹窗形式展示')}</Text>
          </div>
          <Button
            theme='light'
            type='tertiary'
            size='small'
            icon={<Maximize2 size={14} />}
            style={{ marginBottom: 16 }}
            onClick={() => setShowContentModal(true)}
          >
            {t('放大编辑')}
          </Button>
          <div className='mb-4'>
            <div className='mb-2 text-sm font-medium'>
              {t('公告图片（可选）')}
            </div>
            <input
              ref={imageInputRef}
              type='file'
              accept='image/png,image/jpeg,image/gif,image/webp'
              className='hidden'
              aria-label={t('添加公告图片')}
              onChange={(event) => {
                const file = event.target.files?.[0];
                event.target.value = '';
                if (file) void handleImageUpload(file);
              }}
            />
            {announcementForm.imageUrl &&
            isSafeAnnouncementImageURL(announcementForm.imageUrl) ? (
              <div className='relative h-28 w-28 aspect-square overflow-hidden rounded-md border'>
                <img
                  src={announcementForm.imageUrl}
                  alt={t('公告图片')}
                  width={112}
                  height={112}
                  className='h-full w-full object-contain'
                  loading='eager'
                  referrerPolicy='no-referrer'
                />
                <div className='absolute right-1 top-1 flex gap-1'>
                  <Button
                    theme='solid'
                    type='tertiary'
                    size='small'
                    icon={<RefreshCw size={14} />}
                    aria-label={t('替换公告图片')}
                    title={t('替换公告图片')}
                    disabled={isUploadingImage}
                    onClick={() => imageInputRef.current?.click()}
                  />
                  <Button
                    theme='solid'
                    type='danger'
                    size='small'
                    icon={<Trash2 size={14} />}
                    aria-label={t('移除公告图片')}
                    title={t('移除公告图片')}
                    disabled={isUploadingImage}
                    onClick={() => {
                      setAnnouncementForm((previous) => ({
                        ...previous,
                        imageUrl: '',
                      }));
                      setImageUploadError('');
                    }}
                  />
                </div>
              </div>
            ) : (
              <button
                type='button'
                className='flex h-28 w-28 aspect-square flex-col items-center justify-center gap-2 rounded-md border border-dashed text-xs text-semi-color-text-2 hover:bg-semi-color-fill-0 disabled:opacity-60'
                aria-label={t('添加公告图片')}
                title={t('添加公告图片')}
                disabled={isUploadingImage}
                onClick={() => imageInputRef.current?.click()}
              >
                {isUploadingImage ? (
                  <LoaderCircle size={24} className='animate-spin' />
                ) : (
                  <ImagePlus size={24} />
                )}
                <span>{isUploadingImage ? t('上传中...') : t('添加图片')}</span>
              </button>
            )}
            {imageUploadError ? (
              <div className='mt-2 text-sm text-red-500' role='alert'>
                {imageUploadError}
              </div>
            ) : null}
            <div className='mt-2 text-xs text-semi-color-text-2'>
              {t('PNG, JPEG, GIF, or WebP. Maximum {{size}} MiB.', {
                size: imageMaxMiB,
              })}
            </div>
          </div>
          <Form.DatePicker
            field='publishDate'
            label={t('发布日期')}
            type='dateTime'
            rules={[{ required: true, message: t('请选择发布日期') }]}
            onChange={(value) =>
              setAnnouncementForm({ ...announcementForm, publishDate: value })
            }
          />
          <Form.Select
            field='type'
            label={t('公告类型')}
            optionList={typeOptions}
            onChange={(value) =>
              setAnnouncementForm({ ...announcementForm, type: value })
            }
          />
          <Form.Input
            field='extra'
            label={t('说明信息')}
            placeholder={t('可选，公告的补充说明')}
            onChange={(value) =>
              setAnnouncementForm({ ...announcementForm, extra: value })
            }
          />
        </Form>
      </Modal>

      <Modal
        title={t('确认删除')}
        visible={showDeleteModal}
        onOk={confirmDeleteAnnouncement}
        onCancel={() => {
          setShowDeleteModal(false);
          setDeletingAnnouncement(null);
        }}
        okText={t('确认删除')}
        cancelText={t('取消')}
        type='warning'
        okButtonProps={{
          type: 'danger',
          theme: 'solid',
        }}
      >
        <Text>{t('确定要删除此公告吗？')}</Text>
      </Modal>

      {/* 公告内容放大编辑 Modal */}
      <Modal
        title={t('编辑公告内容')}
        visible={showContentModal}
        onOk={() => {
          // 将内容同步到表单
          if (formApiRef.current) {
            formApiRef.current.setValue('content', announcementForm.content);
          }
          setShowContentModal(false);
        }}
        onCancel={() => setShowContentModal(false)}
        okText={t('确定')}
        cancelText={t('取消')}
        width={isMobile ? 'calc(100vw - 24px)' : 800}
      >
        <TextArea
          value={announcementForm.content}
          placeholder={t('请输入公告内容（支持 Markdown/HTML）')}
          rows={15}
          style={{ width: '100%' }}
          onChange={(value) =>
            setAnnouncementForm({ ...announcementForm, content: value })
          }
        />
      </Modal>
    </>
  );
};

export default SettingsAnnouncements;
