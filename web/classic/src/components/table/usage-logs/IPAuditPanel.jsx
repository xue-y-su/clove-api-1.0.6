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

import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Banner,
  Button,
  Descriptions,
  Empty,
  Pagination,
  SideSheet,
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
import { RefreshCw, ShieldOff } from 'lucide-react';

import { API } from '../../../helpers';
import { createCardProPagination } from '../../../helpers/utils';
import { useIsMobile } from '../../../hooks/common/useIsMobile';
import CardPro from '../../common/ui/CardPro';
import CardTable from '../../common/ui/CardTable';
import IPAuditRangeControls from './IPAuditRangeControls';
import { getIPAuditColumns } from './UsageLogsColumnDefs';
import {
  buildUsageLogSearchParams,
  buildIPAuditDetailURL,
  buildIPAuditURL,
  formatIPAuditTimestamp,
  getIPAuditErrorDescriptionKey,
  getIPAuditPageCorrection,
  getIPAuditViewState,
  getProxyWarningPresentation,
  normalizeIPAuditDetail,
  normalizeIPAuditPage,
  readIPAuditQuery,
  updateIPAuditSearchParams,
} from './ip-audit-state';

const timelineKindKeys = {
  api_request: 'API 调用',
  api_error: 'API 错误',
  login_success: '登录成功',
  login_failure: '登录失败',
  logout: '退出',
  registration: '注册',
};

const createEmptyPage = (query) => ({
  enabled: true,
  items: [],
  total: 0,
  page: query.page,
  page_size: query.pageSize,
  range: query.range,
  start_timestamp: query.startTimestamp,
  end_timestamp: query.endTimestamp,
  is_root: false,
  can_view_details: false,
  proxy_warning: '',
});

const formatTimelineKind = (kind, t) => {
  if (timelineKindKeys[kind]) {
    return t(timelineKindKeys[kind]);
  }
  if (kind?.startsWith('activity_')) {
    return t('其他活动');
  }
  return kind || '-';
};

