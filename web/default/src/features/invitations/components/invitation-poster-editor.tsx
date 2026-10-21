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
import { Lock, Maximize2, Upload } from 'lucide-react'
import {
  type PointerEvent as ReactPointerEvent,
  type RefObject,
  useRef,
} from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Slider } from '@/components/ui/slider'
import { Switch } from '@/components/ui/switch'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'
import { cn } from '@/lib/utils'

import {
  POSTER_COPY_LIMITS,
  POSTER_MODULE_REGISTRY,
  POSTER_SIZES,
  clientPointToPosterPoint,
  constrainInvitationPosterTransform,
  moveInvitationPosterModule,
  normalizeInvitationPosterDesign,
  updateInvitationPosterModuleRect,
  type BackgroundCrop,
  type InvitationPosterDesignV2,
  type InvitationPosterSize,
  type ModuleRect,
  type PosterCopyField,
  type PosterModuleId,
} from '../../../../../shared/invitation-poster-design.js'
import type { InvitationPosterScene } from '../../../../../shared/invitation-poster-scene.js'
import type { InvitationPosterBackgroundResource } from '../../../../../shared/invitation-poster-storage.js'
import { isInvitationPosterResolutionLow } from '../../../../../shared/invitation-poster-upload.js'

type InvitationPosterEditorProps = {
  design: InvitationPosterDesignV2
  scene: InvitationPosterScene
  selectedModule: PosterModuleId | null
  backgroundAsset: InvitationPosterBackgroundResource | null
  previewContainerRef: RefObject<HTMLDivElement | null>
  rendering: boolean
  renderError: boolean
  onDesignChange: (design: InvitationPosterDesignV2) => void
  onSelectedModuleChange: (moduleId: PosterModuleId) => void
  onUpload: (file: File) => Promise<void>
}

type DragState = {
  pointerId: number
  moduleId: PosterModuleId
  mode: 'move' | 'resize'
  from: ModuleRect
  start: { x: number; y: number }
}

const POSTER_SIZE_IDS = Object.keys(POSTER_SIZES) as InvitationPosterSize[]
const BUILTIN_BACKGROUNDS = [
  { id: 'clean-light', label: 'Clean light', color: '#F2F5F4' },
  { id: 'ink-contrast', label: 'Ink contrast', color: '#18221F' },
  { id: 'coral-paper', label: 'Coral paper', color: '#F3DDD5' },
] as const
const MODULE_LABELS: Record<PosterModuleId, string> = {
  brand: 'Brand',
  eyebrow: 'Eyebrow',
  inviter: 'Inviter',
  title: 'Title',
  subtitle: 'Subtitle',
  qr: 'QR code',
  inviteCode: 'Invitation code',
  domain: 'Domain',
  footer: 'Footer',
}
const COPY_FIELDS: PosterCopyField[] = [
  'eyebrow',
  'title',
  'subtitle',
  'footer',
]

