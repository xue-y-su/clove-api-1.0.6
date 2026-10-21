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
export const SCREEN_CAPTURE_ERROR_MESSAGES = {
  CANVAS_UNAVAILABLE: '无法绘制屏幕截图',
  EMPTY_IMAGE: '屏幕截图未生成图片',
  UNSUPPORTED: '当前浏览器不支持屏幕截图',
};

const pad = (value) => String(value).padStart(2, '0');

export function buildScreenshotFileName(date = new Date()) {
  return `playground-screenshot-${date.getUTCFullYear()}${pad(date.getUTCMonth() + 1)}${pad(date.getUTCDate())}-${pad(date.getUTCHours())}${pad(date.getUTCMinutes())}${pad(date.getUTCSeconds())}.png`;
}

const waitForMetadata = (video) => {
  if (video.readyState >= 1 && video.videoWidth) return Promise.resolve();
  return new Promise((resolve, reject) => {
    const cleanup = () => {
      video.removeEventListener('loadedmetadata', onLoaded);
      video.removeEventListener('error', onError);
    };
    const onLoaded = () => {
      cleanup();
      resolve();
    };
    const onError = () => {
      cleanup();
      reject(new Error(SCREEN_CAPTURE_ERROR_MESSAGES.EMPTY_IMAGE));
    };
    video.addEventListener('loadedmetadata', onLoaded, { once: true });
    video.addEventListener('error', onError, { once: true });
  });
};

export async function captureDisplayScreenshotFile() {
  if (!navigator.mediaDevices?.getDisplayMedia) {
    throw new Error(SCREEN_CAPTURE_ERROR_MESSAGES.UNSUPPORTED);
  }
  const stream = await navigator.mediaDevices.getDisplayMedia({
    audio: false,
    video: true,
  });
  const video = document.createElement('video');
  try {
    video.muted = true;
    video.playsInline = true;
    video.srcObject = stream;
    video.style.cssText =
      'position:fixed;left:-10000px;width:1px;height:1px;opacity:0';
    document.body.append(video);
    await waitForMetadata(video);
    await video.play();
    await new Promise((resolve) => requestAnimationFrame(resolve));
    if (!video.videoWidth || !video.videoHeight) {
      throw new Error(SCREEN_CAPTURE_ERROR_MESSAGES.EMPTY_IMAGE);
    }
    const canvas = document.createElement('canvas');
    canvas.width = video.videoWidth;
    canvas.height = video.videoHeight;
    const context = canvas.getContext('2d');
    if (!context) {
      throw new Error(SCREEN_CAPTURE_ERROR_MESSAGES.CANVAS_UNAVAILABLE);
    }
    context.drawImage(video, 0, 0, canvas.width, canvas.height);
    const blob = await new Promise((resolve, reject) =>
      canvas.toBlob(
        (value) =>
          value
            ? resolve(value)
            : reject(new Error(SCREEN_CAPTURE_ERROR_MESSAGES.EMPTY_IMAGE)),
        'image/png',
      ),
    );
    return new File([blob], buildScreenshotFileName(), { type: 'image/png' });
  } finally {
    video.pause();
    video.srcObject = null;
    video.remove();
    stream.getTracks().forEach((track) => track.stop());
  }
}
