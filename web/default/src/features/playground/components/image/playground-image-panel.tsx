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
  CopyIcon,
  DownloadIcon,
  ImageIcon,
  Loader2Icon,
  RefreshCwIcon,
} from 'lucide-react'
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Combobox } from '@/components/ui/combobox'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Textarea } from '@/components/ui/textarea'
import { useCopyToClipboard } from '@/hooks/use-copy-to-clipboard'

import { getUserImageModels, sendImageGeneration } from '../../api'
import {
  buildImageGenerationRequest,
  createImagePreviews,
  getImageGenerationErrorMessage,
  getImageGenerationCapabilities,
  revokeImagePreviews,
} from '../../lib'
import type { ImagePreview } from '../../lib/image/image-generation'
import type { GroupOption, ModelOption, PlaygroundImageSize } from '../../types'

const IMAGE_SIZES: PlaygroundImageSize[] = [
  '1024x1024',
  '1024x1536',
  '1536x1024',
  '1024x1792',
  '1792x1024',
]

const IMAGE_OPTION_LABELS: Record<string, string> = {
  auto: 'Auto',
  low: 'Low',
  medium: 'Medium',
  high: 'High',
  standard: 'Standard',
  hd: 'HD',
  vivid: 'Vivid',
  natural: 'Natural',
  opaque: 'Opaque',
  transparent: 'Transparent',
}

function getImageOptionLabel(value: string): string {
  return IMAGE_OPTION_LABELS[value] || value
}

function clampImageCount(value: number): number {
  if (!Number.isFinite(value)) return 1
  return Math.min(4, Math.max(1, Math.trunc(value)))
}

type PlaygroundImagePanelProps = {
  groups: GroupOption[]
  initialGroup: string
}