export function InvitationPosterEditor(props: InvitationPosterEditorProps) {
  const { t } = useTranslation()
  const fileInputRef = useRef<HTMLInputElement>(null)
  const dragRef = useRef<DragState | null>(null)
  const dimensions = POSTER_SIZES[props.design.size]
  const uploadBackground =
    props.design.background.kind === 'upload' ? props.design.background : null
  const sceneText = Object.fromEntries(
    props.scene.modules.map((item) => [item.id, item.text])
  ) as Record<PosterModuleId, string>
  const resolvedCopy = {
    eyebrow: sceneText.eyebrow || '',
    title: sceneText.title || '',
    subtitle: sceneText.subtitle || '',
    footer: sceneText.footer || '',
  }

  const updateRect = (
    moduleId: PosterModuleId,
    from: ModuleRect,
    proposed: ModuleRect
  ) => {
    const result = constrainInvitationPosterTransform({
      design: props.design,
      size: props.design.size,
      moduleId,
      from,
      proposed,
      resolvedCopy,
    })
    props.onDesignChange(
      updateInvitationPosterModuleRect(
        props.design,
        props.design.size,
        moduleId,
        result.rect
      )
    )
  }

  const beginTransform = (
    event: ReactPointerEvent<HTMLButtonElement>,
    moduleId: PosterModuleId,
    mode: DragState['mode']
  ) => {
    const bounds = event.currentTarget
      .closest('[data-poster-stage]')
      ?.getBoundingClientRect()
    if (!bounds) return
    props.onSelectedModuleChange(moduleId)
    event.currentTarget.setPointerCapture(event.pointerId)
    dragRef.current = {
      pointerId: event.pointerId,
      moduleId,
      mode,
      from: { ...props.design.layouts[props.design.size][moduleId] },
      start: clientPointToPosterPoint(
        event.clientX,
        event.clientY,
        bounds,
        dimensions
      ),
    }
  }

  const continueTransform = (event: ReactPointerEvent<HTMLButtonElement>) => {
    const drag = dragRef.current
    if (!drag || drag.pointerId !== event.pointerId) return
    const bounds = event.currentTarget
      .closest('[data-poster-stage]')
      ?.getBoundingClientRect()
    if (!bounds) return
    const point = clientPointToPosterPoint(
      event.clientX,
      event.clientY,
      bounds,
      dimensions
    )
    const dx = point.x - drag.start.x
    const dy = point.y - drag.start.y
    updateRect(
      drag.moduleId,
      drag.from,
      drag.mode === 'resize'
        ? {
            ...drag.from,
            width: drag.from.width + dx,
            height: drag.from.height + dy,
          }
        : { ...drag.from, x: drag.from.x + dx, y: drag.from.y + dy }
    )
  }

  const endTransform = (event: ReactPointerEvent<HTMLButtonElement>) => {
    if (event.currentTarget.hasPointerCapture(event.pointerId)) {
      event.currentTarget.releasePointerCapture(event.pointerId)
    }
    dragRef.current = null
  }

  const updateBackgroundCrop = (next: Partial<BackgroundCrop>) => {
    if (props.design.background.kind !== 'upload') return
    const current = props.design.background.cropBySize[props.design.size]
    props.onDesignChange({
      ...props.design,
      background: {
        ...props.design.background,
        cropBySize: {
          ...props.design.background.cropBySize,
          [props.design.size]: { ...current, ...next },
        },
      },
    })
  }

  return (
    <div className='grid min-w-0 gap-4 lg:grid-cols-[minmax(0,1fr)_20rem]'>
      <div className='min-w-0'>
        <div
          data-poster-stage
          className='bg-muted relative mx-auto max-h-[45vh] w-full overflow-hidden rounded-md border shadow-sm'
          style={{
            aspectRatio: `${dimensions.width} / ${dimensions.height}`,
            width: `min(100%, calc(45vh * ${dimensions.width} / ${dimensions.height}))`,
          }}
        >
          <div
            ref={props.previewContainerRef}
            className='size-full'
            aria-hidden='true'
          />
          {props.scene.modules.map((module) => {
            const selected = props.selectedModule === module.id
            const rectStyle = {
              left: `${(module.rect.x / dimensions.width) * 100}%`,
              top: `${(module.rect.y / dimensions.height) * 100}%`,
              width: `${(module.rect.width / dimensions.width) * 100}%`,
              height: `${(module.rect.height / dimensions.height) * 100}%`,
            }
            return (
              <div
                key={module.id}
                className='pointer-events-none absolute'
                style={rectStyle}
              >
                <button
                  type='button'
                  className={cn(
                    'pointer-events-auto absolute inset-0 min-h-11 min-w-11 touch-none border bg-transparent outline-none',
                    selected
                      ? 'border-primary ring-primary/35 ring-2'
                      : 'border-foreground/30 hover:border-primary/70'
                  )}
                  aria-label={t(MODULE_LABELS[module.id])}
                  aria-selected={selected}
                  onPointerDown={(event) =>
                    beginTransform(event, module.id, 'move')
                  }
                  onPointerMove={continueTransform}
                  onPointerUp={endTransform}
                  onPointerCancel={endTransform}
                  onKeyDown={(event) => {
                    const step = event.shiftKey ? 10 : 1
                    const movement = {
                      ArrowLeft: [-step, 0],
                      ArrowRight: [step, 0],
                      ArrowUp: [0, -step],
                      ArrowDown: [0, step],
                    }[event.key]
                    if (!movement) return
                    event.preventDefault()
                    props.onDesignChange(
                      moveInvitationPosterModule(
                        props.design,
                        props.design.size,
                        module.id,
                        movement[0],
                        movement[1],
                        resolvedCopy
                      )
                    )
                  }}
                />
                {selected ? (
                  <button
                    type='button'
                    className='bg-background text-foreground pointer-events-auto absolute -right-5 -bottom-5 grid min-h-11 min-w-11 touch-none place-items-center rounded-full border shadow-sm'
                    aria-label={t('Resize')}
                    onPointerDown={(event) =>
                      beginTransform(event, module.id, 'resize')
                    }
                    onPointerMove={continueTransform}
                    onPointerUp={endTransform}
                    onPointerCancel={endTransform}
                  >
                    <Maximize2 className='size-4' aria-hidden='true' />
                  </button>
                ) : null}
              </div>
            )
          })}
          {props.rendering ? (
            <div className='bg-background/65 pointer-events-none absolute inset-0' />
          ) : null}
          {props.renderError ? (
            <div className='bg-background/90 pointer-events-none absolute inset-0 grid place-items-center p-6 text-center text-sm'>
              {t('Failed to render invitation poster')}
            </div>
          ) : null}
        </div>
      </div>

      <div className='min-w-0 space-y-5 lg:max-h-[58vh] lg:overflow-y-auto lg:border-l lg:pl-4'>
        <section className='space-y-3'>
          <Label>{t('Poster size')}</Label>
          <ToggleGroup
            value={[props.design.size]}
            onValueChange={(values) => {
              const next = values.find((value) => value !== props.design.size)
              if (next && Object.hasOwn(POSTER_SIZES, next)) {
                props.onDesignChange({
                  ...props.design,
                  size: next as InvitationPosterSize,
                })
              }
            }}
            aria-label={t('Poster size')}
            variant='outline'
            spacing={0}
            className='w-full'
          >
            {POSTER_SIZE_IDS.map((size) => (
              <ToggleGroupItem key={size} value={size} className='h-9 flex-1'>
                {size}
              </ToggleGroupItem>
            ))}
          </ToggleGroup>
          <label className='flex min-h-9 items-center justify-between gap-3 border-y py-2 text-sm'>
            <span>{t('Show my nickname')}</span>
            <Switch
              checked={props.design.showNickname}
              onCheckedChange={(checked) =>
                props.onDesignChange({
                  ...props.design,
                  showNickname: Boolean(checked),
                })
              }
              aria-label={t('Show my nickname')}
            />
          </label>
        </section>

        <section className='space-y-3 border-t pt-4'>
          <Label>{t('Poster background')}</Label>
          <div className='grid grid-cols-3 gap-2'>
            {BUILTIN_BACKGROUNDS.map((background) => (
              <button
                key={background.id}
                type='button'
                className={cn(
                  'focus-visible:ring-ring flex min-h-11 flex-col items-center justify-center gap-1 rounded-md border px-2 py-2 text-xs outline-none focus-visible:ring-2',
                  props.design.background.kind === 'builtin' &&
                    props.design.background.id === background.id
                    ? 'border-primary'
                    : 'border-input'
                )}
                aria-pressed={
                  props.design.background.kind === 'builtin' &&
                  props.design.background.id === background.id
                }
                onClick={() =>
                  props.onDesignChange({
                    ...props.design,
                    background: { kind: 'builtin', id: background.id },
                  })
                }
              >
                <span
                  className='size-5 rounded-sm border'
                  style={{ backgroundColor: background.color }}
                  aria-hidden='true'
                />
                <span>{t(background.label)}</span>
              </button>
            ))}
          </div>
          <div className='grid grid-cols-[2.5rem_1fr] gap-2'>
            <input
              type='color'
              className='h-9 w-10 cursor-pointer rounded border bg-transparent p-1'
              value={
                props.design.background.kind === 'solid'
                  ? props.design.background.color
                  : '#F2F5F4'
              }
              aria-label={t('Solid color')}
              onChange={(event) =>
                props.onDesignChange({
                  ...props.design,
                  background: {
                    kind: 'solid',
                    color: event.target.value.toUpperCase() as `#${string}`,
                  },
                })
              }
            />
            <Input
              value={
                props.design.background.kind === 'solid'
                  ? props.design.background.color
                  : '#F2F5F4'
              }
              aria-label={t('Solid color')}
              className='h-9 font-mono uppercase'
              onChange={(event) => {
                if (!/^#[0-9A-Fa-f]{6}$/.test(event.target.value)) return
                props.onDesignChange({
                  ...props.design,
                  background: {
                    kind: 'solid',
                    color: event.target.value.toUpperCase() as `#${string}`,
                  },
                })
              }}
            />
          </div>
          <input
            ref={fileInputRef}
            type='file'
            accept='image/png,image/jpeg,image/webp'
            className='sr-only'
            onChange={(event) => {
              const file = event.target.files?.[0]
              event.target.value = ''
              if (file) void props.onUpload(file)
            }}
          />
          <Button
            type='button'
            variant='outline'
            className='w-full gap-2'
            onClick={() => fileInputRef.current?.click()}
          >
            <Upload className='size-4' aria-hidden='true' />
            {t('Local image')}
          </Button>
          {props.backgroundAsset &&
          isInvitationPosterResolutionLow(
            props.backgroundAsset,
            props.design.size
          ) ? (
            <p className='text-warning text-xs'>
              {t('This image may look soft at the selected size')}
            </p>
          ) : null}
          {uploadBackground ? (
            <div className='space-y-3 border-t pt-3'>
              <Label>{t('Move background')}</Label>
              {(['focalX', 'focalY'] as const).map((axis) => (
                <div
                  key={axis}
                  className='grid grid-cols-[1rem_1fr] items-center gap-2'
                >
                  <span className='text-muted-foreground text-xs'>
                    {axis === 'focalX' ? 'X' : 'Y'}
                  </span>
                  <Slider
                    min={0}
                    max={1}
                    step={0.01}
                    value={[
                      uploadBackground.cropBySize[props.design.size][axis],
                    ]}
                    aria-label={`${t('Move background')} ${axis === 'focalX' ? 'X' : 'Y'}`}
                    onValueChange={(value) =>
                      updateBackgroundCrop({
                        [axis]: Array.isArray(value) ? value[0] : value,
                      })
                    }
                  />
                </div>
              ))}
              <div className='space-y-2'>
                <Label>{t('Zoom')}</Label>
                <Slider
                  min={1}
                  max={4}
                  step={0.05}
                  value={[uploadBackground.cropBySize[props.design.size].zoom]}
                  aria-label={t('Zoom')}
                  onValueChange={(value) =>
                    updateBackgroundCrop({
                      zoom: Array.isArray(value) ? value[0] : value,
                    })
                  }
                />
              </div>
            </div>
          ) : null}
        </section>

        <section className='space-y-3 border-t pt-4'>
          <Label>{t('Poster modules')}</Label>
          <div className='grid grid-cols-2 gap-2'>
            {(Object.keys(POSTER_MODULE_REGISTRY) as PosterModuleId[]).map(
              (moduleId) => (
                <button
                  key={moduleId}
                  type='button'
                  className={cn(
                    'flex min-h-11 min-w-0 items-center gap-2 rounded-md border px-2 text-left text-xs',
                    props.selectedModule === moduleId
                      ? 'border-primary bg-primary/5'
                      : 'border-input'
                  )}
                  aria-pressed={props.selectedModule === moduleId}
                  onClick={() => props.onSelectedModuleChange(moduleId)}
                >
                  {POSTER_MODULE_REGISTRY[moduleId].mandatory ? (
                    <Lock className='size-3.5 shrink-0' aria-hidden='true' />
                  ) : null}
                  <span className='truncate'>{t(MODULE_LABELS[moduleId])}</span>
                </button>
              )
            )}
          </div>
        </section>

        <section className='space-y-3 border-t pt-4'>
          {COPY_FIELDS.map((field) => {
            const preference = props.design.copy[field]
            const value =
              preference.source === 'custom'
                ? preference.value
                : resolvedCopy[field]
            return (
              <div key={field} className='space-y-1.5'>
                <div className='flex items-center justify-between gap-3'>
                  <Label htmlFor={`poster-copy-${field}`}>
                    {t(MODULE_LABELS[field])}
                  </Label>
                  <span className='text-muted-foreground text-xs tabular-nums'>
                    {[...value].length}/{POSTER_COPY_LIMITS[field]}
                  </span>
                </div>
                <Input
                  id={`poster-copy-${field}`}
                  value={value}
                  maxLength={POSTER_COPY_LIMITS[field] * 2}
                  onChange={(event) => {
                    const next = [...event.target.value]
                      .slice(0, POSTER_COPY_LIMITS[field])
                      .join('')
                    props.onDesignChange(
                      normalizeInvitationPosterDesign({
                        ...props.design,
                        copy: {
                          ...props.design.copy,
                          [field]: { source: 'custom', value: next },
                        },
                      }).design
                    )
                  }}
                />
              </div>
            )
          })}
        </section>
      </div>
    </div>
  )
}
