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
import React, { useEffect, useMemo, useRef, useState } from 'react';
import { Button, Modal, Spin, Tooltip } from '@douyinfe/semi-ui';
import { Download, Pencil, RotateCcw, Save, X } from 'lucide-react';
import { QRCodeCanvas } from 'qrcode.react';
import { useTranslation } from 'react-i18next';

import { showError, showWarning } from '../../helpers';
import { createDefaultInvitationPosterDesign } from '../../../../shared/invitation-poster-design.js';
import { resolveInvitationPosterScene } from '../../../../shared/invitation-poster-scene.js';
import {
  cleanupInvitationPosterBackgrounds,
  createUnavailableInvitationPosterBackgroundStore,
  loadInvitationPosterState,
  openInvitationPosterBackgroundStore,
  saveInvitationPosterState,
} from '../../../../shared/invitation-poster-storage.js';
import {
  createInvitationPosterObjectUrlLease,
  validateInvitationPosterUpload,
} from '../../../../shared/invitation-poster-upload.js';
import {
  POSTER_SIZES,
  canvasToPngBlob,
  createInvitationPosterFilename,
  downloadPngBlob,
  renderInvitationPoster,
  resolveInvitationPosterLogoUrl,
} from '../../../../shared/invitation-poster.js';
import InvitationPosterEditor from './InvitationPosterEditor';

const STORAGE_WARNING_KEYS = {
  'indexeddb-unavailable':
    'This background is available only until this dialog closes',
  'localstorage-unavailable': 'Poster changes will reset after reload',
  'storage-unavailable': 'Poster changes will reset after reload',
  'missing-upload': 'Saved background was unavailable and has been reset',
};

function getSiteDomain(registrationUrl) {
  try {
    return new URL(registrationUrl).host;
  } catch {
    return typeof window === 'undefined' ? '' : window.location.host;
  }
}

function writableBrowserStorage() {
  try {
    return window.localStorage;
  } catch {
    return {
      setItem() {
        throw new Error('Local storage is unavailable');
      },
    };
  }
}

