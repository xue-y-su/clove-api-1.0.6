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
const managedImagePattern =
  /^\/api\/announcement-media\/[a-f0-9]{64}\.(?:jpg|png|webp|gif)$/;

export const isManagedAnnouncementImageURL = (value) =>
  typeof value === 'string' && managedImagePattern.test(value.trim());

export const isSafeAnnouncementImageURL = (value) => {
  if (isManagedAnnouncementImageURL(value)) return true;

  try {
    const url = new URL(value);
    const hostname = url.hostname.toLowerCase().replace(/\.$/, '');
    const isIPAddress = /^[\d.]+$/.test(hostname) || hostname.includes(':');
    return (
      url.protocol === 'https:' &&
      url.username === '' &&
      url.password === '' &&
      hostname.includes('.') &&
      hostname !== 'localhost' &&
      !hostname.endsWith('.localhost') &&
      !isIPAddress
    );
  } catch {
    return false;
  }
};

const getResponseData = (response, fallback) => {
  if (!response?.success || response.data === undefined) {
    throw new Error(response?.message || fallback);
  }
  return response.data;
};

const getRequestError = (error, fallback) => {
  const message = error?.response?.data?.message;
  if (typeof message === 'string' && message.trim()) {
    return new Error(message);
  }
  return error instanceof Error ? error : new Error(fallback);
};

export const getGlobalAnnouncementState = async (apiClient) => {
  try {
    const response = await apiClient.get('/api/announcement/popup-state', {
      skipErrorHandler: true,
    });
    return getResponseData(response.data, '加载系统公告失败');
  } catch (error) {
    throw getRequestError(error, '加载系统公告失败');
  }
};

export const acknowledgeGlobalAnnouncement = async (apiClient, signature) => {
  try {
    const response = await apiClient.post(
      '/api/announcement/acknowledge',
      { signature },
      { skipErrorHandler: true },
    );
    const data = getResponseData(response.data, '公告确认失败，请重试');
    if (!data.acknowledged) {
      throw new Error('公告确认失败，请重试');
    }
  } catch (error) {
    throw getRequestError(error, '公告确认失败，请重试');
  }
};

export const uploadAnnouncementMedia = async (apiClient, file) => {
  const formData = new FormData();
  formData.append('file', file);

  try {
    const response = await apiClient.post('/api/announcement/media', formData, {
      skipErrorHandler: true,
    });
    const data = getResponseData(response.data, '公告图片上传失败');
    if (!isManagedAnnouncementImageURL(data.url)) {
      throw new Error('公告图片上传失败');
    }
    return data.url;
  } catch (error) {
    throw getRequestError(error, '公告图片上传失败');
  }
};
