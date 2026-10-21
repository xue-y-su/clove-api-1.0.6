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
import { getRouteApi, useNavigate } from '@tanstack/react-router'
import { useCallback, useMemo } from 'react'
import { useTranslation } from 'react-i18next'

import { SectionPageLayout } from '@/components/layout'
import type { NavGroup } from '@/components/layout/types'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { ComplianceConfirmationsTable } from '@/features/compliance/compliance-confirmations-table'
import { ForbiddenError } from '@/features/errors/forbidden'
import { SourceAttributionConfirmationsTable } from '@/features/source-attribution/source-attribution-confirmations-table'
import { CacheStatsDialog } from '@/features/system-settings/general/channel-affinity/cache-stats-dialog'
import { useIsAdmin } from '@/hooks/use-admin'
import { useSidebarConfig } from '@/hooks/use-sidebar-config'

import { UserInfoDialog } from './components/dialogs/user-info-dialog'
import {
  type LogsViewScope,
  UsageLogsProvider,
  useLogsViewScope,
  useUsageLogsContext,
} from './components/usage-logs-provider'
import { UsageLogsTable } from './components/usage-logs-table'
import {
  isUsageLogsSectionId,
  USAGE_LOGS_DEFAULT_SECTION,
  type UsageLogsSectionId,
} from './section-registry'

const route = getRouteApi('/_authenticated/usage-logs/$section')
const TASK_LOG_SECTIONS = ['drawing', 'task'] as const
type TaskUsageLogSection = (typeof TASK_LOG_SECTIONS)[number]

const SECTION_META: Record<UsageLogsSectionId, { titleKey: string }> = {
  common: {
    titleKey: 'Common Logs',
  },
  drawing: {
    titleKey: 'Drawing Logs',
  },
  task: {
    titleKey: 'Task Logs',
  },
  compliance: {
    titleKey: 'Compliance Confirmations',
  },
  'source-attribution': {
    titleKey: 'Source Attribution Confirmations',
  },
  'ip-audit': {
    titleKey: 'IP Audit',
  },
}

function isTaskUsageLogSection(
  section: UsageLogsSectionId
): section is TaskUsageLogSection {
  return (TASK_LOG_SECTIONS as readonly string[]).includes(section)
}

function UsageLogsContent() {
  const { t } = useTranslation()
  const isAdmin = useIsAdmin()
  const navigate = useNavigate()
  const params = route.useParams()
  const activeCategory: UsageLogsSectionId =
    params.section && isUsageLogsSectionId(params.section)
      ? params.section
      : USAGE_LOGS_DEFAULT_SECTION
  const {
    selectedUserId,
    userInfoDialogOpen,
    setUserInfoDialogOpen,
    affinityTarget,
    affinityDialogOpen,
    setAffinityDialogOpen,
  } = useUsageLogsContext()
  const { canManageScope, viewScope, setViewScope } = useLogsViewScope()
  const tabNavGroups = useMemo<NavGroup[]>(
    () => [
      {
        title: 'Task Logs',
        items: [
          ...TASK_LOG_SECTIONS.map((section) => ({
            title: SECTION_META[section].titleKey,
            url: `/usage-logs/${section}`,
          })),
          ...(isAdmin
            ? [
                {
                  title: SECTION_META.compliance.titleKey,
                  url: '/usage-logs/compliance',
                },
                {
                  title: SECTION_META['source-attribution'].titleKey,
                  url: '/usage-logs/source-attribution',
                },
              ]
            : []),
        ],
      },
    ],
    [isAdmin]
  )
  const filteredTabGroups = useSidebarConfig(tabNavGroups)
  const visibleSections = useMemo(
    () =>
      (filteredTabGroups[0]?.items ?? [])
        .map((item) => {
          if (!('url' in item) || typeof item.url !== 'string') return null
          return item.url.split('/').pop() ?? null
        })
        .filter((section): section is UsageLogsSectionId =>
          Boolean(section && isUsageLogsSectionId(section))
        ),
    [filteredTabGroups]
  )

  const handleSectionChange = useCallback(
    (section: string) => {
      void navigate({
        to: '/usage-logs/$section',
        params: { section: section as UsageLogsSectionId },
      })
    },
    [navigate]
  )
  const handleViewScopeChange = useCallback(
    (scope: string) => {
      if (scope === 'all' || scope === 'self') {
        setViewScope(scope as LogsViewScope)
      }
    },
    [setViewScope]
  )

  if (activeCategory === 'compliance') {
    if (!isAdmin) return <ForbiddenError />
  }
  if (activeCategory === 'source-attribution') {
    if (!isAdmin) return <ForbiddenError />
  }
  if (activeCategory === 'ip-audit') {
    if (!isAdmin) return <ForbiddenError />
  }
  const pageMeta =
    activeCategory === 'common'
      ? SECTION_META.common
      : SECTION_META[activeCategory]
  const showTaskSwitcher =
    activeCategory !== 'common' &&
    activeCategory !== 'ip-audit' &&
    visibleSections.length > 1
  let content = null
  if (activeCategory === 'compliance') {
    content = <ComplianceConfirmationsTable />
  } else if (activeCategory === 'source-attribution') {
    content = <SourceAttributionConfirmationsTable />
  } else if (
    isTaskUsageLogSection(activeCategory) ||
    activeCategory === 'common'
  ) {
    content = <UsageLogsTable logCategory={activeCategory} />
  }

  return (
    <>
      <SectionPageLayout fixedContent>
        <SectionPageLayout.Title>
          {t(pageMeta.titleKey)}
        </SectionPageLayout.Title>
        {canManageScope && activeCategory !== 'ip-audit' && (
          <SectionPageLayout.Actions>
            <Tabs value={viewScope} onValueChange={handleViewScopeChange}>
              <TabsList>
                <TabsTrigger value='all'>{t('All')}</TabsTrigger>
                <TabsTrigger value='self'>{t('Only Mine')}</TabsTrigger>
              </TabsList>
            </Tabs>
          </SectionPageLayout.Actions>
        )}
        <SectionPageLayout.Content>
          <div className='flex h-full min-h-0 flex-col gap-4'>
            {showTaskSwitcher && (
              <Tabs value={activeCategory} onValueChange={handleSectionChange}>
                <TabsList className='max-w-full flex-wrap justify-start group-data-horizontal/tabs:h-auto'>
                  {visibleSections.map((section) => (
                    <TabsTrigger key={section} value={section}>
                      {t(SECTION_META[section].titleKey)}
                    </TabsTrigger>
                  ))}
                </TabsList>
              </Tabs>
            )}
            <div className='min-h-0 flex-1'>{content}</div>
          </div>
        </SectionPageLayout.Content>
      </SectionPageLayout>

      <UserInfoDialog
        userId={selectedUserId}
        open={userInfoDialogOpen}
        onOpenChange={setUserInfoDialogOpen}
      />

      <CacheStatsDialog
        open={affinityDialogOpen}
        onOpenChange={setAffinityDialogOpen}
        target={
          affinityTarget
            ? {
                rule_name: affinityTarget.rule_name || '',
                using_group:
                  affinityTarget.using_group ||
                  affinityTarget.selected_group ||
                  '',
                key_hint: affinityTarget.key_hint || '',
                key_fp: affinityTarget.key_fp || '',
              }
            : null
        }
      />
    </>
  )
}

export function UsageLogs() {
  return (
    <UsageLogsProvider>
      <UsageLogsContent />
    </UsageLogsProvider>
  )
}
