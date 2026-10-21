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
import { Mail, Shield, Send, Link2, Unlink } from 'lucide-react'
import { useEffect, useMemo, useState, useCallback } from 'react'
import { useTranslation } from 'react-i18next'
import { SiGithub, SiWechat, SiLinux } from 'react-icons/si'
import { toast } from 'sonner'

import { IconDiscord } from '@/assets/brand-icons'
import { ConfirmDialog } from '@/components/confirm-dialog'
import { StatusBadge } from '@/components/status-badge'
import { Button } from '@/components/ui/button'
import { Separator } from '@/components/ui/separator'
import { OAUTH_BIND_STORAGE_KEY } from '@/features/auth/constants'
import {
  getOAuthSessionStorage,
  markOAuthBindPopup,
} from '@/features/auth/lib/oauth-callback-mode'
import { useDialogs } from '@/hooks/use-dialog'
import { useStatus } from '@/hooks/use-status'
import {
  buildDiscordOAuthUrl,
  buildGitHubOAuthUrl,
  buildLinuxDOOAuthUrl,
  buildOIDCOAuthUrl,
  buildCustomOAuthUrl,
  getOAuthState,
} from '@/lib/oauth'

import {
  getSelfOAuthBindings,
  unbindCustomOAuth,
  unbindSelfBinding,
  type CustomOAuthBinding,
} from '../../api'
import type {
  UserProfile,
  BindingItem,
  CustomOAuthProviderInfo,
} from '../../types'
import { EmailBindDialog } from '../dialogs/email-bind-dialog'
import { TelegramBindDialog } from '../dialogs/telegram-bind-dialog'
import { WeChatBindDialog } from '../dialogs/wechat-bind-dialog'

// ============================================================================
// Account Bindings Tab Component
// ============================================================================

interface AccountBindingsTabProps {
  profile: UserProfile | null
  onUpdate: () => void
}

type DialogKey = 'email' | 'wechat' | 'telegram'
type BuiltInUnbindTarget = {
  id: string
  label: string
}

type CustomOAuthBindingItem = {
  key: string
  name: string
  icon?: string
  isEnabled: boolean
  isBound: boolean
  binding: CustomOAuthBinding | null
  provider: CustomOAuthProviderInfo | null
}

