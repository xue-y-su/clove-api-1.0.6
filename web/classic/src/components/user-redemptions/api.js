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
import { API } from '../../helpers';

export async function listUserRedemptions(params = {}, admin = false) {
  const endpoint = admin
    ? '/api/user-redemption/admin/'
    : '/api/user-redemption/';
  const response = await API.get(endpoint, { params });
  return response.data?.data || { items: [], total: 0, page: 1, page_size: 20 };
}

export async function createUserRedemption(payload) {
  const response = await API.post('/api/user-redemption/', payload);
  if (!response.data?.success) {
    throw new Error(response.data?.message || 'Request failed');
  }
  return response.data?.data;
}

export async function claimUserRedemption(code, expectedRewardType) {
  const response = await API.post('/api/user-redemption/claim', {
    code,
    expected_reward_type: expectedRewardType,
  });
  if (!response.data?.success) {
    throw new Error(response.data?.message || 'Request failed');
  }
  return response.data?.data;
}

export async function getUserRedemption(id, admin = false) {
  const endpoint = admin
    ? `/api/user-redemption/admin/${id}`
    : `/api/user-redemption/${id}`;
  const response = await API.get(endpoint);
  return response.data?.data;
}

export async function cancelUserRedemption(id) {
  const response = await API.post(`/api/user-redemption/${id}/cancel`);
  return response.data?.data;
}

export async function disableAdminUserRedemption(id, reason) {
  const response = await API.post(`/api/user-redemption/admin/${id}/disable`, {
    reason,
  });
  return response.data?.data;
}

export async function enableAdminUserRedemption(id, reason) {
  const response = await API.post(`/api/user-redemption/admin/${id}/enable`, {
    reason,
  });
  return response.data?.data;
}

export async function terminateAdminUserRedemption(id, settlement, reason) {
  const response = await API.post(
    `/api/user-redemption/admin/${id}/terminate`,
    { settlement, reason },
  );
  return response.data?.data;
}

export async function deleteAdminUserRedemption(id, settlement, reason) {
  const response = await API.post(`/api/user-redemption/admin/${id}/delete`, {
    settlement,
    reason,
  });
  return response.data?.data;
}

export async function batchSettleAdminUserRedemptions({
  ids,
  disposition,
  reason,
}) {
  const response = await API.post(
    '/api/user-redemption/admin/batch-settlement',
    { ids, disposition, reason },
  );
  return response.data?.data || [];
}
