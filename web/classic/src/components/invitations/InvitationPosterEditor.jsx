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
import React, { useRef } from 'react';
import {
  Button,
  Input,
  Radio,
  RadioGroup,
  Slider,
  Switch,
  Typography,
} from '@douyinfe/semi-ui';
import clsx from 'clsx';
import { Lock, Maximize2, Upload } from 'lucide-react';
import { useTranslation } from 'react-i18next';

import {
  POSTER_COPY_LIMITS,
  POSTER_MODULE_REGISTRY,
  POSTER_SIZES,
  clientPointToPosterPoint,
  constrainInvitationPosterTransform,
  moveInvitationPosterModule,
  normalizeInvitationPosterDesign,
  updateInvitationPosterModuleRect,
} from '../../../../shared/invitation-poster-design.js';
import { isInvitationPosterResolutionLow } from '../../../../shared/invitation-poster-upload.js';

const POSTER_SIZE_IDS = Object.keys(POSTER_SIZES);
const BUILTIN_BACKGROUNDS = [
  { id: 'clean-light', label: 'Clean light', color: '#F2F5F4' },
  { id: 'ink-contrast', label: 'Ink contrast', color: '#18221F' },
  { id: 'coral-paper', label: 'Coral paper', color: '#F3DDD5' },
];
const MODULE_LABELS = {
  brand: 'Brand',
  eyebrow: 'Eyebrow',
  inviter: 'Inviter',
  title: 'Title',
  subtitle: 'Subtitle',
  qr: 'QR code',
  inviteCode: 'Invitation code',
  domain: 'Domain',
  footer: 'Footer',
};
const COPY_FIELDS = ['eyebrow', 'title', 'subtitle', 'footer'];

