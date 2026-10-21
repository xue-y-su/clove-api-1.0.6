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
import {
  Download,
  LoaderCircle,
  Pencil,
  RotateCcw,
  Save,
  X,
} from 'lucide-react'
import { QRCodeCanvas } from 'qrcode.react'
import { useEffect, useMemo, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Dialog } from '@/components/dialog'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Button } from '@/components/ui/button'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'

import {
  createDefaultInvitationPosterDesign,
  type BackgroundCrop,
  type InvitationPosterDesignV2,
  type InvitationPosterSize,
  type PosterModuleId,
} from '../../../../../shared/invitation-poster-design.js'
import {
  resolveInvitationPosterScene,
  type InvitationPosterRuntime,
} from '../../../../../shared/invitation-poster-scene.js'
import {
  cleanupInvitationPosterBackgrounds,
  createUnavailableInvitationPosterBackgroundStore,
  loadInvitationPosterState,
  openInvitationPosterBackgroundStore,
  saveInvitationPosterState,
  type InvitationPosterBackgroundResource,
  type InvitationPosterBackgroundStore,
} from '../../../../../shared/invitation-poster-storage.js'
import {
  createInvitationPosterObjectUrlLease,
  validateInvitationPosterUpload,
  type InvitationPosterObjectUrlLease,
} from '../../../../../shared/invitation-poster-upload.js'
import {
  POSTER_SIZES,
  canvasToPngBlob,
  createInvitationPosterFilename,
  downloadPngBlob,
  renderInvitationPoster,
  resolveInvitationPosterLogoUrl,
} from '../../../../../shared/invitation-poster.js'
import { InvitationPosterEditor } from './invitation-poster-editor'

interface InvitationQRDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  value: string
  code: string
  systemName: string
  logo: string
  displayName: string
  title: string
  subtitle: string
}

const STORAGE_WARNING_KEYS = {
  'indexeddb-unavailable':
    'This background is available only until this dialog closes',
  'localstorage-unavailable': 'Poster changes will reset after reload',
  'storage-unavailable': 'Poster changes will reset after reload',
  'missing-upload': 'Saved background was unavailable and has been reset',
} as const

function getSiteDomain(registrationUrl: string): string {
  try {
    return new URL(registrationUrl).host
  } catch {
    return typeof window === 'undefined' ? '' : window.location.host
  }
}

function writableBrowserStorage(): Pick<Storage, 'setItem'> {
  try {
    return window.localStorage
  } catch {
    return {
      setItem() {
        throw new Error('Local storage is unavailable')
      },
    }
  }
}