export function PlaygroundImagePanel({
  groups,
  initialGroup,
}: PlaygroundImagePanelProps) {
  const { t } = useTranslation()
  const { copyToClipboard } = useCopyToClipboard()
  const [group, setGroup] = useState(initialGroup)
  const [model, setModel] = useState('')
  const [models, setModels] = useState<ModelOption[]>([])
  const [modelsLoading, setModelsLoading] = useState(false)
  const [modelsError, setModelsError] = useState('')
  const [prompt, setPrompt] = useState('')
  const [size, setSize] = useState<PlaygroundImageSize>('1024x1024')
  const [count, setCount] = useState(1)
  const [quality, setQuality] = useState('')
  const [style, setStyle] = useState('')
  const [background, setBackground] = useState('auto')
  const [images, setImages] = useState<ImagePreview[]>([])
  const [generationError, setGenerationError] = useState('')
  const [isGenerating, setIsGenerating] = useState(false)
  const abortRef = useRef<AbortController | null>(null)
  const modelAbortRef = useRef<AbortController | null>(null)
  const imagesRef = useRef<ImagePreview[]>([])
  const capabilities = useMemo(
    () => getImageGenerationCapabilities(model),
    [model]
  )

  const replaceImages = useCallback((next: ImagePreview[]) => {
    revokeImagePreviews(imagesRef.current)
    imagesRef.current = next
    setImages(next)
  }, [])

  const loadModels = useCallback(async () => {
    modelAbortRef.current?.abort()
    setModels([])
    setModel('')
    if (!group) {
      setModelsLoading(false)
      return
    }

    const abortController = new AbortController()
    modelAbortRef.current = abortController
    setModelsLoading(true)
    setModelsError('')
    try {
      const nextModels = await getUserImageModels(group, abortController.signal)
      if (abortController.signal.aborted) return
      setModels(nextModels)
      setModel(nextModels[0]?.value || '')
    } catch (error) {
      if (abortController.signal.aborted) return
      setModels([])
      setModel('')
      setModelsError(
        error instanceof Error ? error.message : t('Failed to load models')
      )
    } finally {
      if (modelAbortRef.current === abortController) {
        modelAbortRef.current = null
        setModelsLoading(false)
      }
    }
  }, [group, t])

  const handleGroupChange = useCallback(
    (value: string | null) => {
      if (!value || value === group) return
      modelAbortRef.current?.abort()
      setModels([])
      setModel('')
      setModelsError('')
      setGroup(value)
    },
    [group]
  )

  useEffect(() => {
    if (!group && groups.length > 0) {
      setGroup(initialGroup || groups[0].value)
    }
  }, [group, groups, initialGroup])

  useEffect(() => {
    void loadModels()
  }, [loadModels])

  useEffect(() => {
    setQuality(capabilities.quality[0] || '')
    setStyle(capabilities.style[0] || '')
    if (!capabilities.background) setBackground('auto')
  }, [capabilities])

  useEffect(
    () => () => {
      abortRef.current?.abort()
      modelAbortRef.current?.abort()
      revokeImagePreviews(imagesRef.current)
      imagesRef.current = []
    },
    []
  )

  async function handleGenerate() {
    const trimmedPrompt = prompt.trim()
    if (!trimmedPrompt || !model || isGenerating) return

    const abortController = new AbortController()
    abortRef.current = abortController
    setGenerationError('')
    setIsGenerating(true)

    try {
      const response = await sendImageGeneration(
        buildImageGenerationRequest({
          background,
          count,
          group,
          model,
          prompt: trimmedPrompt,
          quality,
          size,
          style,
        }),
        abortController.signal
      )
      if (abortController.signal.aborted) return
      replaceImages(
        createImagePreviews(response.data ?? [], { prompt: trimmedPrompt })
      )
    } catch (error) {
      if (abortController.signal.aborted) return
      const errorMessage = t(
        getImageGenerationErrorMessage(error, t('Unknown error'))
      )
      setGenerationError(errorMessage)
      toast.error(t('Image generation failed'), {
        description: errorMessage,
      })
    } finally {
      if (abortRef.current === abortController) {
        abortRef.current = null
        setIsGenerating(false)
      }
    }
  }

  function handleStop() {
    abortRef.current?.abort()
    abortRef.current = null
    setIsGenerating(false)
  }

  async function copyImageURL(url: string) {
    await copyToClipboard(url)
  }

  return (
    <div className='flex min-h-0 flex-1 flex-col overflow-hidden'>
      <div className='min-h-0 flex-1 overflow-y-auto'>
        <div className='mx-auto grid w-full max-w-4xl gap-4 px-4 py-4'>
          <div className='border-border bg-card/40 grid gap-4 rounded-lg border p-4'>
            <div className='grid gap-2'>
              <Label htmlFor='playground-image-prompt'>{t('Prompt')}</Label>
              <Textarea
                id='playground-image-prompt'
                className='min-h-28 resize-none'
                disabled={isGenerating}
                onChange={(event) => setPrompt(event.target.value)}
                placeholder={t('Describe the image to generate')}
                value={prompt}
              />
            </div>

            <div className='grid gap-3 sm:grid-cols-2 lg:grid-cols-4'>
              <div className='grid gap-2'>
                <Label>{t('Group')}</Label>
                <Select value={group} onValueChange={handleGroupChange}>
                  <SelectTrigger
                    aria-label={t('Group')}
                    className='w-full'
                    disabled={isGenerating}
                  >
                    <SelectValue placeholder={t('Select group')} />
                  </SelectTrigger>
                  <SelectContent>
                    {groups.map((option) => (
                      <SelectItem key={option.value} value={option.value}>
                        {option.label}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>

              <div className='grid gap-2 sm:col-span-1 lg:col-span-2'>
                <Label htmlFor='playground-image-model'>{t('Model')}</Label>
                <Combobox
                  id='playground-image-model'
                  ariaLabel={t('Model')}
                  disabled={
                    isGenerating || modelsLoading || models.length === 0
                  }
                  emptyText={t('No models are available for this group.')}
                  onValueChange={(value) => value && setModel(value)}
                  openOnFocus
                  options={models}
                  placeholder={t('Select model')}
                  searchPlaceholder={t('Search models...')}
                  value={model}
                />
              </div>

              <div className='grid gap-2'>
                <Label htmlFor='playground-image-count'>{t('Count')}</Label>
                <Input
                  id='playground-image-count'
                  disabled={isGenerating}
                  max={4}
                  min={1}
                  onChange={(event) =>
                    setCount(clampImageCount(Number(event.target.value)))
                  }
                  type='number'
                  value={count}
                />
              </div>

              <div className='grid gap-2'>
                <Label>{t('Size')}</Label>
                <Select
                  value={size}
                  onValueChange={(value) =>
                    value && setSize(value as PlaygroundImageSize)
                  }
                >
                  <SelectTrigger
                    aria-label={t('Size')}
                    className='w-full'
                    disabled={isGenerating}
                  >
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {IMAGE_SIZES.map((value) => (
                      <SelectItem key={value} value={value}>
                        {value}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>

              {capabilities.quality.length > 0 ? (
                <div className='grid gap-2'>
                  <Label>{t('Quality')}</Label>
                  <Select
                    value={quality}
                    onValueChange={(value) => value && setQuality(value)}
                  >
                    <SelectTrigger
                      aria-label={t('Quality')}
                      className='w-full'
                      disabled={isGenerating}
                    >
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {capabilities.quality.map((value) => (
                        <SelectItem key={value} value={value}>
                          {t(getImageOptionLabel(value))}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
              ) : null}

              {capabilities.style.length > 0 ? (
                <div className='grid gap-2'>
                  <Label>{t('Style')}</Label>
                  <Select
                    value={style}
                    onValueChange={(value) => value && setStyle(value)}
                  >
                    <SelectTrigger
                      aria-label={t('Style')}
                      className='w-full'
                      disabled={isGenerating}
                    >
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {capabilities.style.map((value) => (
                        <SelectItem key={value} value={value}>
                          {t(getImageOptionLabel(value))}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
              ) : null}

              {capabilities.background ? (
                <div className='grid gap-2'>
                  <Label>{t('Background')}</Label>
                  <Select
                    value={background}
                    onValueChange={(value) => value && setBackground(value)}
                  >
                    <SelectTrigger
                      aria-label={t('Background')}
                      className='w-full'
                      disabled={isGenerating}
                    >
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {['auto', 'opaque', 'transparent'].map((value) => (
                        <SelectItem key={value} value={value}>
                          {t(getImageOptionLabel(value))}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
              ) : null}
            </div>

            {modelsLoading ? (
              <p className='text-muted-foreground flex items-center gap-2 text-sm'>
                <Loader2Icon className='size-4 animate-spin' />
                {t('Loading models...')}
              </p>
            ) : null}
            {!modelsLoading && modelsError ? (
              <div className='flex items-center justify-between gap-3 text-sm'>
                <p className='text-destructive'>{modelsError}</p>
                <Button
                  onClick={() => void loadModels()}
                  size='sm'
                  type='button'
                  variant='outline'
                >
                  <RefreshCwIcon className='size-4' />
                  {t('Retry')}
                </Button>
              </div>
            ) : null}
            {!modelsLoading && !modelsError && models.length === 0 ? (
              <p className='text-muted-foreground text-sm'>
                {t('No models are available for this group.')}
              </p>
            ) : null}

            {generationError ? (
              <Alert variant='destructive'>
                <AlertTitle>{t('Image generation failed')}</AlertTitle>
                <AlertDescription className='break-words'>
                  {generationError}
                </AlertDescription>
                <Button
                  className='mt-2 w-fit'
                  disabled={isGenerating}
                  onClick={() => void handleGenerate()}
                  size='sm'
                  type='button'
                  variant='outline'
                >
                  <RefreshCwIcon className='size-4' />
                  {t('Retry')}
                </Button>
              </Alert>
            ) : null}

            <div className='flex justify-end gap-2'>
              {isGenerating ? (
                <Button onClick={handleStop} type='button' variant='outline'>
                  {t('Stop')}
                </Button>
              ) : null}
              <Button
                disabled={
                  !prompt.trim() || !model || isGenerating || modelsLoading
                }
                onClick={() => void handleGenerate()}
                type='button'
              >
                {isGenerating ? (
                  <Loader2Icon className='size-4 animate-spin' />
                ) : (
                  <ImageIcon className='size-4' />
                )}
                {t('Generate')}
              </Button>
            </div>
          </div>

          {images.length > 0 ? (
            <div className='grid gap-3 sm:grid-cols-2'>
              {images.map((image, index) => (
                <div
                  className='border-border bg-card/40 overflow-hidden rounded-lg border'
                  key={image.id}
                >
                  {image.src ? (
                    <img
                      alt={image.displayPrompt}
                      className='bg-muted aspect-square w-full object-contain'
                      src={image.src}
                    />
                  ) : (
                    <div className='text-muted-foreground flex aspect-square items-center justify-center text-sm'>
                      {t('No preview available')}
                    </div>
                  )}
                  <div className='flex items-center justify-between gap-2 p-2'>
                    <span className='text-muted-foreground min-w-0 truncate text-xs'>
                      {image.displayPrompt}
                    </span>
                    <div className='flex shrink-0 items-center gap-1'>
                      {image.url ? (
                        <Button
                          aria-label={t('Copy URL')}
                          onClick={() => void copyImageURL(image.url || '')}
                          size='icon-sm'
                          type='button'
                          variant='ghost'
                        >
                          <CopyIcon className='size-4' />
                        </Button>
                      ) : null}
                      {image.src ? (
                        <Button
                          aria-label={t('Download')}
                          render={
                            <a
                              href={image.src}
                              download={`playground-image-${index + 1}.png`}
                            />
                          }
                          size='icon-sm'
                          variant='ghost'
                        >
                          <DownloadIcon className='size-4' />
                        </Button>
                      ) : null}
                    </div>
                  </div>
                </div>
              ))}
            </div>
          ) : null}
        </div>
      </div>
    </div>
  )
}