const IPAuditDetailRow = ({ ip, query, locale, t, onRequestIDSearch }) => {
  const [detailQuery, setDetailQuery] = useState({
    userPage: 1,
    userPageSize: 20,
    timelinePage: 1,
    timelinePageSize: 20,
  });
  const requestURL = useMemo(
    () => buildIPAuditDetailURL(ip, { ...query, ...detailQuery }),
    [
      ip,
      query.range,
      query.view,
      query.startTimestamp,
      query.endTimestamp,
      detailQuery.userPage,
      detailQuery.userPageSize,
      detailQuery.timelinePage,
      detailQuery.timelinePageSize,
    ],
  );
  const [state, setState] = useState({
    data: null,
    loading: true,
    error: '',
  });
  const [reloadToken, setReloadToken] = useState(0);

  useEffect(() => {
    let disposed = false;
    setState({ data: null, loading: true, error: '' });
    API.get(requestURL, { skipErrorHandler: true })
      .then((response) => {
        if (disposed) return;
        if (!response.data?.success) {
          setState({
            data: null,
            loading: false,
            error: response.data?.message || 'IP 审计加载失败',
          });
          return;
        }
        setState({
          data: normalizeIPAuditDetail(response.data.data),
          loading: false,
          error: '',
        });
      })
      .catch((reason) => {
        if (disposed) return;
        setState({
          data: null,
          loading: false,
          error:
            reason?.response?.data?.message ||
            reason?.message ||
            'IP 审计加载失败',
        });
      });
    return () => {
      disposed = true;
    };
  }, [reloadToken, requestURL]);

  if (state.loading) {
    return (
      <div className='flex min-h-20 items-center justify-center'>
        <Spin tip={t('加载中...')} />
      </div>
    );
  }
  if (state.error) {
    return (
      <Banner
        type='danger'
        title={t('IP 审计加载失败')}
        description={t(getIPAuditErrorDescriptionKey(state.error))}
        closeIcon={null}
        action={
          <Button
            size='small'
            icon={<RefreshCw size={15} />}
            onClick={() => setReloadToken((value) => value + 1)}
          >
            {t('重试')}
          </Button>
        }
      />
    );
  }

  const users = state.data?.users || [];
  const timeline = state.data?.timeline || [];
  return (
    <div className='space-y-3'>
      <section className='space-y-2'>
        <div className='flex flex-wrap items-center gap-2'>
          <Typography.Text type='tertiary'>{t('关联账号')}</Typography.Text>
          <Tag color='grey' shape='circle'>
            {t('总计')}: {state.data.user_total}
          </Tag>
        </div>
        <div className='flex flex-wrap items-center gap-2'>
          {users.length ? (
            users.map((user) => (
              <Tag key={user.user_id} color='violet' shape='circle'>
                {user.username || `#${user.user_id}`}
              </Tag>
            ))
          ) : (
            <Typography.Text type='tertiary'>{t('暂无数据')}</Typography.Text>
          )}
        </div>
        {state.data.user_total > 0 ? (
          <Pagination
            currentPage={state.data.user_page}
            pageSize={state.data.user_page_size}
            total={state.data.user_total}
            pageSizeOpts={[10, 20, 50, 100]}
            showSizeChanger
            size='small'
            onPageChange={(userPage) =>
              setDetailQuery((current) => ({ ...current, userPage }))
            }
            onPageSizeChange={(userPageSize) =>
              setDetailQuery((current) => ({
                ...current,
                userPage: 1,
                userPageSize,
              }))
            }
          />
        ) : null}
      </section>
      <Typography.Text type='tertiary' size='small'>
        {t(
          'API 调用归属所用 API Key 的所有者；登录、登出和注册事件归属直接执行认证行为的账号。',
        )}
      </Typography.Text>
      <section className='space-y-2'>
        <div className='flex flex-wrap items-center gap-2'>
          <Typography.Text type='tertiary'>
            {t('IP 活动时间线')}
          </Typography.Text>
          <Tag color='grey' shape='circle'>
            {t('总计')}: {state.data.timeline_total}
          </Tag>
        </div>
        {timeline.length ? (
          <Descriptions
            row
            data={timeline.map((event, index) => ({
              key: `${formatIPAuditTimestamp(event.created_at, locale)} · ${formatTimelineKind(event.kind, t)}`,
              value: (
                <Space wrap spacing={4}>
                  <span>{event.username || `#${event.user_id}`}</span>
                  {event.request_id ? (
                    <Button
                      theme='borderless'
                      type='tertiary'
                      size='small'
                      className='!h-auto !px-0 font-mono'
                      onClick={() => onRequestIDSearch(event.request_id)}
                    >
                      {event.request_id}
                    </Button>
                  ) : null}
                </Space>
              ),
              hidden: false,
              index,
            }))}
          />
        ) : (
          <Typography.Text type='tertiary'>{t('暂无数据')}</Typography.Text>
        )}
        {state.data.timeline_total > 0 ? (
          <Pagination
            currentPage={state.data.timeline_page}
            pageSize={state.data.timeline_page_size}
            total={state.data.timeline_total}
            pageSizeOpts={[10, 20, 50, 100]}
            showSizeChanger
            size='small'
            onPageChange={(timelinePage) =>
              setDetailQuery((current) => ({ ...current, timelinePage }))
            }
            onPageSizeChange={(timelinePageSize) =>
              setDetailQuery((current) => ({
                ...current,
                timelinePage: 1,
                timelinePageSize,
              }))
            }
          />
        ) : null}
      </section>
    </div>
  );
};

