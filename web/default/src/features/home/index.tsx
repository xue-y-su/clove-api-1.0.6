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
import { useCallback, useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { PublicLayout } from '@/components/layout'
import { Footer } from '@/components/layout/components/footer'
import { RichContent } from '@/components/rich-content'
import { useTheme } from '@/context/theme-provider'
import { useStatus } from '@/hooks/use-status'
import { isLikelyHtml } from '@/lib/content-format'
import { useAuthStore } from '@/stores/auth-store'

import { CTA, Features, Hero, HowItWorks, Stats } from './components'
import { useHomePageContent } from './hooks'
import { isSnowHomePage } from './lib/home-page-mode'
import { getIframeMessageTargetOrigin } from './lib/iframe-origin'
import {
  nextSnowHeaderScrollState,
  type SnowHeaderScrollState,
} from './lib/snow-header-scroll'

const SNOW_HEADER_SCROLL_INITIAL_STATE: Required<SnowHeaderScrollState> = {
  completedCycles: 0,
  offset: 0,
  scrolled: false,
}

export function Home() {
  const { i18n, t } = useTranslation()
  const iframeRef = useRef<HTMLIFrameElement>(null)
  const snowIframeRef = useRef<HTMLIFrameElement>(null)
  const detachSnowHeaderScrollRef = useRef<(() => void) | null>(null)
  const [snowHeaderScroll, setSnowHeaderScroll] = useState<
    Required<SnowHeaderScrollState>
  >(SNOW_HEADER_SCROLL_INITIAL_STATE)
  const { resolvedTheme } = useTheme()
  const { auth } = useAuthStore()
  const isAuthenticated = !!auth.user
  const { content, isLoaded, isUrl } = useHomePageContent()
  const { status, loading: statusLoading } = useStatus()
  const showSnowHome = isSnowHomePage(status?.home_page_mode)

  const resetSnowHeaderScroll = useCallback(() => {
    setSnowHeaderScroll((previous) =>
      previous.offset === 0 && !previous.scrolled
        ? previous
        : SNOW_HEADER_SCROLL_INITIAL_STATE
    )
  }, [])

  const updateSnowHeaderScroll = useCallback((deltaY: number) => {
    setSnowHeaderScroll((previous) =>
      nextSnowHeaderScrollState(previous, deltaY)
    )
  }, [])

  const attachSnowHeaderScrollSync = useCallback(() => {
    detachSnowHeaderScrollRef.current?.()
    detachSnowHeaderScrollRef.current = null
    resetSnowHeaderScroll()

    const frameWindow = snowIframeRef.current?.contentWindow
    if (!frameWindow) return

    let lastTouchY: number | null = null
    const listenerOptions: AddEventListenerOptions = {
      capture: true,
      passive: true,
    }

    const handleWheel = (event: WheelEvent) => {
      updateSnowHeaderScroll(event.deltaY)
    }

    const handleTouchStart = (event: TouchEvent) => {
      lastTouchY = event.touches[0]?.clientY ?? null
    }

    const handleTouchMove = (event: TouchEvent) => {
      const currentTouchY = event.touches[0]?.clientY ?? null
      if (lastTouchY === null || currentTouchY === null) {
        lastTouchY = currentTouchY
        return
      }

      updateSnowHeaderScroll(lastTouchY - currentTouchY)
      lastTouchY = currentTouchY
    }

    const handleKeyDown = (event: KeyboardEvent) => {
      if (
        event.key === 'ArrowDown' ||
        event.key === 'PageDown' ||
        event.key === ' '
      ) {
        updateSnowHeaderScroll(80)
      }
      if (event.key === 'ArrowUp' || event.key === 'PageUp') {
        updateSnowHeaderScroll(-80)
      }
      if (event.key === 'Home') {
        resetSnowHeaderScroll()
      }
    }

    try {
      frameWindow.addEventListener('wheel', handleWheel, listenerOptions)
      frameWindow.addEventListener(
        'touchstart',
        handleTouchStart,
        listenerOptions
      )
      frameWindow.addEventListener(
        'touchmove',
        handleTouchMove,
        listenerOptions
      )
      frameWindow.addEventListener('keydown', handleKeyDown, listenerOptions)
    } catch {
      return
    }

    detachSnowHeaderScrollRef.current = () => {
      frameWindow.removeEventListener('wheel', handleWheel, listenerOptions)
      frameWindow.removeEventListener(
        'touchstart',
        handleTouchStart,
        listenerOptions
      )
      frameWindow.removeEventListener(
        'touchmove',
        handleTouchMove,
        listenerOptions
      )
      frameWindow.removeEventListener('keydown', handleKeyDown, listenerOptions)
    }
  }, [resetSnowHeaderScroll, updateSnowHeaderScroll])

  useEffect(() => {
    if (!showSnowHome) {
      detachSnowHeaderScrollRef.current?.()
      detachSnowHeaderScrollRef.current = null
      resetSnowHeaderScroll()
    }
  }, [resetSnowHeaderScroll, showSnowHome])

  useEffect(() => {
    return () => {
      detachSnowHeaderScrollRef.current?.()
    }
  }, [])

  const syncIframePreferences = useCallback(() => {
    const targetOrigin = getIframeMessageTargetOrigin(content)
    if (!targetOrigin) return

    try {
      iframeRef.current?.contentWindow?.postMessage(
        { themeMode: resolvedTheme },
        targetOrigin
      )
      iframeRef.current?.contentWindow?.postMessage(
        { lang: i18n.language },
        targetOrigin
      )
    } catch {
      // Cross-origin frames may reject access while navigating.
    }
  }, [content, i18n.language, resolvedTheme])

  useEffect(() => {
    if (isUrl) {
      syncIframePreferences()
    }
  }, [isUrl, syncIframePreferences])

  if (!isLoaded || statusLoading) {
    return (
      <PublicLayout showMainContainer={false}>
        <main className='flex min-h-screen items-center justify-center'>
          <div className='text-muted-foreground'>{t('Loading...')}</div>
        </main>
      </PublicLayout>
    )
  }

  if (showSnowHome) {
    return (
      <PublicLayout
        showMainContainer={false}
        headerProps={{ scrolled: snowHeaderScroll.scrolled }}
      >
        <main className='h-svh'>
          {/* oxlint-disable-next-line react/iframe-missing-sandbox -- Snow is trusted, bundled same-origin content. */}
          <iframe
            ref={snowIframeRef}
            src='/snow-assets/'
            className='block h-full w-full border-none'
            title={t('Snow House Home Page')}
            allow='autoplay'
            onLoad={attachSnowHeaderScrollSync}
          />
        </main>
      </PublicLayout>
    )
  }

  if (content) {
    if (isUrl) {
      return (
        <PublicLayout showMainContainer={false}>
          {/*
            allow-top-navigation-by-user-activation: the custom home page URL is
            admin-configured (trusted); this lets its target="_top" nav/menu links
            navigate the top-level window on user click. The default sandbox blocks
            this on desktop, while some mobile browsers allow it via allow-popups,
            causing inconsistent behavior. This token only permits user-activated
            top-level navigation and does NOT grant same-origin access.
          */}
          <iframe
            ref={iframeRef}
            src={content}
            className='h-screen w-full border-none'
            title={t('Custom Home Page')}
            sandbox='allow-forms allow-popups allow-popups-to-escape-sandbox allow-scripts allow-top-navigation-by-user-activation'
            onLoad={syncIframePreferences}
          />
        </PublicLayout>
      )
    }

    const contentIsHtml = isLikelyHtml(content)

    if (contentIsHtml) {
      return (
        <PublicLayout showMainContainer={false}>
          <RichContent
            mode='html'
            htmlVariant='isolated'
            content={content}
            className='custom-home-content'
          />
        </PublicLayout>
      )
    }

    return (
      <PublicLayout>
        <div className='mx-auto max-w-6xl px-4 py-8'>
          <RichContent
            mode='markdown'
            content={content}
            className='custom-home-content'
          />
        </div>
      </PublicLayout>
    )
  }

  return (
    <PublicLayout showMainContainer={false}>
      <main>
        <Hero isAuthenticated={isAuthenticated} />
        <Stats />
        <Features />
        <HowItWorks />
        <CTA isAuthenticated={isAuthenticated} />
      </main>
      <Footer />
    </PublicLayout>
  )
}