export function AccountBindingsTab({
  profile,
  onUpdate,
}: AccountBindingsTabProps) {
  const { t } = useTranslation()
  const dialogs = useDialogs<DialogKey>()
  const { status, loading } = useStatus()
  const [customBindings, setCustomBindings] = useState<CustomOAuthBinding[]>([])
  const [unbindTarget, setUnbindTarget] = useState<CustomOAuthBinding | null>(
    null
  )
  const [builtInUnbindTarget, setBuiltInUnbindTarget] =
    useState<BuiltInUnbindTarget | null>(null)
  const [unbinding, setUnbinding] = useState(false)
  const [unbindingBuiltIn, setUnbindingBuiltIn] = useState(false)

  const customProviders = useMemo(
    () => (status?.custom_oauth_providers ?? []) as CustomOAuthProviderInfo[],
    [status?.custom_oauth_providers]
  )

  const fetchCustomBindings = useCallback(async () => {
    try {
      const res = await getSelfOAuthBindings()
      if (res.success && res.data) {
        setCustomBindings(res.data)
      }
    } catch {
      // ignore
    }
  }, [])

  useEffect(() => {
    fetchCustomBindings()
  }, [fetchCustomBindings])

  const handleUnbindCustom = async () => {
    if (!unbindTarget) return
    setUnbinding(true)
    try {
      const res = await unbindCustomOAuth(unbindTarget.provider_id)
      if (res.success) {
        toast.success(
          t('Unbound {{provider}}', {
            provider: unbindTarget.provider_name,
          })
        )
        await fetchCustomBindings()
        onUpdate()
      } else {
        toast.error(res.message || t('Unbind failed'))
      }
    } catch {
      toast.error(t('Unbind failed'))
    } finally {
      setUnbinding(false)
      setUnbindTarget(null)
    }
  }

  const handleUnbindBuiltIn = async () => {
    if (!builtInUnbindTarget) return
    setUnbindingBuiltIn(true)
    try {
      const res = await unbindSelfBinding(builtInUnbindTarget.id)
      if (res.success) {
        toast.success(
          t('Unbound {{provider}}', { provider: builtInUnbindTarget.label })
        )
        onUpdate()
      } else {
        toast.error(res.message || t('Unbind failed'))
      }
    } catch {
      toast.error(t('Unbind failed'))
    } finally {
      setUnbindingBuiltIn(false)
      setBuiltInUnbindTarget(null)
    }
  }

  const startOAuthBinding = useCallback(
    async (provider: string, buildUrl: (state: string) => string) => {
      const popup = window.open('', '_blank')
      if (!popup) {
        toast.error(t('Failed to initialize OAuth'))
        return
      }
      try {
        const state = await getOAuthState()
        if (
          !state ||
          !markOAuthBindPopup(getOAuthSessionStorage(popup), provider, state)
        ) {
          throw new Error('OAuth bind marker unavailable')
        }
        popup.location.replace(buildUrl(state))
      } catch {
        popup.close()
        toast.error(t('Failed to initialize OAuth'))
      }
    },
    [t]
  )

  const handleBindCustomOAuth = async (provider: CustomOAuthProviderInfo) => {
    const slug = provider.slug?.trim()
    if (!slug || !provider.authorization_endpoint || !provider.client_id) {
      toast.error(t('OAuth provider is missing configuration'))
      return
    }
    await startOAuthBinding(slug, (state) =>
      buildCustomOAuthUrl(provider, state)
    )
  }

  useEffect(() => {
    if (typeof window === 'undefined') return

    const handleStorage = (event: StorageEvent) => {
      if (event.key !== OAUTH_BIND_STORAGE_KEY || !event.newValue) return
      try {
        const payload = JSON.parse(event.newValue) as {
          status?: string
          provider?: string
          timestamp?: number
        }
        if (payload?.status === 'success') {
          onUpdate()
        }
      } catch {
        // ignore malformed payloads
      }
      try {
        window.localStorage.removeItem(OAUTH_BIND_STORAGE_KEY)
      } catch {
        // ignore cleanup failure
      }
    }

    window.addEventListener('storage', handleStorage)
    return () => window.removeEventListener('storage', handleStorage)
  }, [onUpdate])

  // Memoize bindings to prevent unnecessary recalculations
  const bindings: BindingItem[] = useMemo(() => {
    if (!profile || !status) return []

    return [
      {
        id: 'email',
        label: t('Email'),
        icon: Mail,
        value: profile.email,
        isBound: Boolean(profile.email),
        isEnabled: true,
        onBind: () => dialogs.open('email'),
      },
      {
        id: 'wechat',
        label: t('WeChat'),
        icon: SiWechat as React.ComponentType<{ className?: string }>,
        value: undefined,
        isBound: Boolean(
          (profile as unknown as Record<string, unknown>).wechat_id
        ),
        isEnabled: status?.wechat_login || false,
        onBind: () => dialogs.open('wechat'),
        onUnbind: () =>
          setBuiltInUnbindTarget({ id: 'wechat', label: t('WeChat') }),
      },
      {
        id: 'github',
        label: t('GitHub'),
        icon: SiGithub,
        value: (profile as unknown as Record<string, unknown>).github_id as
          | string
          | undefined,
        isBound: Boolean(
          (profile as unknown as Record<string, unknown>).github_id
        ),
        isEnabled: status?.github_oauth || false,
        onBind: () => {
          const clientId = status?.github_client_id
          if (clientId) {
            void startOAuthBinding('github', (state) =>
              buildGitHubOAuthUrl(clientId, state)
            )
          }
        },
        onUnbind: () =>
          setBuiltInUnbindTarget({ id: 'github', label: t('GitHub') }),
      },
      {
        id: 'discord',
        label: t('Discord'),
        icon: IconDiscord,
        value: (profile as unknown as Record<string, unknown>).discord_id as
          | string
          | undefined,
        isBound: Boolean(
          (profile as unknown as Record<string, unknown>).discord_id
        ),
        isEnabled: status?.discord_oauth || false,
        onBind: () => {
          const clientId = status?.discord_client_id
          if (clientId) {
            void startOAuthBinding('discord', (state) =>
              buildDiscordOAuthUrl(clientId, state)
            )
          }
        },
        onUnbind: () =>
          setBuiltInUnbindTarget({ id: 'discord', label: t('Discord') }),
      },
      {
        id: 'oidc',
        label: t('OIDC'),
        icon: Shield,
        value: (profile as unknown as Record<string, unknown>).oidc_id as
          | string
          | undefined,
        isBound: Boolean(
          (profile as unknown as Record<string, unknown>).oidc_id
        ),
        isEnabled: status?.oidc_enabled || false,
        onBind: () => {
          const authorizationEndpoint = status?.oidc_authorization_endpoint
          const clientId = status?.oidc_client_id
          if (authorizationEndpoint && clientId) {
            void startOAuthBinding('oidc', (state) =>
              buildOIDCOAuthUrl(authorizationEndpoint, clientId, state)
            )
          }
        },
        onUnbind: () =>
          setBuiltInUnbindTarget({ id: 'oidc', label: t('OIDC') }),
      },
      {
        id: 'telegram',
        label: t('Telegram'),
        icon: Send,
        value: (profile as unknown as Record<string, unknown>).telegram_id as
          | string
          | undefined,
        isBound: Boolean(
          (profile as unknown as Record<string, unknown>).telegram_id
        ),
        isEnabled: status?.telegram_oauth || false,
        onBind: () => dialogs.open('telegram'),
        onUnbind: () =>
          setBuiltInUnbindTarget({ id: 'telegram', label: t('Telegram') }),
      },
      {
        id: 'linuxdo',
        label: t('LinuxDO'),
        icon: SiLinux as React.ComponentType<{ className?: string }>,
        value: (profile as unknown as Record<string, unknown>).linux_do_id as
          | string
          | undefined,
        isBound: Boolean(
          (profile as unknown as Record<string, unknown>).linux_do_id
        ),
        isEnabled: status?.linuxdo_oauth || false,
        onBind: () => {
          const clientId = status?.linuxdo_client_id
          if (clientId) {
            void startOAuthBinding('linuxdo', (state) =>
              buildLinuxDOOAuthUrl(clientId, state)
            )
          }
        },
        onUnbind: () =>
          setBuiltInUnbindTarget({ id: 'linuxdo', label: t('LinuxDO') }),
      },
    ].filter((binding) => binding.isEnabled || binding.isBound)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [profile, status, t])

  const customBindingItems = useMemo<CustomOAuthBindingItem[]>(() => {
    const bindingMap = new Map(
      customBindings.map((binding) => [String(binding.provider_id), binding])
    )
    const seenProviderIds = new Set<string>()
    const items: CustomOAuthBindingItem[] = []

    for (const provider of customProviders) {
      const providerId = String(provider.id)
      seenProviderIds.add(providerId)
      const binding = bindingMap.get(providerId) ?? null
      items.push({
        key: `custom_${providerId}`,
        name: provider.name,
        icon: provider.icon,
        isEnabled: true,
        isBound: !!binding,
        binding,
        provider,
      })
    }

    for (const binding of customBindings) {
      const providerId = String(binding.provider_id)
      if (seenProviderIds.has(providerId)) continue
      items.push({
        key: `custom_${providerId}`,
        name: binding.provider_name,
        icon: binding.provider_icon,
        isEnabled: false,
        isBound: true,
        binding,
        provider: null,
      })
    }

    return items
  }, [customBindings, customProviders])

  if (!profile || loading) return null

  return (
    <>
      <div className='grid grid-cols-1 gap-2.5 sm:grid-cols-2 sm:gap-3'>
        {bindings.map((binding) => (
          <div
            key={binding.id}
            className='flex items-center justify-between gap-2.5 rounded-lg border p-2.5 sm:gap-3 sm:p-3'
          >
            <div className='flex min-w-0 items-center gap-2.5 sm:gap-3'>
              <div className='bg-muted shrink-0 rounded-md p-1.5 sm:p-2'>
                <binding.icon className='h-4 w-4' />
              </div>
              <div className='min-w-0'>
                <div className='flex items-center gap-1.5'>
                  <p className='text-sm font-medium'>{binding.label}</p>
                  {binding.isBound && (
                    <StatusBadge
                      label={t('Bound')}
                      variant='success'
                      copyable={false}
                    />
                  )}
                  {!binding.isEnabled && (
                    <StatusBadge
                      label={t('Disabled')}
                      variant='neutral'
                      copyable={false}
                    />
                  )}
                </div>
                <p className='text-muted-foreground truncate text-xs'>
                  {binding.value || t('Not bound')}
                </p>
              </div>
            </div>
            {binding.isBound && binding.onUnbind ? (
              <Button
                variant='ghost'
                size='sm'
                className='text-destructive h-7 shrink-0 px-2.5 text-xs'
                onClick={binding.onUnbind}
              >
                <Unlink className='mr-1 h-3 w-3' />
                {t('Unbind')}
              </Button>
            ) : (
              <Button
                variant='outline'
                size='sm'
                className='h-7 shrink-0 px-2.5 text-xs'
                onClick={binding.onBind}
              >
                {binding.isBound && binding.id === 'email'
                  ? t('Change')
                  : t('Bind')}
              </Button>
            )}
          </div>
        ))}
      </div>

      {/* Custom OAuth Bindings */}
      {customBindingItems.length > 0 && (
        <>
          <Separator className='my-4' />
          <p className='text-muted-foreground mb-3 text-sm font-medium'>
            {t('Custom OAuth')}
          </p>
          <div className='grid grid-cols-1 gap-2.5 sm:grid-cols-2 sm:gap-3'>
            {customBindingItems.map((item) => {
              return (
                <div
                  key={item.key}
                  className='flex items-center justify-between gap-2.5 rounded-lg border p-2.5 sm:gap-3 sm:p-3'
                >
                  <div className='flex min-w-0 items-center gap-2.5 sm:gap-3'>
                    <div className='bg-muted shrink-0 rounded-md p-1.5 sm:p-2'>
                      {item.icon ? (
                        <img
                          src={item.icon}
                          alt=''
                          className='h-4 w-4 rounded-sm object-contain'
                          onError={(event) => {
                            event.currentTarget.style.display = 'none'
                          }}
                        />
                      ) : (
                        <Link2 className='h-4 w-4' />
                      )}
                    </div>
                    <div className='min-w-0'>
                      <div className='flex items-center gap-1.5'>
                        <p className='text-sm font-medium'>{item.name}</p>
                        {item.isBound && (
                          <StatusBadge
                            label={t('Bound')}
                            variant='success'
                            copyable={false}
                          />
                        )}
                        {!item.isEnabled && (
                          <StatusBadge
                            label={t('Disabled')}
                            variant='neutral'
                            copyable={false}
                          />
                        )}
                      </div>
                      <p className='text-muted-foreground truncate text-xs'>
                        {item.isBound
                          ? item.binding?.provider_user_id ||
                            item.binding?.external_id ||
                            t('Bound')
                          : t('Not bound')}
                      </p>
                    </div>
                  </div>
                  {item.isBound && item.binding ? (
                    <Button
                      variant='ghost'
                      size='sm'
                      className='text-destructive h-7 shrink-0 px-2.5 text-xs'
                      onClick={() => setUnbindTarget(item.binding)}
                    >
                      <Unlink className='mr-1 h-3 w-3' />
                      {t('Unbind')}
                    </Button>
                  ) : item.provider ? (
                    <Button
                      variant='outline'
                      size='sm'
                      className='h-7 shrink-0 px-2.5 text-xs'
                      onClick={() => {
                        if (item.provider) {
                          handleBindCustomOAuth(item.provider)
                        }
                      }}
                    >
                      {t('Bind')}
                    </Button>
                  ) : (
                    <span className='w-0 shrink-0' />
                  )}
                </div>
              )
            })}
          </div>
        </>
      )}

      {/* Custom OAuth Unbind Confirmation */}
      <ConfirmDialog
        open={!!unbindTarget}
        onOpenChange={(open) => !open && setUnbindTarget(null)}
        title={t('Confirm Unbind')}
        desc={t(
          'Are you sure you want to unbind {{provider}}? You will no longer be able to log in via this method.',
          {
            provider: unbindTarget?.provider_name || '',
          }
        )}
        confirmText={t('Confirm Unbind')}
        destructive
        handleConfirm={handleUnbindCustom}
        isLoading={unbinding}
      />

      <ConfirmDialog
        open={!!builtInUnbindTarget}
        onOpenChange={(open) => !open && setBuiltInUnbindTarget(null)}
        title={t('Confirm Unbind')}
        desc={t(
          'Are you sure you want to unbind {{provider}}? You will no longer be able to log in via this method.',
          {
            provider: builtInUnbindTarget?.label || '',
          }
        )}
        confirmText={t('Confirm Unbind')}
        destructive
        handleConfirm={handleUnbindBuiltIn}
        isLoading={unbindingBuiltIn}
      />

      {/* Email Bind Dialog */}
      <EmailBindDialog
        open={dialogs.isOpen('email')}
        onOpenChange={(open) =>
          open ? dialogs.open('email') : dialogs.close('email')
        }
        currentEmail={profile.email}
        onSuccess={onUpdate}
      />

      {/* WeChat Bind Dialog */}
      <WeChatBindDialog
        open={dialogs.isOpen('wechat')}
        onOpenChange={(open) =>
          open ? dialogs.open('wechat') : dialogs.close('wechat')
        }
        onSuccess={onUpdate}
      />

      {/* Telegram Bind Dialog */}
      {status?.telegram_bot_name && (
        <TelegramBindDialog
          open={dialogs.isOpen('telegram')}
          onOpenChange={(open) =>
            open ? dialogs.open('telegram') : dialogs.close('telegram')
          }
          botName={status.telegram_bot_name as string}
          onSuccess={onUpdate}
        />
      )}
    </>
  )
}
