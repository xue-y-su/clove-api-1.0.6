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
import { ImageIcon, MessageSquareIcon } from 'lucide-react'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { useStatus } from '@/hooks/use-status'

import { PlaygroundChat } from './components/chat/playground-chat'
import { PlaygroundImagePanel } from './components/image/playground-image-panel'
import { PlaygroundInput } from './components/input/playground-input'
import {
  useChatHandler,
  usePlaygroundConversation,
  usePlaygroundOptions,
  usePlaygroundState,
} from './hooks'
import { getPlaygroundWebSearchCapability } from './lib/web-search-capability'

type PlaygroundMode = 'chat' | 'image'

export function Playground() {
  const { t } = useTranslation()
  const { status } = useStatus()
  const [mode, setMode] = useState<PlaygroundMode>('chat')
  const {
    config,
    parameterEnabled,
    messages,
    isLoadingMessages,
    models,
    groups,
    updateMessages,
    setModels,
    setGroups,
    updateConfig,
    updateParameterEnabled,
    clearMessages,
  } = usePlaygroundState()

  const { sendChat, stopGeneration, isGenerating } = useChatHandler({
    config,
    parameterEnabled,
    onMessageUpdate: updateMessages,
  })

  const {
    editingMessageKey,
    handleSendMessage,
    handleRegenerateMessage,
    handleEditMessage,
    handleEditOpenChange,
    applyEdit,
    handleDeleteMessage,
  } = usePlaygroundConversation({
    messages,
    updateMessages,
    sendChat,
  })

  const handleClearMessages = () => {
    handleEditOpenChange(false)
    clearMessages()
  }

  const { isLoadingModels } = usePlaygroundOptions({
    currentGroup: config.group,
    currentModel: config.model,
    setGroups,
    setModels,
    updateConfig,
  })
  const webSearchCapability = getPlaygroundWebSearchCapability(status)

  useEffect(() => {
    if (!webSearchCapability.available && config.web_search !== 'off') {
      updateConfig('web_search', 'off')
    }
  }, [config.web_search, updateConfig, webSearchCapability.available])

  return (
    <Tabs
      className='relative flex size-full min-h-0 flex-col gap-0 overflow-hidden'
      onValueChange={(value) => setMode(value as PlaygroundMode)}
      value={mode}
    >
      <div className='border-border/60 bg-background/90 flex shrink-0 justify-center border-b px-4 py-2 backdrop-blur'>
        <TabsList>
          <TabsTrigger value='chat'>
            <MessageSquareIcon className='size-4' />
            {t('Chat')}
          </TabsTrigger>
          <TabsTrigger value='image'>
            <ImageIcon className='size-4' />
            {t('Image')}
          </TabsTrigger>
        </TabsList>
      </div>

      <TabsContent className='flex min-h-0 flex-1 flex-col' value='chat'>
        <div className='flex min-h-0 flex-1 flex-col overflow-hidden'>
          <PlaygroundChat
            messages={messages}
            isLoadingMessages={isLoadingMessages}
            onRegenerateMessage={handleRegenerateMessage}
            onEditMessage={handleEditMessage}
            onDeleteMessage={handleDeleteMessage}
            onSelectPrompt={handleSendMessage}
            isGenerating={isGenerating}
            editingKey={editingMessageKey}
            onCancelEdit={handleEditOpenChange}
            onSaveEdit={(newContent) => applyEdit(newContent, false)}
            onSaveEditAndSubmit={(newContent) => applyEdit(newContent, true)}
          />
        </div>

        <div className='mx-auto w-full max-w-4xl'>
          <PlaygroundInput
            config={config}
            disabled={isGenerating}
            groups={groups}
            groupValue={config.group}
            isGenerating={isGenerating}
            isModelLoading={isLoadingModels}
            modelValue={config.model}
            models={models}
            onGroupChange={(value) => updateConfig('group', value)}
            onConfigChange={updateConfig}
            onClearMessages={handleClearMessages}
            onModelChange={(value) => updateConfig('model', value)}
            onParameterEnabledChange={updateParameterEnabled}
            onStop={stopGeneration}
            onSubmit={handleSendMessage}
            parameterEnabled={parameterEnabled}
            hasMessages={messages.length > 0}
            onWebSearchChange={(value) => updateConfig('web_search', value)}
            webSearchMode={config.web_search}
            webSearchAvailable={webSearchCapability.available}
          />
        </div>
      </TabsContent>

      <TabsContent className='flex min-h-0 flex-1 flex-col' value='image'>
        <PlaygroundImagePanel groups={groups} initialGroup={config.group} />
      </TabsContent>
    </Tabs>
  )
}
