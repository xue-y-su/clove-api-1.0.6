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
import { FileTextIcon, ImageIcon, Loader2Icon, XIcon } from 'lucide-react'
import {
  type ChangeEvent,
  type ClipboardEvent,
  useCallback,
  useRef,
  useState,
} from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import {
  PromptInput,
  PromptInputHeader,
  PromptInputFooter,
  PromptInputTextarea,
  type PromptInputMessage,
} from '@/components/ai-elements/prompt-input'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'

import {
  captureDisplayScreenshotFile,
  formatAttachmentSize,
  readPlaygroundAttachments,
  getSubmittableInputText,
} from '../../lib'
import type {
  ModelOption,
  GroupOption,
  PlaygroundAttachment,
  ParameterEnabled,
  PlaygroundConfig,
  PlaygroundWebSearchMode,
} from '../../types'
import { PlaygroundInputControls } from './playground-input-controls'
import { PlaygroundInputTools } from './playground-input-tools'

interface PlaygroundInputProps {
  config: PlaygroundConfig
  onSubmit: (text: string, attachments?: PlaygroundAttachment[]) => void
  onStop?: () => void
  disabled?: boolean
  isGenerating?: boolean
  models: ModelOption[]
  modelValue: string
  onModelChange: (value: string) => void
  isModelLoading?: boolean
  webSearchMode?: PlaygroundWebSearchMode
  webSearchAvailable?: boolean
  onWebSearchChange?: (mode: PlaygroundWebSearchMode) => void
  groups: GroupOption[]
  groupValue: string
  onGroupChange: (value: string) => void
  hasMessages?: boolean
  onConfigChange: <K extends keyof PlaygroundConfig>(
    key: K,
    value: PlaygroundConfig[K]
  ) => void
  onClearMessages?: () => void
  onParameterEnabledChange: (
    key: keyof ParameterEnabled,
    value: boolean
  ) => void
  parameterEnabled: ParameterEnabled
}

