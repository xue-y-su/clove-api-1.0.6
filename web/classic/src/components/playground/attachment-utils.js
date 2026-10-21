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
export const MAX_IMAGE_ATTACHMENTS = 6;
export const MAX_IMAGE_ATTACHMENT_BYTES = 8 * 1024 * 1024;
export const MAX_TEXT_ATTACHMENTS = 8;
export const MAX_TEXT_ATTACHMENT_BYTES = 512 * 1024;
export const MAX_DOCUMENT_ATTACHMENT_BYTES = 16 * 1024 * 1024;
export const MAX_TEXT_ATTACHMENT_CHARS = 80000;
export const MAX_TOTAL_TEXT_ATTACHMENT_CHARS = 120000;

const TEXT_EXTENSIONS = new Set([
  'txt',
  'md',
  'markdown',
  'json',
  'csv',
  'tsv',
  'log',
  'yaml',
  'yml',
  'xml',
]);
const DOCUMENT_EXTENSIONS = new Set(['pdf', 'docx', 'xls', 'xlsx', 'ods']);

const createAttachmentId = () =>
  globalThis.crypto?.randomUUID?.() ||
  `${Date.now()}-${Math.random().toString(36).slice(2)}`;

const getExtension = (name = '') => {
  const index = name.lastIndexOf('.');
  return index < 0 ? '' : name.slice(index + 1).toLowerCase();
};

const getMimeType = (file) =>
  (file.type || 'application/octet-stream')
    .split(';', 1)[0]
    .trim()
    .toLowerCase();

const isTextFile = (file) => {
  const mimeType = getMimeType(file);
  return (
    mimeType.startsWith('text/') ||
    ['application/json', 'application/xml', 'application/x-yaml'].includes(
      mimeType,
    ) ||
    TEXT_EXTENSIONS.has(getExtension(file.name))
  );
};

const isDocumentFile = (file) =>
  DOCUMENT_EXTENSIONS.has(getExtension(file.name));

const readAsDataUrl = (file) =>
  new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.addEventListener(
      'load',
      () =>
        typeof reader.result === 'string'
          ? resolve(reader.result)
          : reject(new Error('Failed to read image attachment')),
      { once: true },
    );
    reader.addEventListener('error', () => reject(reader.error), {
      once: true,
    });
    reader.readAsDataURL(file);
  });

const truncateText = (text) =>
  text.length <= MAX_TEXT_ATTACHMENT_CHARS
    ? text
    : `${text.slice(0, MAX_TEXT_ATTACHMENT_CHARS)}\n\n[Attachment truncated]`;

const readPdfText = async (file) => {
  const pdfjs = await import('pdfjs-dist/legacy/build/pdf.mjs');
  pdfjs.GlobalWorkerOptions.workerSrc ||= new URL(
    'pdfjs-dist/legacy/build/pdf.worker.mjs',
    import.meta.url,
  ).toString();
  const task = pdfjs.getDocument({
    data: new Uint8Array(await file.arrayBuffer()),
  });
  const pdf = await task.promise;
  const pages = [];
  try {
    for (let pageNumber = 1; pageNumber <= pdf.numPages; pageNumber += 1) {
      const page = await pdf.getPage(pageNumber);
      const content = await page.getTextContent();
      const text = content.items
        .map((item) => ('str' in item ? item.str : ''))
        .join(' ')
        .replaceAll(/\s+/g, ' ')
        .trim();
      if (text) pages.push(`[Page ${pageNumber}]\n${text}`);
      page.cleanup();
    }
  } finally {
    await task.destroy();
  }
  return pages.join('\n\n');
};

const readDocumentText = async (file) => {
  const extension = getExtension(file.name);
  if (extension === 'pdf') return readPdfText(file);
  if (extension === 'docx') {
    const mammoth = await import('mammoth');
    const result = await mammoth.extractRawText({
      arrayBuffer: await file.arrayBuffer(),
    });
    return result.value.trim();
  }
  if (['xls', 'xlsx', 'ods'].includes(extension)) {
    const xlsx = await import('@e965/xlsx');
    const workbook = xlsx.read(await file.arrayBuffer(), { type: 'array' });
    return workbook.SheetNames.flatMap((sheetName) => {
      const csv = xlsx.utils
        .sheet_to_csv(workbook.Sheets[sheetName], { blankrows: false })
        .trim();
      return csv ? [`[Sheet: ${sheetName}]\n${csv}`] : [];
    }).join('\n\n');
  }
  return '';
};

export function createImageAttachmentFromDataUrl(
  dataUrl,
  name = 'image',
  size = 0,
) {
  const dataUrlMimeType = /^data:([^;,]+)/i.exec(dataUrl)?.[1];
  return {
    id: createAttachmentId(),
    kind: 'image',
    mimeType: dataUrlMimeType || 'image/*',
    name,
    size,
    dataUrl,
  };
}

