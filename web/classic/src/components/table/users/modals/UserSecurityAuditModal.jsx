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

import React, { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Banner,
  Button,
  Empty,
  Modal,
  Pagination,
  Space,
  Spin,
  Tag,
  Typography,
} from '@douyinfe/semi-ui';
import {
  IllustrationFailure,
  IllustrationFailureDark,
  IllustrationNoResult,
  IllustrationNoResultDark,
} from '@douyinfe/semi-illustrations';
import { RefreshCw, ShieldCheck, ShieldOff } from 'lucide-react';

import { API } from '../../../../helpers';
import { useIsMobile } from '../../../../hooks/common/useIsMobile';
import CardTable from '../../../common/ui/CardTable';
import IPAuditRangeControls from '../../usage-logs/IPAuditRangeControls';
import {
  buildUserSecurityAuditURL,
  formatIPAuditTimestamp,
  getIPAuditErrorDescriptionKey,
  getIPAuditPageCorrection,
  getProxyWarningPresentation,
  getUserSecurityAuditPageCorrection,
  getUserSecurityAuditRequestView,
  hasUserSecurityAuditActivity,
  normalizeUserSecurityAudit,
} from '../../usage-logs/ip-audit-state';

const timelineKindKeys = {
  registration: '注册',
  api_request: 'API 调用',
  api_error: 'API 错误',
  login_success: '登录成功',
  login_failure: '登录失败',
  logout: '退出',
};

const initialQuery = {
  range: '30d',
  view: 'risk',
  startTimestamp: 0,
  endTimestamp: 0,
  page: 1,
  pageSize: 10,
  relatedPage: 1,
  relatedPageSize: 20,
};

const formatTimelineKind = (kind, t) => {
  if (timelineKindKeys[kind]) return t(timelineKindKeys[kind]);
  if (kind?.startsWith('activity_')) return t('其他活动');
  return kind || '-';
};

const SecurityMetric = ({ label, value, danger = false, mono = false }) => (
  <div className='min-w-0 border-b border-semi-color-border px-1 py-3 sm:border-b-0 sm:border-r sm:px-4 last:border-0'>
    <dt className='text-xs text-semi-color-text-2'>{label}</dt>
    <dd
      className={`mt-1 break-all text-sm font-semibold ${danger ? 'text-semi-color-danger' : 'text-semi-color-text-0'} ${mono ? 'font-mono' : ''}`}
    >
      {value ?? '-'}
    </dd>
  </div>
);