export function PlaygroundInput({
  config,
  onSubmit,
  onStop,
  disabled,
  isGenerating,
  models,
  modelValue,
  onModelChange,
  isModelLoading = false,
  webSearchMode = 'off',
  webSearchAvailable = false,
  onWebSearchChange,
  groups,
  groupValue,
  onGroupChange,
  hasMessages = false,
  onConfigChange,
  onClearMessages,
  onParameterEnabledChange,
  parameterEnabled,
}: PlaygroundInputProps) {
  const { t } = useTranslation()
  const [text, setText] = useState('')
  const [attachments, setAttachments] = useState<PlaygroundAttachment[]>([])
  const [isReadingAttachments, setIsReadingAttachments] = useState(false)
  const [isCapturingScreenshot, setIsCapturingScreenshot] = useState(false)
  const imageInputRef = useRef<HTMLInputElement | null>(null)
  const cameraInputRef = useRef<HTMLInputElement | null>(null)
  const fileInputRef = useRef<HTMLInputElement | null>(null)

  const addFiles = useCallback(
    async (files: File[] | FileList): Promise<number> => {
      if (disabled || isReadingAttachments) return 0

      setIsReadingAttachments(true)
      try {
        const result = await readPlaygroundAttachments(files, attachments)

        if (result.attachments.length > 0) {
          setAttachments((currentAttachments) => [
            ...currentAttachments,
            ...result.attachments,
          ])
        }

        for (const rejectedMessage of result.rejected) {
          toast.warning(t('Attachment not added'), {
            description: t(rejectedMessage.key, rejectedMessage.values),
          })
        }

        return result.attachments.length
      } catch (error) {
        toast.error(t('Failed to read attachment'), {
          description:
            error instanceof Error ? error.message : t('Unknown error'),
        })
        return 0
      } finally {
        setIsReadingAttachments(false)
      }
    },
    [attachments, disabled, isReadingAttachments, t]
  )

  const handleFileInputChange = async (
    event: ChangeEvent<HTMLInputElement>
  ) => {
    const input = event.currentTarget
    const files = input.files

    if (files?.length) {
      await addFiles(files)
    }

    input.value = ''
  }

  const handleCapturePhoto = () => {
    if (disabled || isReadingAttachments) return

    cameraInputRef.current?.click()
  }

  const handleTakeScreenshot = async () => {
    if (disabled || isReadingAttachments || isCapturingScreenshot) return

    setIsCapturingScreenshot(true)
    try {
      const file = await captureDisplayScreenshotFile()
      const acceptedCount = await addFiles([file])
      if (acceptedCount > 0) {
        toast.success(t('Screenshot captured'))
      }
    } catch (error) {
      const isScreenCaptureCancelled =
        typeof DOMException !== 'undefined' &&
        error instanceof DOMException &&
        (error.name === 'AbortError' || error.name === 'NotAllowedError')

      if (isScreenCaptureCancelled) {
        toast.info(t('Screen capture was cancelled'))
        return
      }

      toast.error(t('Failed to capture screenshot'), {
        description:
          error instanceof Error ? t(error.message) : t('Unknown error'),
      })
    } finally {
      setIsCapturingScreenshot(false)
    }
  }

  const handlePaste = (event: ClipboardEvent<HTMLTextAreaElement>) => {
    const items = event.clipboardData?.items
    if (!items) return

    const files: File[] = []
    for (const item of items) {
      if (item.kind !== 'file') continue

      const file = item.getAsFile()
      if (file) {
        files.push(file)
      }
    }

    if (files.length === 0) return

    event.preventDefault()
    void addFiles(files)
  }

  const removeAttachment = (attachmentId: string) => {
    setAttachments((currentAttachments) =>
      currentAttachments.filter((attachment) => attachment.id !== attachmentId)
    )
  }

  const handleSubmit = (message: PromptInputMessage) => {
    const submittableText = getSubmittableInputText(
      {
        files: attachments,
        text: message.text,
      },
      disabled || isReadingAttachments
    )

    if (submittableText === null) return

    onSubmit(submittableText, attachments)
    setText('')
    setAttachments([])
  }

  return (
    <div className='grid shrink-0 gap-4 px-1 md:pb-4'>
      <input
        accept='image/*'
        aria-label={t('Upload photo')}
        className='hidden'
        multiple
        onChange={handleFileInputChange}
        ref={imageInputRef}
        type='file'
      />
      <input
        accept='image/*'
        aria-label={t('Take photo')}
        capture='environment'
        className='hidden'
        onChange={handleFileInputChange}
        ref={cameraInputRef}
        type='file'
      />
      <input
        aria-label={t('Upload file')}
        accept='.txt,.md,.markdown,.json,.csv,.tsv,.log,.yaml,.yml,.xml,.pdf,.doc,.docx,.xls,.xlsx,.ods'
        className='hidden'
        multiple
        onChange={handleFileInputChange}
        ref={fileInputRef}
        type='file'
      />
      <PromptInput
        className='relative'
        groupClassName='bg-background/95 dark:bg-background/80 border-border/70 shadow-[0_18px_60px_-32px_rgba(0,0,0,0.65)] ring-1 ring-foreground/5 rounded-xl overflow-hidden transition-all duration-200 focus-within:border-primary/45 focus-within:ring-primary/15 focus-within:shadow-[0_22px_70px_-34px_rgba(0,0,0,0.75)]'
        maxFiles={0}
        onSubmit={handleSubmit}
      >
        {attachments.length > 0 && (
          <PromptInputHeader className='border-border/60 border-b px-3 py-2'>
            <div className='flex w-full flex-wrap items-center gap-2'>
              {attachments.map((attachment) => {
                const isImage = attachment.kind === 'image'
                const Icon = isImage ? ImageIcon : FileTextIcon

                return (
                  <div
                    className='bg-muted/40 border-border/70 text-foreground flex h-9 max-w-full items-center gap-2 rounded-md border px-2 text-xs'
                    key={attachment.id}
                  >
                    <div className='bg-background flex size-6 shrink-0 items-center justify-center overflow-hidden rounded border'>
                      {isImage && attachment.dataUrl ? (
                        <img
                          alt={attachment.name}
                          className='size-full object-cover'
                          src={attachment.dataUrl}
                        />
                      ) : (
                        <Icon className='text-muted-foreground size-3.5' />
                      )}
                    </div>
                    <span className='max-w-44 truncate'>{attachment.name}</span>
                    <span className='text-muted-foreground shrink-0 tabular-nums'>
                      {formatAttachmentSize(attachment.size)}
                    </span>
                    <Button
                      aria-label={t('Remove attachment')}
                      className='text-muted-foreground hover:text-foreground -mr-1 size-6 rounded p-0'
                      onClick={() => removeAttachment(attachment.id)}
                      size='icon-xs'
                      type='button'
                      variant='ghost'
                    >
                      <XIcon className='size-3' />
                    </Button>
                  </div>
                )
              })}
              {isReadingAttachments && (
                <div className='text-muted-foreground flex h-9 items-center gap-2 rounded-md px-2 text-xs'>
                  <Loader2Icon className='size-3.5 animate-spin' />
                  {t('Reading attachment...')}
                </div>
              )}
            </div>
          </PromptInputHeader>
        )}
        <PromptInputTextarea
          autoComplete='off'
          autoCorrect='off'
          autoCapitalize='off'
          spellCheck={false}
          className={cn(
            'min-h-20 px-5 pt-4 pb-3 leading-7 md:min-h-24 md:text-base',
            attachments.length > 0 && 'pt-3'
          )}
          disabled={disabled}
          onChange={(event) => setText(event.target.value)}
          onPaste={handlePaste}
          placeholder={t('Ask anything')}
          value={text}
        />

        <PromptInputFooter className='border-border/60 bg-muted/20 dark:bg-muted/10 border-t px-3 py-2.5 backdrop-blur'>
          <PlaygroundInputControls
            disabled={disabled}
            groups={groups}
            groupValue={groupValue}
            hasAttachments={attachments.length > 0}
            isGenerating={isGenerating}
            isModelLoading={isModelLoading}
            models={models}
            modelValue={modelValue}
            onGroupChange={onGroupChange}
            onModelChange={onModelChange}
            onStop={onStop}
            text={text}
            tools={
              <PlaygroundInputTools
                config={config}
                disabled={
                  disabled || isReadingAttachments || isCapturingScreenshot
                }
                hasMessages={hasMessages}
                onConfigChange={onConfigChange}
                onClearMessages={onClearMessages}
                onCapturePhoto={handleCapturePhoto}
                onSelectFiles={() => fileInputRef.current?.click()}
                onSelectImages={() => imageInputRef.current?.click()}
                onTakeScreenshot={() => void handleTakeScreenshot()}
                onWebSearchChange={onWebSearchChange}
                webSearchMode={webSearchMode}
                webSearchAvailable={webSearchAvailable}
                onParameterEnabledChange={onParameterEnabledChange}
                parameterEnabled={parameterEnabled}
              />
            }
          />
        </PromptInputFooter>
      </PromptInput>
    </div>
  )
}
