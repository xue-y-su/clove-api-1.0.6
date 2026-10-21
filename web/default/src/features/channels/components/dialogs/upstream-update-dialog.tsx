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
import { Search } from 'lucide-react'
import { useEffect, useState, useMemo } from 'react'
import { useTranslation } from 'react-i18next'

import { Dialog } from '@/components/dialog'
import { StatusBadge } from '@/components/status-badge'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'

import { createUpstreamDiffSelection } from '../../lib/upstream-update-utils'

interface UpstreamUpdateDialogProps {
  open: boolean
  addModels: string[]
  retainedModels: string[]
  removeModels: string[]
  preferredTab: 'add' | 'remove'
  isExplicitPublication?: boolean
  confirmLoading: boolean
  onConfirm: (data: { addModels: string[]; removeModels: string[] }) => void
  onCancel: () => void
}

export function UpstreamUpdateDialog(props: UpstreamUpdateDialogProps) {
  const { t } = useTranslation()
  const [activeTab, setActiveTab] = useState<'add' | 'retained' | 'remove'>(
    props.preferredTab
  )
  const [searchAdd, setSearchAdd] = useState('')
  const [searchRetained, setSearchRetained] = useState('')
  const [searchRemove, setSearchRemove] = useState('')
  const [selectedAdd, setSelectedAdd] = useState<Set<string>>(new Set())
  const [selectedRemove, setSelectedRemove] = useState<Set<string>>(new Set())

  useEffect(() => {
    if (!props.open) return
    const selection = createUpstreamDiffSelection(
      props.addModels,
      props.removeModels
    )
    setSelectedAdd(new Set(selection.selectedAdd))
    setSelectedRemove(new Set(selection.selectedRemove))
    setSearchAdd('')
    setSearchRetained('')
    setSearchRemove('')
    let nextTab: 'add' | 'retained' | 'remove' = 'retained'
    if (props.addModels.length > 0) {
      nextTab = 'add'
    } else if (props.removeModels.length > 0) {
      nextTab = 'remove'
    }
    setActiveTab(nextTab)
  }, [props.open, props.addModels, props.removeModels])

  const filteredAdd = useMemo(
    () =>
      props.addModels.filter((m) =>
        m.toLowerCase().includes(searchAdd.toLowerCase())
      ),
    [props.addModels, searchAdd]
  )

  const filteredRemove = useMemo(
    () =>
      props.removeModels.filter((m) =>
        m.toLowerCase().includes(searchRemove.toLowerCase())
      ),
    [props.removeModels, searchRemove]
  )

  const filteredRetained = useMemo(
    () =>
      props.retainedModels.filter((model) =>
        model.toLowerCase().includes(searchRetained.toLowerCase())
      ),
    [props.retainedModels, searchRetained]
  )

  const toggleModel = (
    model: string,
    set: Set<string>,
    setter: (s: Set<string>) => void
  ) => {
    const next = new Set(set)
    if (next.has(model)) next.delete(model)
    else next.add(model)
    setter(next)
  }

  const toggleAllVisible = (
    models: string[],
    set: Set<string>,
    setter: (s: Set<string>) => void
  ) => {
    const allSelected = models.every((m) => set.has(m))
    const next = new Set(set)
    if (allSelected) {
      models.forEach((m) => next.delete(m))
    } else {
      models.forEach((m) => next.add(m))
    }
    setter(next)
  }

  const handleConfirm = () => {
    props.onConfirm({
      addModels: [...selectedAdd],
      removeModels: [...selectedRemove],
    })
  }

  return (
    <Dialog
      open={props.open}
      onOpenChange={(v) => !v && props.onCancel()}
      title={t('Upstream Model Updates')}
      contentClassName='sm:max-w-2xl'
      contentHeight='auto'
      bodyClassName='max-h-[70vh] space-y-4 overflow-y-auto'
      footer={
        <>
          <Button variant='outline' onClick={props.onCancel}>
            {t('Cancel')}
          </Button>
          <Button
            onClick={handleConfirm}
            disabled={
              props.confirmLoading ||
              (props.addModels.length === 0 && props.removeModels.length === 0)
            }
          >
            {t('Confirm')}
          </Button>
        </>
      }
    >
      <p className='text-muted-foreground text-sm'>
        {t(
          'Review added, retained, and missing upstream models before confirming.'
        )}
      </p>
      {props.isExplicitPublication && (
        <p className='rounded border border-amber-500/40 bg-amber-500/10 p-3 text-sm'>
          {t(
            'Approved additions remain hidden from the model catalog until they have explicit pricing and at least one real enabled group.'
          )}
        </p>
      )}
      <Tabs
        value={activeTab}
        onValueChange={(v) => setActiveTab(v as 'add' | 'retained' | 'remove')}
      >
        <TabsList className='grid w-full grid-cols-3'>
          <TabsTrigger value='add' className='gap-1'>
            {t('Added')}
            <StatusBadge variant='neutral' className='ml-1' copyable={false}>
              {selectedAdd.size}/{props.addModels.length}
            </StatusBadge>
          </TabsTrigger>
          <TabsTrigger value='retained' className='gap-1'>
            {t('Retained')}
            <StatusBadge variant='neutral' className='ml-1' copyable={false}>
              {props.retainedModels.length}
            </StatusBadge>
          </TabsTrigger>
          <TabsTrigger value='remove' className='gap-1'>
            {t('Missing')}
            <StatusBadge variant='neutral' className='ml-1' copyable={false}>
              {selectedRemove.size}/{props.removeModels.length}
            </StatusBadge>
          </TabsTrigger>
        </TabsList>

        <TabsContent value='add' className='space-y-3'>
          <div className='relative'>
            <Search className='text-muted-foreground absolute top-2.5 left-2.5 h-4 w-4' />
            <Input
              placeholder={t('Search models...')}
              className='pl-8'
              value={searchAdd}
              onChange={(e) => setSearchAdd(e.target.value)}
            />
          </div>
          {filteredAdd.length > 0 && (
            <div className='flex items-center gap-2'>
              <Checkbox
                checked={filteredAdd.every((m) => selectedAdd.has(m))}
                onCheckedChange={() =>
                  toggleAllVisible(filteredAdd, selectedAdd, setSelectedAdd)
                }
              />
              <span className='text-muted-foreground text-xs'>
                {t('Select All Visible')}
              </span>
            </div>
          )}
          <ScrollArea className='h-[280px] rounded-md border p-2'>
            {filteredAdd.length > 0 ? (
              <div className='space-y-1'>
                {filteredAdd.map((model) => (
                  <label
                    key={model}
                    className='hover:bg-accent flex cursor-pointer items-center gap-2 rounded px-2 py-1.5'
                  >
                    <Checkbox
                      checked={selectedAdd.has(model)}
                      onCheckedChange={() =>
                        toggleModel(model, selectedAdd, setSelectedAdd)
                      }
                    />
                    <span className='truncate text-sm'>{model}</span>
                  </label>
                ))}
              </div>
            ) : (
              <p className='text-muted-foreground py-8 text-center text-sm'>
                {props.addModels.length === 0
                  ? t('No models to add')
                  : t('No matching results')}
              </p>
            )}
          </ScrollArea>
        </TabsContent>

        <TabsContent value='retained' className='space-y-3'>
          <div className='relative'>
            <Search className='text-muted-foreground absolute top-2.5 left-2.5 h-4 w-4' />
            <Input
              placeholder={t('Search models...')}
              className='pl-8'
              value={searchRetained}
              onChange={(e) => setSearchRetained(e.target.value)}
            />
          </div>
          <ScrollArea className='h-[280px] rounded-md border p-2'>
            <div className='space-y-1'>
              {filteredRetained.map((model) => (
                <div key={model} className='rounded px-2 py-1.5 text-sm'>
                  {model}
                </div>
              ))}
            </div>
          </ScrollArea>
        </TabsContent>

        <TabsContent value='remove' className='space-y-3'>
          <div className='relative'>
            <Search className='text-muted-foreground absolute top-2.5 left-2.5 h-4 w-4' />
            <Input
              placeholder={t('Search models...')}
              className='pl-8'
              value={searchRemove}
              onChange={(e) => setSearchRemove(e.target.value)}
            />
          </div>
          {filteredRemove.length > 0 && (
            <div className='flex items-center gap-2'>
              <Checkbox
                checked={filteredRemove.every((m) => selectedRemove.has(m))}
                onCheckedChange={() =>
                  toggleAllVisible(
                    filteredRemove,
                    selectedRemove,
                    setSelectedRemove
                  )
                }
              />
              <span className='text-muted-foreground text-xs'>
                {t('Select All Visible')}
              </span>
            </div>
          )}
          <ScrollArea className='h-[280px] rounded-md border p-2'>
            {filteredRemove.length > 0 ? (
              <div className='space-y-1'>
                {filteredRemove.map((model) => (
                  <label
                    key={model}
                    className='hover:bg-accent flex cursor-pointer items-center gap-2 rounded px-2 py-1.5'
                  >
                    <Checkbox
                      checked={selectedRemove.has(model)}
                      onCheckedChange={() =>
                        toggleModel(model, selectedRemove, setSelectedRemove)
                      }
                    />
                    <span className='truncate text-sm'>{model}</span>
                  </label>
                ))}
              </div>
            ) : (
              <p className='text-muted-foreground py-8 text-center text-sm'>
                {props.removeModels.length === 0
                  ? t('No missing models')
                  : t('No matching results')}
              </p>
            )}
          </ScrollArea>
        </TabsContent>
      </Tabs>
    </Dialog>
  )
}
