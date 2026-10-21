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
import { BrainIcon, GlobeIcon, PaperclipIcon, Trash2Icon } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import {
  PromptInputButton,
  PromptInputTools,
} from '@/components/ai-elements/prompt-input'
import { ConfirmDialog } from '@/components/confirm-dialog'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { cn } from '@/lib/utils'

import { ATTACHMENT_ACTIONS } from '../../lib'
import type {
  ParameterEnabled,
  PlaygroundConfig,
  PlaygroundReasoningEffort,
  PlaygroundWebSearchMode,
} from '../../types'
import { PlaygroundParameterPanel } from './playground-parameter-panel'

const REASONING_EFFORT_OPTIONS: Array<{
  value: PlaygroundReasoningEffort
  label: string
}> = [
  { value: 'off', label: 'Off' },
  { value: 'low', label: 'Low' },
  { value: 'medium', label: 'Medium' },
  { value: 'high', label: 'High' },
]

const WEB_SEARCH_OPTIONS: Array<{
  value: PlaygroundWebSearchMode
  label: string
}> = [
  { value: 'off', label: 'Off' },
  { value: 'basic', label: 'Basic' },
  { value: 'advanced', label: 'Advanced' },
]

type PlaygroundInputToolsProps = {
  config: PlaygroundConfig
  disabled?: boolean
  hasMessages?: boolean
  onClearMessages?: () => void
  onCapturePhoto?: () => void
  onSelectFiles?: () => void
  onSelectImages?: () => void
  onTakeScreenshot?: () => void
  onWebSearchChange?: (mode: PlaygroundWebSearchMode) => void
  webSearchMode?: PlaygroundWebSearchMode
  webSearchAvailable?: boolean
  onConfigChange: <K extends keyof PlaygroundConfig>(
    key: K,
    value: PlaygroundConfig[K]
  ) => void
  onParameterEnabledChange: (
    key: keyof ParameterEnabled,
    value: boolean
  ) => void
  parameterEnabled: ParameterEnabled
}