export function buildPendingPlaygroundAttachments(
  uploadedAttachments = [],
  imageEnabled = false,
  imageUrls = [],
) {
  const uploaded = Array.isArray(uploadedAttachments)
    ? uploadedAttachments
    : [];
  if (!imageEnabled) return uploaded;

  const manualImages = (Array.isArray(imageUrls) ? imageUrls : [])
    .filter((url) => typeof url === 'string' && url.trim() !== '')
    .map((url, index) =>
      createImageAttachmentFromDataUrl(url.trim(), `image-${index + 1}`),
    );
  return [...uploaded, ...manualImages];
}

export async function readPlaygroundAttachments(files, existing = []) {
  const attachments = [];
  const rejected = [];
  let imageCount = existing.filter((item) => item.kind === 'image').length;
  let textCount = existing.filter((item) => item.kind === 'text').length;
  let totalTextChars = existing.reduce(
    (total, item) =>
      total + (item.kind === 'text' ? item.text?.length || 0 : 0),
    0,
  );

  for (const file of Array.from(files || [])) {
    const mimeType = getMimeType(file);
    if (mimeType.startsWith('image/')) {
      if (imageCount >= MAX_IMAGE_ATTACHMENTS) {
        rejected.push({
          key: '最多可添加 {{count}} 张图片',
          values: { count: MAX_IMAGE_ATTACHMENTS },
        });
        continue;
      }
      if (file.size > MAX_IMAGE_ATTACHMENT_BYTES) {
        rejected.push({
          key: '图片 {{name}} 超过大小限制',
          values: { name: file.name },
        });
        continue;
      }
      attachments.push(
        createImageAttachmentFromDataUrl(
          await readAsDataUrl(file),
          file.name || 'image',
          file.size,
        ),
      );
      imageCount += 1;
      continue;
    }

    if (isTextFile(file) || isDocumentFile(file)) {
      if (textCount >= MAX_TEXT_ATTACHMENTS) {
        rejected.push({
          key: '最多可添加 {{count}} 个文件',
          values: { count: MAX_TEXT_ATTACHMENTS },
        });
        continue;
      }
      const maxBytes = isDocumentFile(file)
        ? MAX_DOCUMENT_ATTACHMENT_BYTES
        : MAX_TEXT_ATTACHMENT_BYTES;
      if (file.size > maxBytes) {
        rejected.push({
          key: '文件 {{name}} 超过大小限制',
          values: { name: file.name },
        });
        continue;
      }
      try {
        const text = truncateText(
          isDocumentFile(file)
            ? await readDocumentText(file)
            : await file.text(),
        );
        if (!text.trim()) {
          rejected.push({
            key: '无法从 {{name}} 提取可读文本',
            values: { name: file.name },
          });
          continue;
        }
        if (totalTextChars + text.length > MAX_TOTAL_TEXT_ATTACHMENT_CHARS) {
          rejected.push({ key: '附件文本总长度超过限制' });
          continue;
        }
        attachments.push({
          id: createAttachmentId(),
          kind: 'text',
          mimeType,
          name: file.name || 'file',
          size: file.size,
          text,
        });
        textCount += 1;
        totalTextChars += text.length;
      } catch {
        rejected.push({
          key: '无法读取文件 {{name}}',
          values: { name: file.name },
        });
      }
      continue;
    }

    rejected.push({
      key: '暂不支持文件 {{name}}（{{type}}）',
      values: { name: file.name, type: mimeType },
    });
  }

  return { attachments, rejected };
}

const escapeAttribute = (value) =>
  value
    .replaceAll('&', '&amp;')
    .replaceAll('"', '&quot;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;');

export function buildPromptWithTextAttachments(text, attachments = []) {
  const files = attachments.filter(
    (attachment) => attachment.kind === 'text' && attachment.text,
  );
  if (files.length === 0) return text;
  const context = files
    .map(
      (file) =>
        `<attachment name="${escapeAttribute(file.name)}" type="${escapeAttribute(file.mimeType)}">\n${file.text}\n</attachment>`,
    )
    .join('\n');
  const block = `<attachments>\n${context}\n</attachments>`;
  return text?.trim() ? `${text}\n\n${block}` : block;
}

export function buildPlaygroundMessageContent(text, attachments = []) {
  const prompt = buildPromptWithTextAttachments(text, attachments);
  const images = attachments.filter(
    (attachment) => attachment.kind === 'image' && attachment.dataUrl,
  );
  if (images.length === 0) return prompt;
  return [
    { type: 'text', text: prompt || '' },
    ...images.map((image) => ({
      type: 'image_url',
      image_url: { url: image.dataUrl },
    })),
  ];
}

export function sanitizeAttachmentsForStorage(attachments = []) {
  return attachments.map(({ id, kind, mimeType, name, size }) => ({
    id,
    kind,
    mimeType,
    name,
    size,
  }));
}

export function formatAttachmentSize(size) {
  if (!Number.isFinite(size) || size <= 0) return '0 B';
  if (size < 1024) return `${size} B`;
  if (size < 1024 * 1024) return `${(size / 1024).toFixed(1)} KB`;
  return `${(size / 1024 / 1024).toFixed(1)} MB`;
}
