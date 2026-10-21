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
import React, { useCallback, useEffect, useState } from 'react';
import { Modal, Pagination } from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';

import { showError, showSuccess } from '../../helpers';
import { useIsMobile } from '../../hooks/common/useIsMobile';
import { cancelUserRedemption, listUserRedemptions } from './api.js';
import UserRedemptionDetailModal from './UserRedemptionDetailModal.jsx';
import UserRedemptionsTable from './UserRedemptionsTable.jsx';

const PAGE_SIZE = 20;

export default function MyUserRedemptionsModal({
  visible,
  onClose,
  onBalanceChange,
}) {
  const { t } = useTranslation();
  const isMobile = useIsMobile();
  const [items, setItems] = useState([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(false);
  const [busy, setBusy] = useState(false);
  const [selectedId, setSelectedId] = useState(null);

  const reload = useCallback(async () => {
    if (!visible) return;
    setLoading(true);
    try {
      const result = await listUserRedemptions(
        { page, page_size: PAGE_SIZE, deleted: 'all' },
        false,
      );
      setItems(result.items || []);
      setTotal(result.total || 0);
    } catch (error) {
      showError(error?.message || t('load failed'));
    } finally {
      setLoading(false);
    }
  }, [page, t, visible]);

  useEffect(() => {
    void reload();
  }, [reload]);

  function confirmCancel(id) {
    const target = items.find((item) => item.id === id);
    const isSubscription = target?.reward_type === 'subscription';
    Modal.confirm({
      title: t('Cancel redemption code?'),
      content: t(
        isSubscription
          ? 'Unused subscription units will be refunded to your wallet automatically.'
          : 'The unclaimed balance will be submitted for administrator settlement.',
      ),
      okText: t('Cancel redemption code'),
      cancelText: t('Close'),
      type: 'warning',
      onOk: async () => {
        setBusy(true);
        try {
          await cancelUserRedemption(id);
          showSuccess(
            t(
              isSubscription
                ? 'Unused subscription units were refunded automatically'
                : 'Redemption code submitted for administrator settlement',
            ),
          );
          await reload();
          await onBalanceChange?.();
        } catch (error) {
          showError(error?.message || t('cancel failed'));
        } finally {
          setBusy(false);
        }
      },
    });
  }

  return (
    <>
      <Modal
        title={t('My Redemption Codes')}
        visible={visible}
        onCancel={() => {
          setPage(1);
          setSelectedId(null);
          onClose?.();
        }}
        footer={null}
        width={isMobile ? 'calc(100vw - 32px)' : 1040}
        bodyStyle={{
          maxHeight: isMobile ? 'calc(100vh - 160px)' : '75vh',
          overflowX: 'hidden',
          overflowY: 'auto',
        }}
      >
        <UserRedemptionsTable
          items={items}
          loading={loading}
          busy={busy}
          onView={setSelectedId}
          onCancel={confirmCancel}
        />
        {total > PAGE_SIZE ? (
          <div className='mt-3 flex justify-center sm:justify-end'>
            <Pagination
              currentPage={page}
              pageSize={PAGE_SIZE}
              total={total}
              onPageChange={setPage}
              showTotal={!isMobile}
            />
          </div>
        ) : null}
      </Modal>
      <UserRedemptionDetailModal
        id={selectedId}
        onClose={() => setSelectedId(null)}
      />
    </>
  );
}