export function PlaygroundInputTools({
  config,
  disabled,
  hasMessages = false,
  onClearMessages,
  onCapturePhoto,
  onSelectFiles,
  onSelectImages,
  onTakeScreenshot,
  onWebSearchChange,
  webSearchMode = 'off',
  webSearchAvailable = false,
  onConfigChange,
  onParameterEnabledChange,
  parameterEnabled,
}: PlaygroundInputToolsProps) {
  const { t } = useTranslation()
  const [clearConfirmOpen, setClearConfirmOpen] = useState(false)
  const reasoningEffort = config.reasoning_effort
  const activeWebSearchLabel =
    WEB_SEARCH_OPTIONS.find((option) => option.value === webSearchMode)
      ?.label ?? 'Off'
  const activeReasoningEffortLabel =
    REASONING_EFFORT_OPTIONS.find((option) => option.value === reasoningEffort)
      ?.label ?? 'Off'

  const handleFileAction = (action: string) => {
    if (action === 'upload-photo') {
      onSelectImages?.()
      return
    }

    if (action === 'upload-file') {
      onSelectFiles?.()
      return
    }

    if (action === 'take-photo') {
      onCapturePhoto?.()
      return
    }

    if (action === 'take-screenshot') {
      onTakeScreenshot?.()
    }
  }

  const handleClearMessages = () => {
    onClearMessages?.()
    setClearConfirmOpen(false)
    toast.success(t('Conversation cleared'))
  }

  return (
    <>
      <PromptInputTools className='bg-background/70 border-border/60 rounded-lg border p-1 shadow-xs'>
        <Tooltip>
          <DropdownMenu>
            <TooltipTrigger
              render={
                <DropdownMenuTrigger
                  render={
                    <PromptInputButton
                      aria-label={t('Attach')}
                      className='text-muted-foreground hover:text-foreground hover:bg-muted/70 font-medium'
                      disabled={disabled}
                      variant='ghost'
                    />
                  }
                >
                  <PaperclipIcon size={16} />
                </DropdownMenuTrigger>
              }
            />
            <TooltipContent>
              <p>{t('Attach')}</p>
            </TooltipContent>
            <DropdownMenuContent align='start'>
              {ATTACHMENT_ACTIONS.map(({ action, icon: Icon, label }) => (
                <DropdownMenuItem
                  key={action}
                  onClick={() => handleFileAction(action)}
                >
                  <Icon className='mr-2' size={16} />
                  {t(label)}
                </DropdownMenuItem>
              ))}
            </DropdownMenuContent>
          </DropdownMenu>
        </Tooltip>

        <Tooltip>
          <DropdownMenu>
            <TooltipTrigger
              render={
                <DropdownMenuTrigger
                  render={
                    <PromptInputButton
                      aria-label={t('Web search')}
                      aria-pressed={webSearchMode !== 'off'}
                      className={cn(
                        'font-medium',
                        webSearchMode !== 'off'
                          ? 'bg-primary/10 text-primary hover:bg-primary/15 hover:text-primary'
                          : 'text-muted-foreground hover:text-foreground hover:bg-muted/70'
                      )}
                      disabled={disabled || !webSearchAvailable}
                      variant='ghost'
                    />
                  }
                >
                  <GlobeIcon size={16} />
                  {webSearchMode !== 'off' ? (
                    <span className='hidden text-xs sm:inline'>
                      {t(activeWebSearchLabel)}
                    </span>
                  ) : null}
                </DropdownMenuTrigger>
              }
            />
            <TooltipContent>
              <p>
                {webSearchAvailable
                  ? t('Search the public web at inference time')
                  : t('Web search is not configured by the administrator.')}
              </p>
            </TooltipContent>
            <DropdownMenuContent align='start' className='min-w-36'>
              {WEB_SEARCH_OPTIONS.map((option) => (
                <DropdownMenuItem
                  key={option.value}
                  onClick={() => onWebSearchChange?.(option.value)}
                >
                  <span className='min-w-0 flex-1'>{t(option.label)}</span>
                  {webSearchMode === option.value ? (
                    <span className='text-primary text-xs'>
                      {t('Selected')}
                    </span>
                  ) : null}
                </DropdownMenuItem>
              ))}
            </DropdownMenuContent>
          </DropdownMenu>
        </Tooltip>

        <Tooltip>
          <DropdownMenu>
            <TooltipTrigger
              render={
                <DropdownMenuTrigger
                  render={
                    <PromptInputButton
                      aria-label={t('Reasoning Effort')}
                      aria-pressed={reasoningEffort !== 'off'}
                      className={cn(
                        'font-medium',
                        reasoningEffort !== 'off'
                          ? 'bg-primary/10 text-primary hover:bg-primary/15 hover:text-primary'
                          : 'text-muted-foreground hover:text-foreground hover:bg-muted/70'
                      )}
                      disabled={disabled}
                      variant='ghost'
                    />
                  }
                >
                  <BrainIcon size={16} />
                  {reasoningEffort !== 'off' ? (
                    <span className='hidden text-xs sm:inline'>
                      {t(activeReasoningEffortLabel)}
                    </span>
                  ) : null}
                </DropdownMenuTrigger>
              }
            />
            <TooltipContent>
              <p>{t('Reasoning Effort')}</p>
              <p className='text-muted-foreground mt-1 max-w-52 text-xs'>
                {t('Higher reasoning effort may use more tokens')}
              </p>
            </TooltipContent>
            <DropdownMenuContent align='start' className='min-w-36'>
              {REASONING_EFFORT_OPTIONS.map((option) => (
                <DropdownMenuItem
                  key={option.value}
                  onClick={() =>
                    onConfigChange('reasoning_effort', option.value)
                  }
                >
                  <span className='min-w-0 flex-1'>{t(option.label)}</span>
                  {reasoningEffort === option.value ? (
                    <span className='text-primary text-xs'>
                      {t('Selected')}
                    </span>
                  ) : null}
                </DropdownMenuItem>
              ))}
            </DropdownMenuContent>
          </DropdownMenu>
        </Tooltip>

        <PlaygroundParameterPanel
          config={config}
          disabled={disabled}
          onConfigChange={onConfigChange}
          onParameterEnabledChange={onParameterEnabledChange}
          parameterEnabled={parameterEnabled}
        />

        <Tooltip>
          <TooltipTrigger
            render={
              <PromptInputButton
                aria-label={t('Clear chat history')}
                className='text-muted-foreground hover:text-destructive hover:bg-destructive/10 font-medium'
                disabled={disabled || !hasMessages || !onClearMessages}
                onClick={() => setClearConfirmOpen(true)}
                variant='ghost'
              >
                <Trash2Icon size={16} />
              </PromptInputButton>
            }
          />
          <TooltipContent>
            <p>{t('Clear chat history')}</p>
          </TooltipContent>
        </Tooltip>
      </PromptInputTools>

      <ConfirmDialog
        destructive
        desc={t(
          'All playground messages saved in this browser will be removed. This cannot be undone.'
        )}
        confirmText={t('Clear')}
        handleConfirm={handleClearMessages}
        open={clearConfirmOpen}
        onOpenChange={setClearConfirmOpen}
        title={t('Clear chat history?')}
      />
    </>
  )
}
