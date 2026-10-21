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
import { Eye, EyeOff, KeyRound, LoaderCircle, Search } from 'lucide-react'
import { useRef, useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'

import { PublicLayout } from '@/components/layout'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'

import { queryTokenBalance } from './api'
import { BalanceQueryError } from './components/balance-query-error'
import { BalanceResult } from './components/balance-result'
import type { TokenBalanceData } from './types'

type BalanceQueryErrorState = {
  code: string
  retryAfterSeconds?: number
}

export function TokenBalancePage() {
  const { t } = useTranslation()
  const inputRef = useRef<HTMLInputElement>(null)
  const [key, setKey] = useState('')
  const [showKey, setShowKey] = useState(false)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<BalanceQueryErrorState | null>(null)
  const [result, setResult] = useState<TokenBalanceData | null>(null)

  const handleSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    const normalizedKey = key.trim()
    if (!normalizedKey) {
      setResult(null)
      setError({ code: 'invalid_request' })
      inputRef.current?.focus()
      return
    }

    setLoading(true)
    setError(null)
    setResult(null)
    try {
      const response = await queryTokenBalance(normalizedKey)
      if (!response.success || !response.data) {
        setResult(null)
        setError({
          code: response.code || 'query_failed',
          retryAfterSeconds: response.retry_after_seconds,
        })
        setKey('')
        setShowKey(false)
        return
      }
      setResult(response.data)
      setKey('')
      setShowKey(false)
    } catch {
      setResult(null)
      setError({ code: 'query_failed' })
      setKey('')
      setShowKey(false)
    } finally {
      setLoading(false)
    }
  }

  return (
    <PublicLayout>
      <div className='mx-auto w-full max-w-3xl space-y-4 py-5 sm:py-8'>
        <div className='mb-6 flex items-center gap-3'>
          <div className='bg-muted flex size-9 shrink-0 items-center justify-center rounded-lg border'>
            <KeyRound className='size-4' />
          </div>
          <h1 className='text-xl font-semibold tracking-normal sm:text-2xl'>
            {t('API Key Balance')}
          </h1>
        </div>

        <Card>
          <CardContent>
            <form onSubmit={handleSubmit} className='space-y-2.5'>
              <Label htmlFor='token-balance-key'>{t('API Key')}</Label>
              <div className='flex flex-col gap-2 sm:flex-row'>
                <div className='relative min-w-0 flex-1'>
                  <Input
                    ref={inputRef}
                    id='token-balance-key'
                    name='token-balance-key'
                    type={showKey ? 'text' : 'password'}
                    value={key}
                    onChange={(event) => {
                      setKey(event.target.value)
                      if (error?.code === 'invalid_request') setError(null)
                    }}
                    placeholder={t('Enter API Key')}
                    autoComplete='off'
                    autoCapitalize='none'
                    spellCheck={false}
                    className='h-9 pr-10 font-mono text-sm'
                    aria-invalid={error !== null}
                  />
                  <Button
                    type='button'
                    variant='ghost'
                    size='icon-sm'
                    className='absolute top-1 right-1'
                    onClick={() => setShowKey((visible) => !visible)}
                    aria-label={showKey ? t('Hide API Key') : t('Show API Key')}
                    title={showKey ? t('Hide API Key') : t('Show API Key')}
                  >
                    {showKey ? <EyeOff /> : <Eye />}
                  </Button>
                </div>
                <Button
                  type='submit'
                  className='h-9 sm:min-w-28'
                  disabled={loading}
                >
                  {loading ? (
                    <LoaderCircle className='animate-spin' />
                  ) : (
                    <Search />
                  )}
                  {loading ? t('Querying...') : t('Query Balance')}
                </Button>
              </div>
            </form>
          </CardContent>
        </Card>

        {error ? (
          <BalanceQueryError
            code={error.code}
            retryAfterSeconds={error.retryAfterSeconds}
          />
        ) : null}

        {result ? <BalanceResult data={result} /> : null}
      </div>
    </PublicLayout>
  )
}