export default function InvitationQRModal({
  visible,
  onCancel,
  value,
  code,
  systemName,
  logo,
  displayName,
  title,
  subtitle,
}) {
  const { t } = useTranslation();
  const [qrCanvas, setQrCanvas] = useState(null);
  const [mode, setMode] = useState('preview');
  const [activeDesign, setActiveDesign] = useState(
    createDefaultInvitationPosterDesign,
  );
  const [activeAsset, setActiveAsset] = useState(null);
  const [pendingUpload, setPendingUpload] = useState(null);
  const [draft, setDraft] = useState(null);
  const [selectedModule, setSelectedModule] = useState('title');
  const [hydrated, setHydrated] = useState(false);
  const [rendering, setRendering] = useState(false);
  const [downloading, setDownloading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [renderError, setRenderError] = useState(false);
  const [discardConfirmationOpen, setDiscardConfirmationOpen] = useState(false);
  const previewContainerRef = useRef(null);
  const previewCanvasRef = useRef(null);
  const pencilRef = useRef(null);
  const activeUploadLease = useRef(null);
  const pendingUploadLease = useRef(null);
  const savingRef = useRef(false);
  const backgroundStoreRef = useRef(
    createUnavailableInvitationPosterBackgroundStore(),
  );
  const renderControllerRef = useRef(null);
  const renderFrameRef = useRef(null);

  const posterRuntime = useMemo(
    () => ({
      siteName: systemName,
      logoUrl: resolveInvitationPosterLogoUrl(
        logo,
        typeof window === 'undefined' ? '' : window.location.origin,
      ),
      title,
      subtitle,
      defaultTitle: t('Invite you to join "{{siteName}}"', {
        siteName: systemName,
      }),
      defaultSubtitle: t(
        'Scan to register and fill in the invitation code automatically',
      ),
      displayName,
      inviterText: t('Invitation from {{name}}', { name: displayName }),
      invitationCode: code,
      siteDomain: getSiteDomain(value),
      labels: {
        eyebrow: t('Invitation'),
        code: t('Invitation code'),
        footer: t('Exclusive invitation'),
      },
    }),
    [code, displayName, logo, subtitle, systemName, t, title, value],
  );
  const renderedDesign = mode === 'edit' && draft ? draft : activeDesign;
  const renderedAsset =
    mode === 'edit' && pendingUpload ? pendingUpload : activeAsset;
  const scene = useMemo(
    () => resolveInvitationPosterScene(renderedDesign, posterRuntime),
    [posterRuntime, renderedDesign],
  );
  const dimensions = POSTER_SIZES[renderedDesign.size];
  const dirty = Boolean(
    draft &&
    (pendingUpload || JSON.stringify(draft) !== JSON.stringify(activeDesign)),
  );

  useEffect(() => {
    setHydrated(false);
    if (!visible) {
      renderControllerRef.current?.abort();
      if (renderFrameRef.current !== null) {
        cancelAnimationFrame(renderFrameRef.current);
        renderFrameRef.current = null;
      }
      pendingUploadLease.current?.release();
      pendingUploadLease.current = null;
      activeUploadLease.current?.release();
      activeUploadLease.current = null;
      backgroundStoreRef.current.close?.();
      backgroundStoreRef.current =
        createUnavailableInvitationPosterBackgroundStore();
      setPendingUpload(null);
      setActiveAsset(null);
      setDraft(null);
      setMode('preview');
      setDiscardConfirmationOpen(false);
      return;
    }

    let cancelled = false;
    setMode('preview');
    setDraft(null);
    setPendingUpload(null);
    setSelectedModule('title');
    void openInvitationPosterBackgroundStore()
      .catch(() => createUnavailableInvitationPosterBackgroundStore())
      .then(async (store) => {
        if (cancelled) {
          store.close?.();
          return;
        }
        backgroundStoreRef.current.close?.();
        backgroundStoreRef.current = store;
        const result = await loadInvitationPosterState({
          storage: window.localStorage,
          backgroundStore: store,
        });
        if (cancelled) return;

        let resource = null;
        if (result.asset) {
          const lease = createInvitationPosterObjectUrlLease(result.asset.blob);
          activeUploadLease.current?.release();
          activeUploadLease.current = lease;
          resource = { ...result.asset, objectUrl: lease.url };
        }
        setActiveDesign(result.design);
        setActiveAsset(resource);
        if (result.warning) {
          showWarning(t(STORAGE_WARNING_KEYS[result.warning]));
        }
        const referencedAssetId =
          result.design.background.kind === 'upload'
            ? result.design.background.assetId
            : undefined;
        await cleanupInvitationPosterBackgrounds(
          store,
          referencedAssetId,
        ).catch(() => {});
      })
      .catch(() => {
        if (!cancelled) {
          setActiveDesign(createDefaultInvitationPosterDesign());
          showWarning(t('Poster changes will reset after reload'));
        }
      })
      .finally(() => {
        if (!cancelled) setHydrated(true);
      });

    return () => {
      cancelled = true;
    };
  }, [visible, t]);

  useEffect(() => {
    if (!visible || !value || !qrCanvas) return;
    renderControllerRef.current?.abort();
    if (renderFrameRef.current !== null) {
      cancelAnimationFrame(renderFrameRef.current);
    }
    const controller = new AbortController();
    renderControllerRef.current = controller;
    setRendering(true);
    setRenderError(false);

    renderFrameRef.current = requestAnimationFrame(() => {
      renderFrameRef.current = null;
      void renderInvitationPoster({
        design: renderedDesign,
        runtime: posterRuntime,
        qrCanvas,
        backgroundAsset: renderedAsset,
        signal: controller.signal,
      })
        .then((canvas) => {
          if (controller.signal.aborted) return;
          canvas.className = 'block size-full object-contain';
          previewCanvasRef.current = canvas;
          previewContainerRef.current?.replaceChildren(canvas);
        })
        .catch(() => {
          if (!controller.signal.aborted) {
            previewCanvasRef.current = null;
            setRenderError(true);
          }
        })
        .finally(() => {
          if (!controller.signal.aborted) setRendering(false);
        });
    });

    return () => {
      controller.abort();
      if (renderFrameRef.current !== null) {
        cancelAnimationFrame(renderFrameRef.current);
        renderFrameRef.current = null;
      }
    };
  }, [
    mode,
    posterRuntime,
    qrCanvas,
    renderedAsset,
    renderedDesign,
    value,
    visible,
  ]);

  useEffect(
    () => () => {
      renderControllerRef.current?.abort();
      if (renderFrameRef.current !== null) {
        cancelAnimationFrame(renderFrameRef.current);
      }
      pendingUploadLease.current?.release();
      activeUploadLease.current?.release();
      backgroundStoreRef.current.close?.();
    },
    [],
  );

  function enterEditMode() {
    if (!hydrated) return;
    setDraft(structuredClone(activeDesign));
    setSelectedModule('title');
    setMode('edit');
  }

  function cancelEditMode() {
    pendingUploadLease.current?.release();
    pendingUploadLease.current = null;
    setPendingUpload(null);
    setDraft(null);
    setMode('preview');
    requestAnimationFrame(() => pencilRef.current?.focus());
  }

  function resetDraft() {
    pendingUploadLease.current?.release();
    pendingUploadLease.current = null;
    setPendingUpload(null);
    setDraft(createDefaultInvitationPosterDesign());
  }

  function updateDraft(next) {
    if (
      pendingUpload &&
      (next.background.kind !== 'upload' ||
        next.background.assetId !== pendingUpload.assetId)
    ) {
      pendingUploadLease.current?.release();
      pendingUploadLease.current = null;
      setPendingUpload(null);
    }
    setDraft(next);
  }

  async function saveDraft() {
    if (!draft || savingRef.current) return;
    savingRef.current = true;
    setSaving(true);
    try {
      const result = await saveInvitationPosterState({
        activeDesign,
        activeAsset,
        draftDesign: draft,
        pendingUpload,
        storage: writableBrowserStorage(),
        backgroundStore: backgroundStoreRef.current,
      });
      setActiveDesign(result.design);
      setActiveAsset(result.asset);
      if (pendingUpload && result.asset?.assetId === pendingUpload.assetId) {
        activeUploadLease.current?.release();
        activeUploadLease.current = pendingUploadLease.current;
        pendingUploadLease.current = null;
      } else if (result.design.background.kind !== 'upload') {
        activeUploadLease.current?.release();
        activeUploadLease.current = null;
      }
      setPendingUpload(null);
      setDraft(null);
      setMode('preview');
      requestAnimationFrame(() => pencilRef.current?.focus());
      if (result.warning) {
        showWarning(t(STORAGE_WARNING_KEYS[result.warning]));
      }
    } finally {
      savingRef.current = false;
      setSaving(false);
    }
  }

  async function handleUpload(file) {
    try {
      const validated = await validateInvitationPosterUpload(file);
      const lease = createInvitationPosterObjectUrlLease(validated.blob);
      const resource = {
        ...validated,
        assetId: crypto.randomUUID(),
        createdAt: Date.now(),
        objectUrl: lease.url,
      };
      pendingUploadLease.current?.release();
      pendingUploadLease.current = lease;
      setPendingUpload(resource);
      const cropBySize = Object.fromEntries(
        ['4:5', '3:4', '9:16'].map((size) => [
          size,
          { focalX: 0.5, focalY: 0.5, zoom: 1 },
        ]),
      );
      setDraft((current) =>
        current
          ? {
              ...current,
              background: {
                kind: 'upload',
                assetId: resource.assetId,
                cropBySize,
              },
            }
          : current,
      );
    } catch {
      showError(t('Use a PNG, JPEG, or WebP image up to 10 MiB'));
    }
  }

  async function downloadPoster() {
    const canvas = previewCanvasRef.current;
    if (!canvas) return;
    setDownloading(true);
    try {
      const blob = await canvasToPngBlob(canvas);
      downloadPngBlob(blob, createInvitationPosterFilename(systemName, code));
    } catch {
      showError(t('Failed to download invitation poster'));
    } finally {
      setDownloading(false);
    }
  }

  function requestDialogClose() {
    if (savingRef.current) return;
    if (mode === 'edit' && dirty) {
      setDiscardConfirmationOpen(true);
      return;
    }
    onCancel();
  }

  function discardAndClose() {
    setDiscardConfirmationOpen(false);
    pendingUploadLease.current?.release();
    pendingUploadLease.current = null;
    setPendingUpload(null);
    setDraft(null);
    setMode('preview');
    onCancel();
  }

  const previewFrame = (
    <div
      className='relative mx-auto w-full max-w-[320px] overflow-hidden rounded-md border border-[var(--semi-color-border)] bg-[var(--semi-color-fill-0)] shadow-sm'
      style={{ aspectRatio: `${dimensions.width} / ${dimensions.height}` }}
      role='img'
      aria-label={t('Invitation poster preview')}
    >
      <div ref={previewContainerRef} className='size-full' />
      {rendering ? (
        <div className='absolute inset-0 grid place-items-center bg-white/75'>
          <Spin aria-label={t('加载中')} />
        </div>
      ) : null}
      {renderError ? (
        <div className='absolute inset-0 grid place-items-center bg-[var(--semi-color-bg-0)] p-6 text-center text-sm'>
          {t('Failed to render invitation poster')}
        </div>
      ) : null}
    </div>
  );

  const modalFooter =
    mode === 'preview' ? (
      <div className='flex w-full items-center justify-end gap-2'>
        <Tooltip content={t('Edit poster')}>
          <Button
            ref={pencilRef}
            theme='light'
            type='tertiary'
            icon={<Pencil size={16} aria-hidden='true' />}
            aria-label={t('Edit poster')}
            disabled={!hydrated}
            onClick={enterEditMode}
          />
        </Tooltip>
        <Button
          theme='solid'
          type='primary'
          icon={<Download size={16} aria-hidden='true' />}
          loading={!hydrated || rendering || downloading}
          disabled={!hydrated || renderError || !previewCanvasRef.current}
          onClick={() => void downloadPoster()}
        >
          {t('下载 PNG')}
        </Button>
      </div>
    ) : (
      <div className='flex w-full flex-wrap items-center justify-end gap-2'>
        <Button
          theme='borderless'
          type='tertiary'
          icon={<RotateCcw size={16} aria-hidden='true' />}
          className='mr-auto'
          disabled={saving}
          onClick={resetDraft}
        >
          {t('Reset poster')}
        </Button>
        <Button
          theme='light'
          type='tertiary'
          icon={<X size={16} aria-hidden='true' />}
          disabled={saving}
          onClick={cancelEditMode}
        >
          {t('Cancel')}
        </Button>
        <Button
          theme='solid'
          type='primary'
          icon={<Save size={16} aria-hidden='true' />}
          loading={saving}
          disabled={saving}
          onClick={() => void saveDraft()}
        >
          {t('Save')}
        </Button>
      </div>
    );

  return (
    <>
      <Modal
        title={t('Invitation poster')}
        visible={visible}
        onCancel={requestDialogClose}
        footer={modalFooter}
        width={mode === 'edit' ? 1120 : 520}
        style={{ maxWidth: 'calc(100vw - 24px)' }}
        bodyStyle={{
          maxHeight:
            mode === 'edit' ? 'min(72vh, 760px)' : 'calc(100vh - 180px)',
          overflowY: 'auto',
        }}
        centered
      >
        {mode === 'preview' ? previewFrame : null}
        {mode === 'edit' && draft ? (
          <InvitationPosterEditor
            design={draft}
            scene={scene}
            selectedModule={selectedModule}
            backgroundAsset={renderedAsset}
            previewContainerRef={previewContainerRef}
            rendering={rendering}
            renderError={renderError}
            onDesignChange={updateDraft}
            onSelectedModuleChange={setSelectedModule}
            onUpload={handleUpload}
          />
        ) : null}

        <div
          className='pointer-events-none fixed -left-[10000px] top-0 size-[360px] overflow-hidden'
          aria-hidden='true'
        >
          <QRCodeCanvas
            ref={setQrCanvas}
            value={value}
            size={360}
            level='H'
            marginSize={4}
          />
        </div>
      </Modal>

      <Modal
        title={t('Discard poster changes?')}
        visible={discardConfirmationOpen}
        onCancel={() => setDiscardConfirmationOpen(false)}
        width={420}
        centered
        footer={
          <div className='flex justify-end gap-2'>
            <Button
              theme='light'
              type='tertiary'
              onClick={() => setDiscardConfirmationOpen(false)}
            >
              {t('Keep editing')}
            </Button>
            <Button theme='solid' type='danger' onClick={discardAndClose}>
              {t('Discard changes')}
            </Button>
          </div>
        }
      >
        {t('Poster changes will reset after reload')}
      </Modal>
    </>
  );
}