export function InvitationQRDialog(props: InvitationQRDialogProps) {
  const { t } = useTranslation()
  const [qrCanvas, setQrCanvas] = useState<HTMLCanvasElement | null>(null)
  const [mode, setMode] = useState<'preview' | 'edit'>('preview')
  const [activeDesign, setActiveDesign] = useState(
    createDefaultInvitationPosterDesign
  )
  const [activeAsset, setActiveAsset] =
    useState<InvitationPosterBackgroundResource | null>(null)
  const [pendingUpload, setPendingUpload] =
    useState<InvitationPosterBackgroundResource | null>(null)
  const [draft, setDraft] = useState<InvitationPosterDesignV2 | null>(null)
  const [selectedModule, setSelectedModule] = useState<PosterModuleId | null>(
    'title'
  )
  const [rendering, setRendering] = useState(false)
  const [downloading, setDownloading] = useState(false)
  const [renderError, setRenderError] = useState(false)
  const [discardConfirmationOpen, setDiscardConfirmationOpen] = useState(false)
  const previewContainerRef = useRef<HTMLDivElement>(null)
  const previewCanvasRef = useRef<HTMLCanvasElement | null>(null)
  const pencilRef = useRef<HTMLButtonElement>(null)
  const activeUploadLease = useRef<InvitationPosterObjectUrlLease | null>(null)
  const pendingUploadLease = useRef<InvitationPosterObjectUrlLease | null>(null)
  const backgroundStoreRef = useRef<InvitationPosterBackgroundStore>(
    createUnavailableInvitationPosterBackgroundStore()
  )
  const renderControllerRef = useRef<AbortController | null>(null)
  const renderFrameRef = useRef<number | null>(null)

  const posterRuntime = useMemo<InvitationPosterRuntime>(
    () => ({
      siteName: props.systemName,
      logoUrl: resolveInvitationPosterLogoUrl(
        props.logo,
        typeof window === 'undefined' ? '' : window.location.origin
      ),
      title: props.title,
      subtitle: props.subtitle,
      defaultTitle: t('Invite you to join "{{siteName}}"', {
        siteName: props.systemName,
      }),
      defaultSubtitle: t(
        'Scan to register and fill in the invitation code automatically'
      ),
      displayName: props.displayName,
      inviterText: t('Invitation from {{name}}', {
        name: props.displayName,
      }),
      invitationCode: props.code,
      siteDomain: getSiteDomain(props.value),
      labels: {
        eyebrow: t('Invitation'),
        code: t('Invitation code'),
        footer: t('Exclusive invitation'),
      },
    }),
    [
      props.code,
      props.displayName,
      props.logo,
      props.subtitle,
      props.systemName,
      props.title,
      props.value,
      t,
    ]
  )
  const renderedDesign = mode === 'edit' && draft ? draft : activeDesign
  const renderedAsset =
    mode === 'edit' && pendingUpload ? pendingUpload : activeAsset
  const scene = useMemo(
    () => resolveInvitationPosterScene(renderedDesign, posterRuntime),
    [posterRuntime, renderedDesign]
  )
  const dimensions = POSTER_SIZES[renderedDesign.size]
  const dirty = Boolean(
    draft &&
    (pendingUpload || JSON.stringify(draft) !== JSON.stringify(activeDesign))
  )

  useEffect(() => {
    if (!props.open) {
      renderControllerRef.current?.abort()
      if (renderFrameRef.current !== null) {
        cancelAnimationFrame(renderFrameRef.current)
        renderFrameRef.current = null
      }
      pendingUploadLease.current?.release()
      pendingUploadLease.current = null
      activeUploadLease.current?.release()
      activeUploadLease.current = null
      backgroundStoreRef.current.close?.()
      backgroundStoreRef.current =
        createUnavailableInvitationPosterBackgroundStore()
      setPendingUpload(null)
      setActiveAsset(null)
      setDraft(null)
      setMode('preview')
      return
    }

    let cancelled = false
    setMode('preview')
    setDraft(null)
    setPendingUpload(null)
    setSelectedModule('title')
    void openInvitationPosterBackgroundStore()
      .catch(() => createUnavailableInvitationPosterBackgroundStore())
      .then(async (store) => {
        if (cancelled) {
          store.close?.()
          return
        }
        backgroundStoreRef.current.close?.()
        backgroundStoreRef.current = store
        const result = await loadInvitationPosterState({
          storage: window.localStorage,
          backgroundStore: store,
        })
        if (cancelled) return

        let resource: InvitationPosterBackgroundResource | null = null
        if (result.asset) {
          const lease = createInvitationPosterObjectUrlLease(result.asset.blob)
          activeUploadLease.current?.release()
          activeUploadLease.current = lease
          resource = { ...result.asset, objectUrl: lease.url }
        }
        setActiveDesign(result.design)
        setActiveAsset(resource)
        if (result.warning) {
          toast.warning(t(STORAGE_WARNING_KEYS[result.warning]))
        }
        const referencedAssetId =
          result.design.background.kind === 'upload'
            ? result.design.background.assetId
            : undefined
        await cleanupInvitationPosterBackgrounds(
          store,
          referencedAssetId
        ).catch(() => {})
      })
      .catch(() => {
        if (!cancelled) {
          setActiveDesign(createDefaultInvitationPosterDesign())
          toast.warning(t('Poster changes will reset after reload'))
        }
      })

    return () => {
      cancelled = true
    }
  }, [props.open, t])

  useEffect(() => {
    if (!props.open || !props.value || !qrCanvas) return
    renderControllerRef.current?.abort()
    if (renderFrameRef.current !== null) {
      cancelAnimationFrame(renderFrameRef.current)
    }
    const controller = new AbortController()
    renderControllerRef.current = controller
    setRendering(true)
    setRenderError(false)

    renderFrameRef.current = requestAnimationFrame(() => {
      renderFrameRef.current = null
      void renderInvitationPoster({
        design: renderedDesign,
        runtime: posterRuntime,
        qrCanvas,
        backgroundAsset: renderedAsset,
        signal: controller.signal,
      })
        .then((canvas) => {
          if (controller.signal.aborted) return
          canvas.className = 'block size-full object-contain'
          previewCanvasRef.current = canvas
          previewContainerRef.current?.replaceChildren(canvas)
        })
        .catch(() => {
          if (!controller.signal.aborted) {
            previewCanvasRef.current = null
            setRenderError(true)
          }
        })
        .finally(() => {
          if (!controller.signal.aborted) setRendering(false)
        })
    })

    return () => {
      controller.abort()
      if (renderFrameRef.current !== null) {
        cancelAnimationFrame(renderFrameRef.current)
        renderFrameRef.current = null
      }
    }
  }, [
    mode,
    posterRuntime,
    props.open,
    props.value,
    qrCanvas,
    renderedAsset,
    renderedDesign,
  ])

  useEffect(
    () => () => {
      renderControllerRef.current?.abort()
      if (renderFrameRef.current !== null) {
        cancelAnimationFrame(renderFrameRef.current)
      }
      pendingUploadLease.current?.release()
      activeUploadLease.current?.release()
      backgroundStoreRef.current.close?.()
    },
    []
  )

  function enterEditMode() {
    setDraft(structuredClone(activeDesign))
    setSelectedModule('title')
    setMode('edit')
  }

  function cancelEditMode() {
    pendingUploadLease.current?.release()
    pendingUploadLease.current = null
    setPendingUpload(null)
    setDraft(null)
    setMode('preview')
    requestAnimationFrame(() => pencilRef.current?.focus())
  }

  function resetDraft() {
    pendingUploadLease.current?.release()
    pendingUploadLease.current = null
    setPendingUpload(null)
    setDraft(createDefaultInvitationPosterDesign())
  }

  function updateDraft(next: InvitationPosterDesignV2) {
    if (
      pendingUpload &&
      (next.background.kind !== 'upload' ||
        next.background.assetId !== pendingUpload.assetId)
    ) {
      pendingUploadLease.current?.release()
      pendingUploadLease.current = null
      setPendingUpload(null)
    }
    setDraft(next)
  }

  async function saveDraft() {
    if (!draft) return
    const result = await saveInvitationPosterState({
      activeDesign,
      activeAsset,
      draftDesign: draft,
      pendingUpload,
      storage: writableBrowserStorage(),
      backgroundStore: backgroundStoreRef.current,
    })
    setActiveDesign(result.design)
    setActiveAsset(result.asset)
    if (pendingUpload && result.asset?.assetId === pendingUpload.assetId) {
      activeUploadLease.current?.release()
      activeUploadLease.current = pendingUploadLease.current
      pendingUploadLease.current = null
    } else if (result.design.background.kind !== 'upload') {
      activeUploadLease.current?.release()
      activeUploadLease.current = null
    }
    setPendingUpload(null)
    setDraft(null)
    setMode('preview')
    requestAnimationFrame(() => pencilRef.current?.focus())
    if (result.warning) toast.warning(t(STORAGE_WARNING_KEYS[result.warning]))
  }

  async function handleUpload(file: File) {
    try {
      const validated = await validateInvitationPosterUpload(file)
      const lease = createInvitationPosterObjectUrlLease(validated.blob)
      const resource: InvitationPosterBackgroundResource = {
        ...validated,
        assetId: crypto.randomUUID(),
        createdAt: Date.now(),
        objectUrl: lease.url,
      }
      pendingUploadLease.current?.release()
      pendingUploadLease.current = lease
      setPendingUpload(resource)
      const cropBySize = Object.fromEntries(
        (['4:5', '3:4', '9:16'] as const).map((size) => [
          size,
          { focalX: 0.5, focalY: 0.5, zoom: 1 },
        ])
      ) as Record<InvitationPosterSize, BackgroundCrop>
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
          : current
      )
    } catch {
      toast.error(t('Use a PNG, JPEG, or WebP image up to 10 MiB'))
    }
  }

  async function downloadPoster() {
    const canvas = previewCanvasRef.current
    if (!canvas) return
    setDownloading(true)
    try {
      const blob = await canvasToPngBlob(canvas)
      downloadPngBlob(
        blob,
        createInvitationPosterFilename(props.systemName, props.code)
      )
    } catch {
      toast.error(t('Failed to download invitation poster'))
    } finally {
      setDownloading(false)
    }
  }

  function requestDialogClose() {
    if (mode === 'edit' && dirty) {
      setDiscardConfirmationOpen(true)
      return
    }
    props.onOpenChange(false)
  }

  const previewFrame = (
    <div
      className='bg-muted relative mx-auto w-full max-w-[320px] overflow-hidden rounded-md border shadow-sm'
      style={{ aspectRatio: `${dimensions.width} / ${dimensions.height}` }}
      role='img'
      aria-label={t('Invitation poster preview')}
    >
      <div ref={previewContainerRef} className='size-full' />
      {rendering ? (
        <div className='bg-background/75 absolute inset-0 grid place-items-center'>
          <LoaderCircle
            className='text-muted-foreground size-6 animate-spin'
            aria-label={t('Loading')}
          />
        </div>
      ) : null}
      {renderError ? (
        <div className='bg-background absolute inset-0 grid place-items-center p-6 text-center text-sm'>
          {t('Failed to render invitation poster')}
        </div>
      ) : null}
    </div>
  )

  let dialogBody = null
  if (mode === 'preview') {
    dialogBody = previewFrame
  } else if (draft) {
    dialogBody = (
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
    )
  }

  return (
    <>
      <Dialog
        open={props.open}
        onOpenChange={(open) => {
          if (open) props.onOpenChange(true)
          else requestDialogClose()
        }}
        title={t('Invitation poster')}
        contentClassName={mode === 'edit' ? 'sm:max-w-6xl' : 'sm:max-w-lg'}
        contentHeight={mode === 'edit' ? 'min(70vh, 48rem)' : 'auto'}
        bodyClassName='space-y-4'
        footer={
          mode === 'preview' ? (
            <div className='flex w-full items-center justify-end gap-2'>
              <Tooltip>
                <TooltipTrigger
                  render={
                    <Button
                      ref={pencilRef}
                      type='button'
                      variant='outline'
                      size='icon'
                      aria-label={t('Edit poster')}
                      onClick={enterEditMode}
                    />
                  }
                >
                  <Pencil aria-hidden='true' />
                </TooltipTrigger>
                <TooltipContent>{t('Edit poster')}</TooltipContent>
              </Tooltip>
              <Button
                onClick={() => void downloadPoster()}
                disabled={rendering || downloading || !previewCanvasRef.current}
                className='gap-2'
              >
                {rendering || downloading ? (
                  <LoaderCircle className='animate-spin' aria-hidden='true' />
                ) : (
                  <Download aria-hidden='true' />
                )}
                {t('Download PNG')}
              </Button>
            </div>
          ) : (
            <div className='flex w-full flex-wrap items-center justify-end gap-2'>
              <Button
                type='button'
                variant='ghost'
                className='mr-auto gap-2'
                onClick={resetDraft}
              >
                <RotateCcw aria-hidden='true' />
                {t('Reset poster')}
              </Button>
              <Button
                type='button'
                variant='outline'
                className='gap-2'
                onClick={cancelEditMode}
              >
                <X aria-hidden='true' />
                {t('Cancel')}
              </Button>
              <Button
                type='button'
                className='gap-2'
                onClick={() => void saveDraft()}
              >
                <Save aria-hidden='true' />
                {t('Save')}
              </Button>
            </div>
          )
        }
      >
        {dialogBody}

        <div
          className='pointer-events-none fixed top-0 -left-[10000px] size-[360px] overflow-hidden'
          aria-hidden='true'
        >
          <QRCodeCanvas
            ref={setQrCanvas}
            value={props.value}
            size={360}
            level='H'
            marginSize={4}
          />
        </div>
      </Dialog>

      <AlertDialog
        open={discardConfirmationOpen}
        onOpenChange={setDiscardConfirmationOpen}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('Discard poster changes?')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t('Poster changes will reset after reload')}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t('Keep editing')}</AlertDialogCancel>
            <AlertDialogAction
              variant='destructive'
              onClick={() => {
                setDiscardConfirmationOpen(false)
                pendingUploadLease.current?.release()
                pendingUploadLease.current = null
                setPendingUpload(null)
                setDraft(null)
                setMode('preview')
                props.onOpenChange(false)
              }}
            >
              {t('Discard changes')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  )
}