export default function InvitationPosterEditor({
  design,
  scene,
  selectedModule,
  backgroundAsset,
  previewContainerRef,
  rendering,
  renderError,
  onDesignChange,
  onSelectedModuleChange,
  onUpload,
}) {
  const { t } = useTranslation();
  const fileInputRef = useRef(null);
  const dragRef = useRef(null);
  const dimensions = POSTER_SIZES[design.size];
  const uploadBackground =
    design.background.kind === 'upload' ? design.background : null;
  const sceneText = Object.fromEntries(
    scene.modules.map((item) => [item.id, item.text]),
  );
  const resolvedCopy = {
    eyebrow: sceneText.eyebrow || '',
    title: sceneText.title || '',
    subtitle: sceneText.subtitle || '',
    footer: sceneText.footer || '',
  };

  function updateRect(moduleId, from, proposed) {
    const result = constrainInvitationPosterTransform({
      design,
      size: design.size,
      moduleId,
      from,
      proposed,
      resolvedCopy,
    });
    onDesignChange(
      updateInvitationPosterModuleRect(
        design,
        design.size,
        moduleId,
        result.rect,
      ),
    );
  }

  function beginTransform(event, moduleId, mode) {
    const bounds = event.currentTarget
      .closest('[data-poster-stage]')
      ?.getBoundingClientRect();
    if (!bounds) return;
    onSelectedModuleChange(moduleId);
    event.currentTarget.setPointerCapture(event.pointerId);
    dragRef.current = {
      pointerId: event.pointerId,
      moduleId,
      mode,
      from: { ...design.layouts[design.size][moduleId] },
      start: clientPointToPosterPoint(
        event.clientX,
        event.clientY,
        bounds,
        dimensions,
      ),
    };
  }

  function continueTransform(event) {
    const drag = dragRef.current;
    if (!drag || drag.pointerId !== event.pointerId) return;
    const bounds = event.currentTarget
      .closest('[data-poster-stage]')
      ?.getBoundingClientRect();
    if (!bounds) return;
    const point = clientPointToPosterPoint(
      event.clientX,
      event.clientY,
      bounds,
      dimensions,
    );
    const dx = point.x - drag.start.x;
    const dy = point.y - drag.start.y;
    updateRect(
      drag.moduleId,
      drag.from,
      drag.mode === 'resize'
        ? {
            ...drag.from,
            width: drag.from.width + dx,
            height: drag.from.height + dy,
          }
        : { ...drag.from, x: drag.from.x + dx, y: drag.from.y + dy },
    );
  }

  function endTransform(event) {
    if (event.currentTarget.hasPointerCapture(event.pointerId)) {
      event.currentTarget.releasePointerCapture(event.pointerId);
    }
    dragRef.current = null;
  }

  function updateBackgroundCrop(next) {
    if (design.background.kind !== 'upload') return;
    const current = design.background.cropBySize[design.size];
    onDesignChange({
      ...design,
      background: {
        ...design.background,
        cropBySize: {
          ...design.background.cropBySize,
          [design.size]: { ...current, ...next },
        },
      },
    });
  }

  return (
    <div className='grid min-w-0 gap-4 lg:grid-cols-[minmax(0,1fr)_20rem]'>
      <div className='min-w-0'>
        <div
          data-poster-stage
          className='relative mx-auto max-h-[45vh] w-full overflow-hidden rounded-md border border-[var(--semi-color-border)] bg-[var(--semi-color-fill-0)] shadow-sm'
          style={{
            aspectRatio: `${dimensions.width} / ${dimensions.height}`,
            width: `min(100%, calc(45vh * ${dimensions.width} / ${dimensions.height}))`,
          }}
        >
          <div
            ref={previewContainerRef}
            className='size-full'
            aria-hidden='true'
          />
          {scene.modules.map((module) => {
            const selected = selectedModule === module.id;
            const rectStyle = {
              left: `${(module.rect.x / dimensions.width) * 100}%`,
              top: `${(module.rect.y / dimensions.height) * 100}%`,
              width: `${(module.rect.width / dimensions.width) * 100}%`,
              height: `${(module.rect.height / dimensions.height) * 100}%`,
            };
            return (
              <div
                key={module.id}
                className='pointer-events-none absolute'
                style={rectStyle}
              >
                <button
                  type='button'
                  className={clsx(
                    'pointer-events-auto absolute inset-0 min-h-11 min-w-11 touch-none border bg-transparent outline-none focus-visible:ring-2 focus-visible:ring-[var(--semi-color-primary)] focus-visible:ring-offset-2',
                    selected
                      ? 'border-[var(--semi-color-primary)] ring-2 ring-[var(--semi-color-primary-light-default)]'
                      : 'border-black/30 hover:border-[var(--semi-color-primary)]',
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
                    const step = event.shiftKey ? 10 : 1;
                    const movement = {
                      ArrowLeft: [-step, 0],
                      ArrowRight: [step, 0],
                      ArrowUp: [0, -step],
                      ArrowDown: [0, step],
                    }[event.key];
                    if (!movement) return;
                    event.preventDefault();
                    onDesignChange(
                      moveInvitationPosterModule(
                        design,
                        design.size,
                        module.id,
                        movement[0],
                        movement[1],
                        resolvedCopy,
                      ),
                    );
                  }}
                />
                {selected ? (
                  <button
                    type='button'
                    className='pointer-events-auto absolute -right-5 -bottom-5 grid min-h-11 min-w-11 touch-none place-items-center rounded-full border border-[var(--semi-color-border)] bg-[var(--semi-color-bg-0)] text-[var(--semi-color-text-0)] shadow-sm outline-none focus-visible:ring-2 focus-visible:ring-[var(--semi-color-primary)] focus-visible:ring-offset-2'
                    aria-label={t('Resize')}
                    onPointerDown={(event) =>
                      beginTransform(event, module.id, 'resize')
                    }
                    onPointerMove={continueTransform}
                    onPointerUp={endTransform}
                    onPointerCancel={endTransform}
                    onKeyDown={(event) => {
                      const step = event.shiftKey ? 10 : 1;
                      const resizeDirection = {
                        ArrowLeft: -step,
                        ArrowUp: -step,
                        ArrowRight: step,
                        ArrowDown: step,
                      }[event.key];
                      if (resizeDirection === undefined) return;
                      event.preventDefault();
                      const from = design.layouts[design.size][module.id];
                      updateRect(module.id, from, {
                        ...from,
                        width: from.width + resizeDirection,
                        height: from.height + resizeDirection,
                      });
                    }}
                  >
                    <Maximize2 size={16} aria-hidden='true' />
                  </button>
                ) : null}
              </div>
            );
          })}
          {rendering ? (
            <div className='pointer-events-none absolute inset-0 bg-white/65' />
          ) : null}
          {renderError ? (
            <div className='pointer-events-none absolute inset-0 grid place-items-center bg-[var(--semi-color-bg-0)]/90 p-6 text-center text-sm'>
              {t('Failed to render invitation poster')}
            </div>
          ) : null}
        </div>
      </div>

      <div className='min-w-0 space-y-5 lg:max-h-[58vh] lg:overflow-y-auto lg:border-l lg:border-[var(--semi-color-border)] lg:pl-4'>
        <section className='space-y-3'>
          <Typography.Text strong>{t('Poster size')}</Typography.Text>
          <RadioGroup
            type='button'
            value={design.size}
            onChange={(event) => {
              const next = event.target.value;
              if (Object.hasOwn(POSTER_SIZES, next)) {
                onDesignChange({ ...design, size: next });
              }
            }}
            aria-label={t('Poster size')}
            className='flex w-full'
          >
            {POSTER_SIZE_IDS.map((size) => (
              <Radio key={size} value={size} className='flex-1 text-center'>
                {size}
              </Radio>
            ))}
          </RadioGroup>
          <label className='flex min-h-9 items-center justify-between gap-3 border-y border-[var(--semi-color-border)] py-2 text-sm'>
            <span>{t('Show my nickname')}</span>
            <Switch
              checked={design.showNickname}
              onChange={(checked) =>
                onDesignChange({ ...design, showNickname: Boolean(checked) })
              }
              aria-label={t('Show my nickname')}
            />
          </label>
        </section>

        <section className='space-y-3 border-t border-[var(--semi-color-border)] pt-4'>
          <Typography.Text strong>{t('Poster background')}</Typography.Text>
          <div className='grid grid-cols-3 gap-2'>
            {BUILTIN_BACKGROUNDS.map((background) => {
              const selected =
                design.background.kind === 'builtin' &&
                design.background.id === background.id;
              return (
                <button
                  key={background.id}
                  type='button'
                  className={clsx(
                    'flex min-h-11 flex-col items-center justify-center gap-1 rounded-md border px-2 py-2 text-xs outline-none focus-visible:ring-2',
                    selected
                      ? 'border-[var(--semi-color-primary)]'
                      : 'border-[var(--semi-color-border)]',
                  )}
                  aria-pressed={selected}
                  onClick={() =>
                    onDesignChange({
                      ...design,
                      background: { kind: 'builtin', id: background.id },
                    })
                  }
                >
                  <span
                    className='size-5 rounded-sm border border-black/15'
                    style={{ backgroundColor: background.color }}
                    aria-hidden='true'
                  />
                  <span>{t(background.label)}</span>
                </button>
              );
            })}
          </div>
          <div className='grid grid-cols-[2.5rem_1fr] gap-2'>
            <input
              type='color'
              className='h-9 w-10 cursor-pointer rounded border border-[var(--semi-color-border)] bg-transparent p-1'
              value={
                design.background.kind === 'solid'
                  ? design.background.color
                  : '#F2F5F4'
              }
              aria-label={t('Solid color')}
              onChange={(event) =>
                onDesignChange({
                  ...design,
                  background: {
                    kind: 'solid',
                    color: event.target.value.toUpperCase(),
                  },
                })
              }
            />
            <Input
              value={
                design.background.kind === 'solid'
                  ? design.background.color
                  : '#F2F5F4'
              }
              aria-label={t('Solid color')}
              className='font-mono uppercase'
              onChange={(value) => {
                if (!/^#[0-9A-Fa-f]{6}$/.test(value)) return;
                onDesignChange({
                  ...design,
                  background: {
                    kind: 'solid',
                    color: value.toUpperCase(),
                  },
                });
              }}
            />
          </div>
          <input
            ref={fileInputRef}
            type='file'
            accept='image/png,image/jpeg,image/webp'
            className='sr-only'
            onChange={(event) => {
              const file = event.target.files?.[0];
              event.target.value = '';
              if (file) void onUpload(file);
            }}
          />
          <Button
            type='tertiary'
            theme='light'
            icon={<Upload size={16} />}
            className='w-full'
            onClick={() => fileInputRef.current?.click()}
          >
            {t('Local image')}
          </Button>
          {backgroundAsset &&
          isInvitationPosterResolutionLow(backgroundAsset, design.size) ? (
            <Typography.Text type='warning' size='small'>
              {t('This image may look soft at the selected size')}
            </Typography.Text>
          ) : null}
          {uploadBackground ? (
            <div className='space-y-3 border-t border-[var(--semi-color-border)] pt-3'>
              <Typography.Text strong>{t('Move background')}</Typography.Text>
              {['focalX', 'focalY'].map((axis) => (
                <div
                  key={axis}
                  className='grid grid-cols-[1rem_1fr] items-center gap-2'
                >
                  <span className='text-xs text-[var(--semi-color-text-2)]'>
                    {axis === 'focalX' ? 'X' : 'Y'}
                  </span>
                  <Slider
                    min={0}
                    max={1}
                    step={0.01}
                    value={uploadBackground.cropBySize[design.size][axis]}
                    aria-label={`${t('Move background')} ${axis === 'focalX' ? 'X' : 'Y'}`}
                    onChange={(value) =>
                      updateBackgroundCrop({ [axis]: value })
                    }
                  />
                </div>
              ))}
              <div className='space-y-2'>
                <Typography.Text>{t('Zoom')}</Typography.Text>
                <Slider
                  min={1}
                  max={4}
                  step={0.05}
                  value={uploadBackground.cropBySize[design.size].zoom}
                  aria-label={t('Zoom')}
                  onChange={(value) => updateBackgroundCrop({ zoom: value })}
                />
              </div>
            </div>
          ) : null}
        </section>

        <section className='space-y-3 border-t border-[var(--semi-color-border)] pt-4'>
          <Typography.Text strong>{t('Poster modules')}</Typography.Text>
          <div className='grid grid-cols-2 gap-2'>
            {Object.keys(POSTER_MODULE_REGISTRY).map((moduleId) => (
              <button
                key={moduleId}
                type='button'
                className={clsx(
                  'flex min-h-11 min-w-0 items-center gap-2 rounded-md border px-2 text-left text-xs',
                  selectedModule === moduleId
                    ? 'border-[var(--semi-color-primary)] bg-[var(--semi-color-primary-light-default)]'
                    : 'border-[var(--semi-color-border)]',
                )}
                aria-pressed={selectedModule === moduleId}
                onClick={() => onSelectedModuleChange(moduleId)}
              >
                {POSTER_MODULE_REGISTRY[moduleId].mandatory ? (
                  <Lock size={14} className='shrink-0' aria-hidden='true' />
                ) : null}
                <span className='truncate'>{t(MODULE_LABELS[moduleId])}</span>
              </button>
            ))}
          </div>
        </section>

        <section className='space-y-3 border-t border-[var(--semi-color-border)] pt-4'>
          {COPY_FIELDS.map((field) => {
            const preference = design.copy[field];
            const value =
              preference.source === 'custom'
                ? preference.value
                : resolvedCopy[field];
            return (
              <div key={field} className='space-y-1.5'>
                <div className='flex items-center justify-between gap-3'>
                  <Typography.Text>{t(MODULE_LABELS[field])}</Typography.Text>
                  <span className='text-xs tabular-nums text-[var(--semi-color-text-2)]'>
                    {[...value].length}/{POSTER_COPY_LIMITS[field]}
                  </span>
                </div>
                <Input
                  value={value}
                  aria-label={t(MODULE_LABELS[field])}
                  maxLength={POSTER_COPY_LIMITS[field] * 2}
                  onChange={(nextValue) => {
                    const next = [...nextValue]
                      .slice(0, POSTER_COPY_LIMITS[field])
                      .join('');
                    onDesignChange(
                      normalizeInvitationPosterDesign({
                        ...design,
                        copy: {
                          ...design.copy,
                          [field]: { source: 'custom', value: next },
                        },
                      }).design,
                    );
                  }}
                />
              </div>
            );
          })}
        </section>
      </div>
    </div>
  );
}
