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
import React, { useCallback, useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { useAuthStore } from '@/stores/auth-store'

import {
  fetchTokenKey,
  fetchTokenKeysBatch,
  sendOwnPasswordSetupCode,
  setupOwnPassword,
  verifyTokenRevealPassword,
} from '../api'
import { createTokenRevealMemory } from '../lib/token-reveal-memory'

type RevealContextValue = {
  resolveKey: (id: number) => Promise<string>
  resolveKeys: (ids: number[]) => Promise<Record<number, string>>
  resolvedKeys: Record<number, string>
  loadingKeys: Record<number, boolean>
  clearRevealedKeys: () => void
}

type VerificationWaiter = {
  promise: Promise<void>
  resolve: () => void
  reject: (error: Error) => void
}

const TokenRevealContext = React.createContext<RevealContextValue | null>(null)

function responseCode(error: unknown): string {
  return String(
    (error as { response?: { data?: { code?: string } } })?.response?.data
      ?.code || ''
  )
}

function responseMessage(error: unknown, fallback: string): string {
  return String(
    (error as { response?: { data?: { message?: string } } })?.response?.data
      ?.message || fallback
  )
}

export function TokenRevealProvider({
  children,
}: {
  children: React.ReactNode
}) {
  const { t } = useTranslation()
  const user = useAuthStore((state) => state.auth.user)
  const memoryRef = useRef(createTokenRevealMemory())
  const waiterRef = useRef<VerificationWaiter | null>(null)
  const expiryTimerRef = useRef<ReturnType<typeof setTimeout>>(undefined)
  const pendingRef = useRef<Partial<Record<number, Promise<string>>>>({})
  const generationRef = useRef(0)
  const userIdRef = useRef(user?.id)
  const [resolvedKeys, setResolvedKeys] = useState<Record<number, string>>({})
  const [loadingKeys, setLoadingKeys] = useState<Record<number, boolean>>({})
  const [dialogOpen, setDialogOpen] = useState(false)
  const [setupMode, setSetupMode] = useState(false)
  const [password, setPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [verificationCode, setVerificationCode] = useState('')
  const [dialogError, setDialogError] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [sendingCode, setSendingCode] = useState(false)

  const clearRevealedKeys = useCallback(() => {
    generationRef.current += 1
    clearTimeout(expiryTimerRef.current)
    memoryRef.current.clear()
    pendingRef.current = {}
    setResolvedKeys({})
    setLoadingKeys({})
  }, [])

  const updateWindow = useCallback(
    (verifiedUntil: number) => {
      memoryRef.current.setVerifiedUntil(verifiedUntil)
      clearTimeout(expiryTimerRef.current)
      const delay = memoryRef.current.getVerifiedUntilMs() - Date.now()
      if (delay <= 0) {
        clearRevealedKeys()
        return
      }
      expiryTimerRef.current = setTimeout(clearRevealedKeys, delay)
    },
    [clearRevealedKeys]
  )

  const settleVerification = useCallback((error?: Error) => {
    const waiter = waiterRef.current
    waiterRef.current = null
    setDialogOpen(false)
    setPassword('')
    setConfirmPassword('')
    setVerificationCode('')
    setDialogError('')
    if (!waiter) return
    if (error) waiter.reject(error)
    else waiter.resolve()
  }, [])

  const requestVerification = useCallback(() => {
    if (waiterRef.current) return waiterRef.current.promise
    let resolve!: () => void
    let reject!: (error: Error) => void
    const promise = new Promise<void>((ok, fail) => {
      resolve = ok
      reject = fail
    })
    waiterRef.current = { promise, resolve, reject }
    setSetupMode(false)
    setDialogError('')
    setDialogOpen(true)
    return promise
  }, [])

  const revealWithVerification = useCallback(
    async <T,>(operation: () => Promise<T>): Promise<T> => {
      try {
        return await operation()
      } catch (error) {
        if (responseCode(error) !== 'TOKEN_REVEAL_VERIFICATION_REQUIRED') {
          throw error
        }
        clearRevealedKeys()
        await requestVerification()
        return operation()
      }
    },
    [clearRevealedKeys, requestVerification]
  )

  const resolveKey = useCallback(
    async (id: number): Promise<string> => {
      const cached = memoryRef.current.getKey(id)
      if (cached) return cached
      if (pendingRef.current[id]) return pendingRef.current[id]
      const request = revealWithVerification(async () => {
        const generation = generationRef.current
        setLoadingKeys((current) => ({ ...current, [id]: true }))
        try {
          const result = await fetchTokenKey(id)
          if (generation !== generationRef.current) {
            throw new Error('token reveal cancelled')
          }
          if (!result.payload.success || !result.payload.data?.key) {
            throw new Error(
              result.payload.message || t('Failed to load API key')
            )
          }
          updateWindow(result.verifiedUntil)
          memoryRef.current.setKey(id, result.payload.data.key)
          const key = memoryRef.current.getKey(id)
          if (!key) throw new Error(t('API key reveal window expired'))
          setResolvedKeys(memoryRef.current.snapshot())
          return key
        } finally {
          setLoadingKeys((current) => {
            const next = { ...current }
            delete next[id]
            return next
          })
        }
      }).finally(() => {
        delete pendingRef.current[id]
      })
      pendingRef.current[id] = request
      return request
    },
    [revealWithVerification, t, updateWindow]
  )

  const resolveKeys = useCallback(
    async (ids: number[]) => {
      const uniqueIds = [...new Set(ids.filter((id) => id > 0))]
      const cached: Record<number, string> = {}
      const missing: number[] = []
      for (const id of uniqueIds) {
        const key = memoryRef.current.getKey(id)
        if (key) cached[id] = key
        else missing.push(id)
      }
      if (missing.length === 0) return cached
      const revealed = await revealWithVerification(async () => {
        const generation = generationRef.current
        setLoadingKeys((current) => {
          const next = { ...current }
          for (const id of missing) next[id] = true
          return next
        })
        try {
          const result = await fetchTokenKeysBatch(missing)
          if (generation !== generationRef.current) {
            throw new Error('token reveal cancelled')
          }
          if (!result.payload.success || !result.payload.data?.keys) {
            throw new Error(
              result.payload.message || t('Failed to load API keys')
            )
          }
          updateWindow(result.verifiedUntil)
          memoryRef.current.setKeys(result.payload.data.keys)
          setResolvedKeys(memoryRef.current.snapshot())
          return memoryRef.current.snapshot()
        } finally {
          setLoadingKeys((current) => {
            const next = { ...current }
            for (const id of missing) delete next[id]
            return next
          })
        }
      })
      return { ...cached, ...revealed }
    },
    [revealWithVerification, t, updateWindow]
  )

  useEffect(() => {
    if (userIdRef.current === user?.id) return
    userIdRef.current = user?.id
    clearRevealedKeys()
    if (!user?.id && waiterRef.current) {
      settleVerification(new Error('token reveal cancelled'))
    }
  }, [user?.id, clearRevealedKeys, settleVerification])

  useEffect(
    () => () => {
      clearRevealedKeys()
      waiterRef.current?.reject(new Error('token reveal provider unmounted'))
      waiterRef.current = null
    },
    [clearRevealedKeys]
  )

  const handleVerify = async () => {
    if (!password) {
      setDialogError(t('Password is required'))
      return
    }
    setSubmitting(true)
    setDialogError('')
    try {
      const result = await verifyTokenRevealPassword(password)
      if (!result.success || !result.data?.verified_until) {
        throw new Error(result.message || t('Password verification failed'))
      }
      updateWindow(result.data.verified_until)
      settleVerification()
    } catch (error) {
      if (responseCode(error) === 'PASSWORD_SETUP_REQUIRED') {
        setSetupMode(true)
        setDialogError('')
      } else {
        setDialogError(
          responseMessage(error, t('Password verification failed'))
        )
      }
    } finally {
      setSubmitting(false)
    }
  }

  const handleSendCode = async () => {
    setSendingCode(true)
    setDialogError('')
    try {
      const result = await sendOwnPasswordSetupCode()
      if (!result.success) {
        throw new Error(result.message || t('Failed to send verification code'))
      }
    } catch (error) {
      setDialogError(
        responseMessage(error, t('Failed to send verification code'))
      )
    } finally {
      setSendingCode(false)
    }
  }

  const handleSetup = async () => {
    if (password.length < 8 || password.length > 20) {
      setDialogError(t('Password must be 8 to 20 characters'))
      return
    }
    if (password !== confirmPassword) {
      setDialogError(t('Passwords do not match'))
      return
    }
    if (!verificationCode.trim()) {
      setDialogError(t('Verification code is required'))
      return
    }
    setSubmitting(true)
    setDialogError('')
    try {
      const setup = await setupOwnPassword(password, verificationCode.trim())
      if (!setup.success) {
        throw new Error(setup.message || t('Failed to set password'))
      }
      const verified = await verifyTokenRevealPassword(password)
      if (!verified.success || !verified.data?.verified_until) {
        throw new Error(verified.message || t('Password verification failed'))
      }
      updateWindow(verified.data.verified_until)
      settleVerification()
    } catch (error) {
      setDialogError(
        responseMessage(
          error,
          (error as Error).message || t('Failed to set password')
        )
      )
    } finally {
      setSubmitting(false)
    }
  }

  let submitLabel = t('Verify')
  if (setupMode) submitLabel = t('Set password')
  if (submitting) submitLabel = t('Verifying...')

  return (
    <TokenRevealContext.Provider
      value={{
        resolveKey,
        resolveKeys,
        resolvedKeys,
        loadingKeys,
        clearRevealedKeys,
      }}
    >
      {children}
      <Dialog
        open={dialogOpen}
        onOpenChange={(open) => {
          if (!open && !submitting) {
            settleVerification(new Error('token reveal cancelled'))
          }
        }}
      >
        <DialogContent className='max-sm:w-[calc(100vw-1.5rem)] sm:max-w-md'>
          <DialogHeader>
            <DialogTitle>
              {setupMode
                ? t('Set account password')
                : t('Verify account password')}
            </DialogTitle>
            <DialogDescription>
              {setupMode
                ? t(
                    'Verify your bound email and set a password before viewing API keys.'
                  )
                : t(
                    'Verification allows access to all of your API keys for 30 minutes.'
                  )}
            </DialogDescription>
          </DialogHeader>
          <div className='space-y-4'>
            {setupMode && (
              <div className='space-y-2'>
                <Label>{t('Bound email')}</Label>
                <Input value={user?.email || ''} readOnly />
              </div>
            )}
            <div className='space-y-2'>
              <Label htmlFor='token-reveal-password'>{t('Password')}</Label>
              <Input
                id='token-reveal-password'
                type='password'
                autoComplete={setupMode ? 'new-password' : 'current-password'}
                value={password}
                onChange={(event) => setPassword(event.target.value)}
                onKeyDown={(event) => {
                  if (event.key === 'Enter' && !setupMode) void handleVerify()
                }}
              />
            </div>
            {setupMode && (
              <>
                <div className='space-y-2'>
                  <Label htmlFor='token-reveal-password-confirm'>
                    {t('Confirm password')}
                  </Label>
                  <Input
                    id='token-reveal-password-confirm'
                    type='password'
                    autoComplete='new-password'
                    value={confirmPassword}
                    onChange={(event) => setConfirmPassword(event.target.value)}
                  />
                </div>
                <div className='space-y-2'>
                  <Label htmlFor='token-reveal-code'>
                    {t('Email verification code')}
                  </Label>
                  <div className='flex gap-2'>
                    <Input
                      id='token-reveal-code'
                      inputMode='numeric'
                      value={verificationCode}
                      onChange={(event) =>
                        setVerificationCode(event.target.value)
                      }
                    />
                    <Button
                      type='button'
                      variant='outline'
                      disabled={sendingCode}
                      onClick={handleSendCode}
                    >
                      {sendingCode ? t('Sending...') : t('Send code')}
                    </Button>
                  </div>
                </div>
              </>
            )}
            {dialogError && (
              <p className='text-destructive text-sm'>{dialogError}</p>
            )}
          </div>
          <DialogFooter>
            <Button
              type='button'
              variant='outline'
              disabled={submitting}
              onClick={() =>
                settleVerification(new Error('token reveal cancelled'))
              }
            >
              {t('Cancel')}
            </Button>
            <Button
              type='button'
              disabled={submitting}
              onClick={setupMode ? handleSetup : handleVerify}
            >
              {submitLabel}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </TokenRevealContext.Provider>
  )
}

// eslint-disable-next-line react-refresh/only-export-components
export function useTokenReveal() {
  const context = React.useContext(TokenRevealContext)
  if (!context) {
    throw new Error('useTokenReveal must be used within TokenRevealProvider')
  }
  return context
}