const IPAuditPanel = ({ searchParams, setSearchParams, t }) => {
  const { i18n } = useTranslation();
  const locale = i18n.resolvedLanguage || i18n.language;
  const isMobile = useIsMobile();
  const query = useMemo(() => readIPAuditQuery(searchParams), [searchParams]);
  const requestURL = useMemo(
    () => buildIPAuditURL('/api/log/ip-audit', query),
    [
      query.range,
      query.view,
      query.startTimestamp,
      query.endTimestamp,
      query.page,
      query.pageSize,
    ],
  );
  const [page, setPage] = useState(() => createEmptyPage(query));
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [reloadToken, setReloadToken] = useState(0);
  const [selectedIP, setSelectedIP] = useState('');
  const handleRequestIDSearch = useCallback(
    (requestID) => {
      const nextSearchParams = buildUsageLogSearchParams(
        searchParams,
        requestID,
      );
      if (!nextSearchParams) return;
      setSearchParams(nextSearchParams);
    },
    [searchParams, setSearchParams],
  );

  useEffect(() => {
    let disposed = false;
    let correctingPage = false;
    setLoading(true);
    setError('');

    API.get(requestURL, { skipErrorHandler: true })
      .then((response) => {
        if (disposed) return;
        if (!response.data?.success) {
          setError(response.data?.message || 'IP 审计加载失败');
          return;
        }
        const nextPage = normalizeIPAuditPage(response.data.data);
        const correctedPage = getIPAuditPageCorrection(nextPage);
        if (correctedPage !== null) {
          correctingPage = true;
          const nextSearchParams = updateIPAuditSearchParams(searchParams, {
            page: correctedPage,
          });
          nextSearchParams.set('tab', 'ip-audit');
          setSearchParams(nextSearchParams, { replace: true });
          return;
        }
        setPage(nextPage);
      })
      .catch((reason) => {
        if (disposed) return;
        setError(
          reason?.response?.data?.message ||
            reason?.message ||
            'IP 审计加载失败',
        );
      })
      .finally(() => {
        if (!disposed && !correctingPage) setLoading(false);
      });

    return () => {
      disposed = true;
    };
  }, [requestURL, reloadToken, searchParams, setSearchParams]);

  const updateQuery = useCallback(
    (patch) => {
      const next = updateIPAuditSearchParams(searchParams, patch);
      next.set('tab', 'ip-audit');
      setSearchParams(next, { replace: true });
    },
    [searchParams, setSearchParams],
  );

  useEffect(() => {
    if (selectedIP && !page.items.some((item) => item.ip === selectedIP)) {
      setSelectedIP('');
    }
  }, [page.items, selectedIP]);

  const columns = useMemo(() => {
    const baseColumns = getIPAuditColumns({ t, locale });
    if (!page.can_view_details) {
      return baseColumns;
    }
    return [
      ...baseColumns,
      {
        key: 'details',
        title: t('操作'),
        fixed: isMobile ? undefined : 'right',
        width: 150,
        render: (_, record) => (
          <Button
            type='tertiary'
            size='small'
            onClick={() => setSelectedIP(record.ip)}
          >
            {t('查看 IP 详情')}
          </Button>
        ),
      },
    ];
  }, [isMobile, locale, page.can_view_details, t]);
  const viewState = getIPAuditViewState({
    loading,
    error,
    enabled: page.enabled,
    items: page.items,
  });
  const proxyWarning = getProxyWarningPresentation(page.proxy_warning);

  const emptyState = (
    <Empty
      image={<IllustrationNoResult style={{ width: 150, height: 150 }} />}
      darkModeImage={
        <IllustrationNoResultDark style={{ width: 150, height: 150 }} />
      }
      title={t('暂无 IP 审计记录')}
      description={t('当前时间范围内没有带 IP 的审计事件。')}
      style={{ padding: 30 }}
    />
  );

  const content =
    viewState === 'loading' ? (
      <div className='flex min-h-[240px] items-center justify-center'>
        <Spin size='large' tip={t('加载中...')} />
      </div>
    ) : viewState === 'error' ? (
      <Empty
        image={<IllustrationFailure style={{ width: 150, height: 150 }} />}
        darkModeImage={
          <IllustrationFailureDark style={{ width: 150, height: 150 }} />
        }
        title={t('IP 审计加载失败')}
        description={t(getIPAuditErrorDescriptionKey(error))}
        style={{ padding: 30 }}
      >
        <Button
          icon={<RefreshCw size={16} />}
          onClick={() => setReloadToken((value) => value + 1)}
        >
          {t('重试')}
        </Button>
      </Empty>
    ) : viewState === 'disabled' ? (
      <Empty
        image={<ShieldOff size={56} strokeWidth={1.5} />}
        title={t('IP 审计已关闭')}
        description={t('管理员已关闭 IP 安全事件采集。')}
        style={{ padding: 30 }}
      />
    ) : (
      <CardTable
        columns={columns}
        dataSource={page.items}
        rowKey='ip'
        scroll={{ x: 1320 }}
        size='small'
        empty={emptyState}
        hidePagination
      />
    );

  return (
    <>
      <CardPro
        type='type2'
        statsArea={
          <div className='flex min-w-0 flex-wrap items-center gap-2'>
            <Typography.Text strong>{t('IP 审计')}</Typography.Text>
            {viewState === 'disabled' ? (
              <Tag color='grey' shape='circle'>
                {t('IP 审计已关闭')}
              </Tag>
            ) : viewState === 'success' ? (
              <Tag color={page.is_root ? 'violet' : 'blue'} shape='circle'>
                {page.is_root ? t('Root 完整视图') : t('Admin 掩码视图')}
              </Tag>
            ) : null}
            <Tag color='grey' shape='circle'>
              {t('总计')}: {page.total}
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
        }
        searchArea={
          <IPAuditRangeControls
            query={query}
            onChange={updateQuery}
            disabled={loading || page.enabled === false}
            t={t}
          />
        }
        paginationArea={
          viewState === 'success'
            ? createCardProPagination({
                currentPage: page.page,
                pageSize: page.page_size,
                total: page.total,
                onPageChange: (nextPage) => updateQuery({ page: nextPage }),
                onPageSizeChange: (nextPageSize) =>
                  updateQuery({ pageSize: nextPageSize }),
                isMobile,
                t,
              })
            : null
        }
        t={t}
      >
        {page.enabled !== false && proxyWarning ? (
          <Banner
            type={proxyWarning.tone}
            title={t(proxyWarning.titleKey)}
            description={t(proxyWarning.descriptionKey)}
            closeIcon={null}
            className='mb-3 !rounded-lg'
          />
        ) : null}
        {page.enabled !== false &&
        viewState !== 'loading' &&
        viewState !== 'error' &&
        page.sync?.ready === false ? (
          <Banner
            type='info'
            title={t('IP 审计数据同步中')}
            description={`${t('当前显示已完成同步的数据')} · ${t('已处理历史日志')}: ${page.sync.processed_logs.toLocaleString()}`}
            closeIcon={null}
            className='mb-3 !rounded-lg'
          />
        ) : null}
        {content}
      </CardPro>
      <SideSheet
        visible={Boolean(selectedIP)}
        placement='right'
        width={isMobile ? '100%' : 720}
        title={
          <span className='break-all'>
            {t('查看 IP 详情')}
            {selectedIP ? ` · ${selectedIP}` : ''}
          </span>
        }
        bodyStyle={{ overflowY: 'auto' }}
        closeOnEsc
        onCancel={() => setSelectedIP('')}
      >
        {selectedIP ? (
          <IPAuditDetailRow
            key={`${selectedIP}-${query.range}-${query.view}-${query.startTimestamp}-${query.endTimestamp}`}
            ip={selectedIP}
            query={query}
            locale={locale}
            t={t}
            onRequestIDSearch={handleRequestIDSearch}
          />
        ) : null}
      </SideSheet>
    </>
  );
};

export default IPAuditPanel;
