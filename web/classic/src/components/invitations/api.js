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
import { normalizeInvitationRequestError } from './invitation-request-error';

function responseData(response) {
  if (!response?.data?.success) {
    throw new Error(response?.data?.message || 'Request failed');
  }
  return response.data.data;
}

export async function getInvitationSelf() {
  return responseData(await API.get('/api/invitation/self'));
}

export async function getInvitationRewardSummary() {
  return responseData(await API.get('/api/invitation/rewards'));
}

export async function getInvitationIncentiveSettings() {
  return responseData(await API.get('/api/invitation/incentive-settings'));
}

export async function updateInvitationIncentiveSettings(settings) {
  return responseData(
    await API.put('/api/invitation/incentive-settings', settings, {
      skipErrorHandler: true,
    }),
  );
}

export async function submitInvitationRewardPayout() {
  return mutateInvitation('/api/invitation/rewards/payout-requests');
}

export async function validateInvitationCode(code, signal) {
  return mutateInvitation('/api/invitation/validate', { code }, signal);
}

export async function applyForInvitation(reason) {
  return mutateInvitation('/api/invitation/applications', { reason });
}

export async function withdrawInvitationApplication(applicationId) {
  return mutateInvitation(
    `/api/invitation/applications/${applicationId}/withdraw`,
  );
}

export async function listInvitationApplications(params) {
  return responseData(
    await API.get('/api/invitation/admin/applications', { params }),
  );
}

export async function reviewInvitationApplication(applicationId, input) {
  return mutateInvitation(
    `/api/invitation/admin/applications/${applicationId}/review`,
    input,
  );
}

export async function listInvitationCodes(params) {
  return responseData(await API.get('/api/invitation/admin/codes', { params }));
}

export async function issueInvitationCode(input) {
  return mutateInvitation('/api/invitation/admin/codes', input);
}

export async function revokeInvitationCode(codeId, reason) {
  return mutateInvitation(`/api/invitation/admin/codes/${codeId}/revoke`, {
    reason,
  });
}

export async function revokeOwnInvitationCode(codeId, reason) {
  return mutateInvitation(`/api/invitation/codes/${codeId}/revoke`, {
    reason,
  });
}

export async function listInvitationUses(params) {
  return responseData(await API.get('/api/invitation/admin/uses', { params }));
}

export async function listInvitationRewardPayoutRequests(params) {
  return responseData(
    await API.get('/api/invitation/admin/reward-payout-requests', { params }),
  );
}

export async function reviewInvitationRewardPayoutRequest(requestId, input) {
  return mutateInvitation(
    `/api/invitation/admin/reward-payout-requests/${requestId}/review`,
    input,
  );
}

export async function getInvitationRewardPayoutRequestDetail(requestId) {
  return responseData(
    await API.get(`/api/invitation/admin/reward-payout-requests/${requestId}`),
  );
}

export async function freezeInvitationRewardPayoutRequest(requestId, reason) {
  return mutateInvitation(
    `/api/invitation/admin/reward-payout-requests/${requestId}/freeze`,
    { reason },
  );
}

export async function unfreezeInvitationRewardPayoutRequest(requestId, reason) {
  return mutateInvitation(
    `/api/invitation/admin/reward-payout-requests/${requestId}/unfreeze`,
    { reason },
  );
}

async function mutateInvitation(url, data, signal) {
  try {
    const response = await API.post(url, data, {
      ...(signal ? { signal } : {}),
      skipErrorHandler: true,
    });
    return responseData(response);
  } catch (error) {
    throw normalizeInvitationRequestError(error, 'Request failed');
  }
}
