/*
Copyright (C) 2023-2026 QuantumNous

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
  CANVAS_UNAVAILABLE: 'Could not draw the screen capture',
  EMPTY_IMAGE: 'Screen capture did not produce an image',
  UNSUPPORTED: 'Screen capture is not supported in this browser',
} as const

function padDatePart(value: number): string {
  return value.toString().padStart(2, '0')
}

function waitForVideoMetadata(video: HTMLVideoElement): Promise<void> {
  if (video.readyState >= HTMLMediaElement.HAVE_METADATA && video.videoWidth) {
    return Promise.resolve()
  }

  return new Promise((resolve, reject) => {
    const cleanup = () => {
      video.removeEventListener('loadedmetadata', handleLoadedMetadata)
      video.removeEventListener('error', handleError)
    }
    const handleLoadedMetadata = () => {
      cleanup()
      resolve()
    }
    const handleError = () => {
      cleanup()
      reject(new Error(SCREEN_CAPTURE_ERROR_MESSAGES.EMPTY_IMAGE))
    }

    video.addEventListener('loadedmetadata', handleLoadedMetadata, {
      once: true,
    })
    video.addEventListener('error', handleError, { once: true })
  })
}

function waitForNextFrame(): Promise<void> {
  return new Promise((resolve) => {
    window.requestAnimationFrame(() => resolve())
  })
}

function canvasToPngBlob(canvas: HTMLCanvasElement): Promise<Blob> {
  return new Promise((resolve, reject) => {
    canvas.toBlob((blob) => {
      if (!blob) {
        reject(new Error(SCREEN_CAPTURE_ERROR_MESSAGES.EMPTY_IMAGE))
        return
      }

      resolve(blob)
    }, 'image/png')
  })
}

export function buildScreenshotFileName(date = new Date()): string {
  const year = date.getUTCFullYear()
  const month = padDatePart(date.getUTCMonth() + 1)
  const day = padDatePart(date.getUTCDate())
  const hours = padDatePart(date.getUTCHours())
  const minutes = padDatePart(date.getUTCMinutes())
  const seconds = padDatePart(date.getUTCSeconds())

  return `playground-screenshot-${year}${month}${day}-${hours}${minutes}${seconds}.png`
}

export async function captureDisplayScreenshotFile(): Promise<File> {
  if (!navigator.mediaDevices?.getDisplayMedia) {
    throw new Error(SCREEN_CAPTURE_ERROR_MESSAGES.UNSUPPORTED)
  }

  const stream = await navigator.mediaDevices.getDisplayMedia({
    audio: false,
    video: true,
  })
  const video = document.createElement('video')

  try {
    video.muted = true
    video.playsInline = true
    video.srcObject = stream
    video.className =
      'pointer-events-none fixed -left-[10000px] top-0 size-px opacity-0'
    document.body.append(video)

    await waitForVideoMetadata(video)
    await video.play()
    await waitForNextFrame()

    if (!video.videoWidth || !video.videoHeight) {
      throw new Error(SCREEN_CAPTURE_ERROR_MESSAGES.EMPTY_IMAGE)
    }

    const canvas = document.createElement('canvas')
    canvas.width = video.videoWidth
    canvas.height = video.videoHeight

    const context = canvas.getContext('2d')
    if (!context) {
      throw new Error(SCREEN_CAPTURE_ERROR_MESSAGES.CANVAS_UNAVAILABLE)
    }

    context.drawImage(video, 0, 0, canvas.width, canvas.height)
    const blob = await canvasToPngBlob(canvas)

    return new File([blob], buildScreenshotFileName(), {
      type: 'image/png',
    })
  } finally {
    video.pause()
    video.srcObject = null
    video.remove()
    for (const track of stream.getTracks()) {
      track.stop()
    }
  }
}
