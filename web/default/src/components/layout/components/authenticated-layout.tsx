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
import { useQueryClient } from '@tanstack/react-query'
import { useCallback, useEffect, useRef, useState } from 'react'

import { CommandMenu } from '@/components/command-menu'
import { AnimatedOutlet } from '@/components/page-transition'
import { SkipToMain } from '@/components/skip-to-main'
import { SidebarInset, SidebarProvider } from '@/components/ui/sidebar'
import { Spinner } from '@/components/ui/spinner'
import { LayoutProvider } from '@/context/layout-provider'
import { SearchProvider } from '@/context/search-provider'
import { GlobalAnnouncementDialog } from '@/features/announcements/global-announcement-dialog'
import {
  getAuthenticatedGateAccess,
  gateQueryKey,
  type AuthenticatedGateResolution,
} from '@/features/auth/lib/authenticated-gates'
import { ComplianceGate } from '@/features/compliance/compliance-gate'
import { TokenRevealProvider } from '@/features/keys/components/token-reveal-provider'
import { SourceAttributionGate } from '@/features/source-attribution/source-attribution-gate'
import { getCookie } from '@/lib/cookies'
import { cn } from '@/lib/utils'
import { useAuthStore } from '@/stores/auth-store'

import { AppHeader } from './app-header'
import { AppSidebar } from './app-sidebar'
import { GateLoadError } from './gate-load-error'

type AuthenticatedLayoutProps = {
  children?: React.ReactNode
}

export function AuthenticatedLayout(props: AuthenticatedLayoutProps) {
  const defaultOpen = getCookie('sidebar_state') !== 'false'
  const queryClient = useQueryClient()
  const userId = useAuthStore((state) => state.auth.user?.id)
  const [complianceResolution, setComplianceResolution] =
    useState<AuthenticatedGateResolution>('loading')
  const [sourceAttributionResolution, setSourceAttributionResolution] =
    useState<AuthenticatedGateResolution>('loading')
  const [complianceError, setComplianceError] = useState<unknown>(null)
  const [sourceAttributionError, setSourceAttributionError] =
    useState<unknown>(null)
  const resolvedUserId = useRef(userId)
  const gateAccess = getAuthenticatedGateAccess({
    signedIn: Boolean(userId),
    compliance: complianceResolution,
    sourceAttribution: sourceAttributionResolution,
  })

  useEffect(() => {
    if (resolvedUserId.current === userId) return
    resolvedUserId.current = userId
    setComplianceResolution('loading')
    setSourceAttributionResolution('loading')
    setComplianceError(null)
    setSourceAttributionError(null)
  }, [userId])

  const handleComplianceResolution = useCallback(
    (resolution: AuthenticatedGateResolution, error?: unknown) => {
      setComplianceResolution(resolution)
      setComplianceError(error ?? null)
    },
    []
  )
  const handleSourceAttributionResolution = useCallback(
    (resolution: AuthenticatedGateResolution, error?: unknown) => {
      setSourceAttributionResolution(resolution)
      setSourceAttributionError(error ?? null)
    },
    []
  )

  const retryFailedGate = () => {
    if (!gateAccess.failed) return
    if (gateAccess.failed === 'compliance') {
      setComplianceResolution('loading')
      setComplianceError(null)
    } else {
      setSourceAttributionResolution('loading')
      setSourceAttributionError(null)
    }
    void queryClient.invalidateQueries({
      queryKey: gateQueryKey(gateAccess.failed),
    })
  }

  return (
    <TokenRevealProvider>
      <LayoutProvider>
        <SearchProvider>
          {gateAccess.content ? <CommandMenu /> : null}
          <SidebarProvider defaultOpen={defaultOpen} className='flex-col'>
            <ComplianceGate
              queryEnabled={Boolean(userId)}
              displayEnabled={gateAccess.compliance}
              onResolutionChange={handleComplianceResolution}
            />
            <SourceAttributionGate
              queryEnabled={Boolean(userId)}
              displayEnabled={gateAccess.sourceAttribution}
              onResolutionChange={handleSourceAttributionResolution}
            />
            <GlobalAnnouncementDialog enabled={gateAccess.announcement} />
            {gateAccess.failed ? (
              <GateLoadError
                gate={gateAccess.failed}
                error={
                  gateAccess.failed === 'compliance'
                    ? complianceError
                    : sourceAttributionError
                }
                onRetry={retryFailedGate}
              />
            ) : gateAccess.content ? (
              <>
                <SkipToMain />
                <AppHeader />
                <div className='flex min-h-0 w-full flex-1'>
                  <AppSidebar />
                  <SidebarInset
                    className={cn(
                      '@container/content',
                      'h-[calc(100svh-var(--app-header-height,0px))]',
                      'min-h-0 overflow-hidden',
                      'peer-data-[variant=inset]:h-[calc(100svh-var(--app-header-height,0px)-(var(--spacing)*4))]'
                    )}
                  >
                    {props.children ?? <AnimatedOutlet />}
                  </SidebarInset>
                </div>
              </>
            ) : (
              <div className='flex min-h-svh w-full items-center justify-center'>
                <Spinner className='size-6' />
              </div>
            )}
          </SidebarProvider>
        </SearchProvider>
      </LayoutProvider>
    </TokenRevealProvider>
  )
}