const UserSecurityAuditModal = ({ visible, onCancel, user, t }) => {
  const { i18n } = useTranslation();
  const locale = i18n.resolvedLanguage || i18n.language;
  const isMobile = useIsMobile();
  const [query, setQuery] = useState(initialQuery);
  const [requestState, setRequestState] = useState({
    requestURL: '',
    status: 'idle',
    data: null,
    error: '',
  });
  const [reloadToken, setReloadToken] = useState(0);
  const requestURL = useMemo(
    () => (user?.id ? buildUserSecurityAuditURL(user.id, query) : ''),
    [
      user?.id,
      query.range,
      query.view,
      query.startTimestamp,
      query.endTimestamp,
      query.page,
      query.pageSize,
      query.relatedPage,
      query.relatedPageSize,
    ],
  );

  useEffect(() => {
    if (!visible || !requestURL) return undefined;
    let disposed = false;
    setRequestState({
      requestURL,
      status: 'loading',
      data: null,
      error: '',
    });

    API.get(requestURL, { skipErrorHandler: true })
      .then((response) => {
        if (disposed) return;
        if (!response.data?.success) {
          setRequestState({
            requestURL,
            status: 'error',
            data: null,
            error: response.data?.message || '用户安全审计加载失败',
          });
          return;
        }
        const nextData = normalizeUserSecurityAudit(response.data.data);
        const pageCorrection = getUserSecurityAuditPageCorrection(nextData);
        const relatedPageCorrection = nextData.is_root
          ? getIPAuditPageCorrection({
              items: nextData.related_users,
              total: nextData.related_total,
              page: nextData.related_page,
              page_size: nextData.related_page_size,
            })
          : null;
        if (pageCorrection !== null || relatedPageCorrection !== null) {
          setQuery((current) => ({
            ...current,
            page: pageCorrection ?? current.page,
            relatedPage: relatedPageCorrection ?? current.relatedPage,
          }));
          return;
        }
        setRequestState({
          requestURL,
          status: 'success',
          data: nextData,
          error: '',
        });
      })
      .catch((reason) => {
        if (disposed) return;
        setRequestState({
          requestURL,
          status: 'error',
          data: null,
          error:
            reason?.response?.data?.message ||
            reason?.message ||
            '用户安全审计加载失败',
        });
      });

    return () => {
      disposed = true;
    };
  }, [visible, requestURL, reloadToken]);

  const { data, error, loading } = getUserSecurityAuditRequestView(
    requestURL,
    requestState,
  );

  const handleQueryChange = (patch) => {
    setQuery((current) => ({
      ...current,
      ...patch,
      page: 1,
      relatedPage: 1,
    }));
  };

  const timelineColumns = useMemo(
    () => [
      {
        key: 'created_at',
        title: t('时间'),
        dataIndex: 'created_at',
        width: 170,
        render: (value) => formatIPAuditTimestamp(value, locale),
      },
      {
        key: 'ip',
        title: t('IP 地址'),
        dataIndex: 'ip',
        width: 190,
        render: (value) => (
          <Typography.Text
            code
            className='block min-w-0 whitespace-normal break-all [overflow-wrap:anywhere]'
          >
            {value || '-'}
          </Typography.Text>
        ),
      },
      {
        key: 'kind',
        title: t('事件类型'),
        dataIndex: 'kind',
        width: 130,
        render: (value) => (
          <Tag color='blue'>{formatTimelineKind(value, t)}</Tag>
        ),
      },
      {
        key: 'request_id',
        title: 'Request ID',
        dataIndex: 'request_id',
        render: (value) => value || '-',
      },
    ],
    [locale, t],
  );

  const proxyWarning = getProxyWarningPresentation(data?.proxy_warning);
  const summary = data?.summary;
  const hasActivity = hasUserSecurityAuditActivity(summary);

  let content = null;
  if (loading && !data) {
    content = (
      <div className='flex min-h-[280px] items-center justify-center'>
        <Spin size='large' />
      </div>
    );
  } else if (error) {
    content = (
      <Empty
        image={<IllustrationFailure style={{ width: 140, height: 140 }} />}
        darkModeImage={
          <IllustrationFailureDark style={{ width: 140, height: 140 }} />
        }
        title={t('用户安全审计加载失败')}
        description={t(getIPAuditErrorDescriptionKey(error))}
        style={{ padding: 28 }}
      >
        <Button
          icon={<RefreshCw size={16} />}
          onClick={() => setReloadToken((value) => value + 1)}
        >
          {t('重试')}
        </Button>
      </Empty>
    );
  } else if (data && data.enabled === false) {
    content = (
      <Empty
        image={<ShieldOff size={56} strokeWidth={1.5} />}
        title={t('IP 审计已关闭')}
        description={t('管理员已关闭 IP 安全事件采集。')}
        style={{ padding: 28 }}
      />
    );
  } else if (data) {
    content = (
      <div className='flex flex-col gap-4' aria-live='polite'>
        {proxyWarning ? (
          <Banner
            type={proxyWarning.tone}
            title={t(proxyWarning.titleKey)}
            description={t(proxyWarning.descriptionKey)}
            closeIcon={null}
            className='!rounded-lg'
          />
        ) : null}

        <div className='flex flex-wrap items-center gap-2'>
          <Tag color={data.is_root ? 'violet' : 'blue'} shape='circle'>
            {data.is_root ? t('Root 完整视图') : t('Admin 掩码视图')}
          </Tag>
          <Tag color='grey' shape='circle'>
            {t('用户')}: {data.username || user?.username || `#${user?.id}`}
          </Tag>
          <Button
            className='ml-auto'
            type='tertiary'
            size='small'
            icon={<RefreshCw size={15} />}
            loading={loading}
            onClick={() => setReloadToken((value) => value + 1)}
          >
            {t('刷新')}
          </Button>
        </div>

        <div className='flex min-w-0 items-start gap-2 text-xs text-semi-color-text-2'>
          <ShieldCheck className='mt-0.5 shrink-0' size={14} />
          <span>
            {t(
              'API 调用归属所用 API Key 的所有者；登录、登出和注册事件归属直接执行认证行为的账号。',
            )}
          </span>
        </div>

        {!hasActivity ? (
          <Empty
            image={<IllustrationNoResult style={{ width: 120, height: 120 }} />}
            darkModeImage={
              <IllustrationNoResultDark style={{ width: 120, height: 120 }} />
            }
            title={t('暂无安全审计活动')}
            description={t('当前时间范围内没有注册、登录或 API 事件。')}
          />
        ) : (
          <dl className='grid grid-cols-1 overflow-hidden border-y border-semi-color-border sm:grid-cols-3'>
            <SecurityMetric
              label={t('注册 IP')}
              value={summary.registration_ip || '-'}
              mono
            />
            <SecurityMetric label={t('退出')} value={summary.logout_count} />
            <SecurityMetric
              label={t('最近 IP')}
              value={summary.recent_ip || '-'}
              mono
            />
            <SecurityMetric
              label={t('最近活动')}
              value={
                summary.last_activity_at
                  ? formatIPAuditTimestamp(summary.last_activity_at, locale)
                  : '-'
              }
            />
            <SecurityMetric
              label={t('API 调用')}
              value={summary.api_request_count}
            />
            <SecurityMetric
              label={t('错误')}
              value={summary.error_count}
              danger={summary.error_count > 0}
            />
            <SecurityMetric
              label={t('近期异常')}
              value={summary.anomaly_count}
              danger={summary.anomaly_count > 0}
            />
            <SecurityMetric
              label={t('登录成功')}
              value={summary.login_success_count}
            />
            <SecurityMetric
              label={t('登录失败')}
              value={summary.login_failure_count}
              danger={summary.login_failure_count > 0}
            />
            <SecurityMetric
              label={t('相关账号数')}
              value={summary.related_user_count}
            />
          </dl>
        )}

        {data.is_root ? (
          <section className='border-t border-semi-color-border pt-4'>
            <Typography.Title heading={6} className='!mb-2'>
              {t('相关账号')}
            </Typography.Title>
            {data.related_users.length ? (
              <div className='divide-y divide-semi-color-border border-y border-semi-color-border'>
                {data.related_users.map((relatedUser) => (
                  <div
                    key={relatedUser.user_id}
                    className='flex min-w-0 flex-col gap-2 py-3 sm:flex-row sm:items-start sm:justify-between'
                  >
                    <div className='min-w-0'>
                      <Typography.Text strong className='block break-all'>
                        {relatedUser.username || `#${relatedUser.user_id}`}
                      </Typography.Text>
                      <Typography.Text type='tertiary' size='small'>
                        {t('最近活动')}:{' '}
                        {formatIPAuditTimestamp(
                          relatedUser.last_seen_at,
                          locale,
                        )}
                      </Typography.Text>
                    </div>
                    <Space wrap spacing={4} className='sm:justify-end'>
                      <Tag color='violet' shape='circle'>
                        {t('共享 IP')}: {relatedUser.shared_ip_count}
                      </Tag>
                      <Tag color='orange' shape='circle'>
                        {t('注册')}: {relatedUser.registration_count}
                      </Tag>
                      <Tag color='cyan' shape='circle'>
                        {t('API 调用')}: {relatedUser.api_call_count}
                      </Tag>
                      <Tag color='blue' shape='circle'>
                        {t('认证活动')}: {relatedUser.authentication_count}
                      </Tag>
                      <Tag
                        color={relatedUser.anomaly_count > 0 ? 'red' : 'grey'}
                        shape='circle'
                      >
                        {t('异常')}: {relatedUser.anomaly_count}
                      </Tag>
                    </Space>
                  </div>
                ))}
              </div>
            ) : (
              <Typography.Text type='tertiary'>
                {t('无相关账号')}
              </Typography.Text>
            )}
            {data.related_total > 0 ? (
              <div className='mt-3 flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between'>
                <Typography.Text type='tertiary'>
                  {t('总计')}: {data.related_total}
                </Typography.Text>
                <Pagination
                  currentPage={data.related_page}
                  pageSize={data.related_page_size}
                  total={data.related_total}
                  pageSizeOpts={[10, 20, 50, 100]}
                  showSizeChanger
                  size={isMobile ? 'small' : 'default'}
                  onPageChange={(relatedPage) =>
                    setQuery((current) => ({ ...current, relatedPage }))
                  }
                  onPageSizeChange={(relatedPageSize) =>
                    setQuery((current) => ({
                      ...current,
                      relatedPage: 1,
                      relatedPageSize,
                    }))
                  }
                />
              </div>
            ) : null}
          </section>
        ) : null}

        {data.is_root ? (
          <section className='border-t border-semi-color-border pt-4'>
            <Typography.Title heading={6} className='!mb-2'>
              {t('IP 活动时间线')}
            </Typography.Title>
            <CardTable
              columns={timelineColumns}
              dataSource={data.timeline}
              rowKey={(record) =>
                `${record.created_at}-${record.kind}-${record.ip}-${record.request_id}`
              }
              loading={loading}
              scroll={{ x: 'max-content' }}
              size='small'
              hidePagination
              empty={
                <Empty
                  title={t('暂无时间线记录')}
                  description={t('当前页没有安全事件。')}
                  style={{ padding: 20 }}
                />
              }
            />
            {data.total > 0 ? (
              <div className='mt-3 flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between'>
                <Typography.Text type='tertiary'>
                  {t('总计')}: {data.total}
                </Typography.Text>
                <Pagination
                  currentPage={data.page}
                  pageSize={data.page_size}
                  total={data.total}
                  pageSizeOpts={[10, 20, 50, 100]}
                  showSizeChanger
                  size={isMobile ? 'small' : 'default'}
                  onPageChange={(page) =>
                    setQuery((current) => ({ ...current, page }))
                  }
                  onPageSizeChange={(pageSize) =>
                    setQuery((current) => ({ ...current, page: 1, pageSize }))
                  }
                />
              </div>
            ) : null}
          </section>
        ) : null}
      </div>
    );
  }

  return (
    <Modal
      title={
        <Space>
          <ShieldCheck size={18} />
          <span>{t('安全与 IP 审计')}</span>
        </Space>
      }
      visible={visible}
      onCancel={onCancel}
      footer={null}
      width={isMobile ? 'calc(100vw - 24px)' : 920}
      bodyStyle={{ maxHeight: '75vh', overflowY: 'auto' }}
      closeOnEsc
      maskClosable
    >
      <div className='mb-4'>
        <IPAuditRangeControls
          query={query}
          onChange={handleQueryChange}
          disabled={loading || data?.enabled === false}
          t={t}
        />
      </div>
      {content}
    </Modal>
  );
};

export default UserSecurityAuditModal;
