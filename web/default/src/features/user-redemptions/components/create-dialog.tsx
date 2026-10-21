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
import { useQuery } from '@tanstack/react-query'
import { Copy, Gift, Layers3, Loader2, Ticket } from 'lucide-react'
import { useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

import {
  SideDrawerSection,
  SideDrawerSectionHeader,
  sideDrawerContentClassName,
  sideDrawerFooterClassName,
  sideDrawerFormClassName,
  sideDrawerHeaderClassName,
} from '@/components/drawer-layout'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Sheet,
  SheetClose,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { getPublicPlans } from '@/features/subscriptions/api'
import { formatSubscriptionPrice } from '@/features/subscriptions/lib/format'
import { useCopyToClipboard } from '@/hooks/use-copy-to-clipboard'
import {
  formatQuota,
  parseQuotaFromDollars,
  quotaUnitsToDollars,
} from '@/lib/format'
import {
  DEFAULT_CURRENCY_CONFIG,
  useSystemConfigStore,
} from '@/stores/system-config-store'

import {
  buildCreateUserRedemptionRequest,
  createUserRedemption,
  type UserRedemptionCode,
} from '../api'
import {
  calculateSubscriptionRedemptionCost,
  getMinimumRedemptionAmount,
  getRedemptionAmountValidation,
  getUserRedemptionExpiryOptions,
  getUserRedemptionExpiryTranslation,
  normalizeRedemptionAmount,
  sanitizeRedemptionAmountInput,
} from '../create-state'
import type { UserRedemptionKind, UserRedemptionRewardType } from '../types'

type BalanceRedemptionKind = Exclude<UserRedemptionKind, 'fixed'>

type CreateUserRedemptionDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  onCreated?: (code: UserRedemptionCode) => void | Promise<void>
  singleEnabled?: boolean
  randomEnabled?: boolean
  subscriptionEnabled?: boolean
  maxClaimsPerCode?: number
  maxExpiryHours?: number
}

export function CreateUserRedemptionDialog(
  props: CreateUserRedemptionDialogProps
) {
  const { t } = useTranslation()
  const { copyToClipboard } = useCopyToClipboard()
  const configuredQuotaPerUnit = useSystemConfigStore(
    (state) => state.config.currency.quotaPerUnit
  )
  const quotaPerUnit =
    configuredQuotaPerUnit > 0
      ? configuredQuotaPerUnit
      : DEFAULT_CURRENCY_CONFIG.quotaPerUnit
  const minimumUnitAmount = quotaUnitsToDollars(quotaPerUnit)
  const singleEnabled = props.singleEnabled !== false
  const randomEnabled = props.randomEnabled !== false
  const balanceEnabled = singleEnabled || randomEnabled
  const subscriptionEnabled = props.subscriptionEnabled !== false
  const maxClaimsPerCode = props.maxClaimsPerCode ?? 100
  const maxExpiryHours = props.maxExpiryHours ?? 24 * 30
  const defaultRewardType: UserRedemptionRewardType = balanceEnabled
    ? 'balance'
    : 'subscription'
  const defaultKind: BalanceRedemptionKind = singleEnabled ? 'single' : 'random'
  const [rewardType, setRewardType] =
    useState<UserRedemptionRewardType>(defaultRewardType)
  const [kind, setKind] = useState<BalanceRedemptionKind>(defaultKind)
  const [amount, setAmount] = useState(() => minimumUnitAmount)
  const [maxClaims, setMaxClaims] = useState(2)
  const [planId, setPlanId] = useState(0)
  const [units, setUnits] = useState(1)
  const [hours, setHours] = useState<number>(24)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [createdCode, setCreatedCode] = useState<UserRedemptionCode | null>(
    null
  )
  const plansQuery = useQuery({
    queryKey: ['subscription-plans', 'public', 'redemption-creation'],
    queryFn: getPublicPlans,
    enabled: props.open && rewardType === 'subscription',
  })
  const eligiblePlans = useMemo(
    () =>
      (plansQuery.data?.data ?? []).filter(
        ({ plan }) => plan.enabled && plan.allow_balance_pay !== false
      ),
    [plansQuery.data]
  )
  const selectedPlan = eligiblePlans.find(
    ({ plan }) => plan.id === planId
  )?.plan
  const subscriptionCost = calculateSubscriptionRedemptionCost(
    selectedPlan?.price_amount ?? 0,
    units,
    quotaPerUnit
  )
  const expiryOptions = useMemo(
    () => getUserRedemptionExpiryOptions(maxExpiryHours),
    [maxExpiryHours]
  )
  const minimumAmount = getMinimumRedemptionAmount(
    kind,
    maxClaims,
    minimumUnitAmount
  )
  const amountValidation = getRedemptionAmountValidation(
    kind,
    amount,
    maxClaims,
    minimumUnitAmount
  )

  useEffect(() => {
    if (!props.open) return
    setAmount(minimumUnitAmount)
    setRewardType((current) => {
      if (current === 'balance' && balanceEnabled) return current
      if (current === 'subscription' && subscriptionEnabled) return current
      return defaultRewardType
    })
    setKind((current) => {
      if (current === 'single' && singleEnabled) return current
      if (current === 'random' && randomEnabled) return current
      return defaultKind
    })
    setMaxClaims((current) => Math.min(Math.max(current, 2), maxClaimsPerCode))
    setUnits((current) => Math.min(Math.max(current, 1), maxClaimsPerCode))
    setHours((current) => {
      if (current <= maxExpiryHours) return current
      return expiryOptions.at(-1) ?? 1
    })
  }, [
    balanceEnabled,
    defaultKind,
    defaultRewardType,
    expiryOptions,
    maxClaimsPerCode,
    maxExpiryHours,
    minimumUnitAmount,
    props.open,
    randomEnabled,
    singleEnabled,
    subscriptionEnabled,
  ])

  useEffect(() => {
    if (rewardType !== 'subscription' || eligiblePlans.length === 0) return
    if (eligiblePlans.some(({ plan }) => plan.id === planId)) return
    setPlanId(eligiblePlans[0].plan.id)
  }, [eligiblePlans, planId, rewardType])

  function reset() {
    setRewardType(defaultRewardType)
    setKind(defaultKind)
    setAmount(minimumUnitAmount)
    setMaxClaims(2)
    setPlanId(0)
    setUnits(1)
    setHours(Math.min(24, maxExpiryHours))
    setError('')
    setCreatedCode(null)
  }

  async function handleCreate() {
    if (rewardType === 'subscription' && !subscriptionEnabled) {
      setError(t('This redemption mode is currently disabled'))
      return
    }
    if (
      rewardType === 'balance' &&
      ((kind === 'single' && !singleEnabled) ||
        (kind === 'random' && !randomEnabled))
    ) {
      setError(t('This redemption mode is currently disabled'))
      return
    }

    setBusy(true)
    setError('')
    try {
      const expiresAt = Math.floor(Date.now() / 1000) + hours * 3600
      const request =
        rewardType === 'subscription'
          ? buildCreateUserRedemptionRequest({
              rewardType: 'subscription',
              planId,
              units,
              expiresAt,
            })
          : buildCreateUserRedemptionRequest({
              rewardType: 'balance',
              kind,
              totalQuota: String(
                parseQuotaFromDollars(
                  normalizeRedemptionAmount(
                    kind,
                    amount,
                    maxClaims,
                    minimumUnitAmount,
                    maxClaimsPerCode
                  )
                )
              ),
              maxClaims: kind === 'random' ? maxClaims : 1,
              expiresAt,
            })
      const code = await createUserRedemption(request)
      setCreatedCode(code)
      await props.onCreated?.(code)
    } catch (caught: unknown) {
      setError(
        caught instanceof Error && caught.message
          ? caught.message
          : t('create failed')
      )
    } finally {
      setBusy(false)
    }
  }

  const creationDisabled =
    busy ||
    (rewardType === 'balance'
      ? amountValidation !== null
      : plansQuery.isLoading || !selectedPlan || units < 1)

  return (
    <Sheet
      open={props.open}
      onOpenChange={(open) => {
        props.onOpenChange(open)
        if (!open) reset()
      }}
    >
      <SheetContent className={sideDrawerContentClassName('sm:max-w-[560px]')}>
        <SheetHeader className={sideDrawerHeaderClassName()}>
          <SheetTitle>{t('Create Redemption Code')}</SheetTitle>
          <SheetDescription>
            {t('Reserve wallet balance in a code you can share.')}
          </SheetDescription>
        </SheetHeader>

        <div className={sideDrawerFormClassName()}>
          {createdCode ? (
            <SideDrawerSection>
              <SideDrawerSectionHeader
                title={t('Redemption code created')}
                description={t(
                  'Copy it now and share it only with the recipient.'
                )}
                icon={<Ticket />}
                iconTone='success'
              />
              <Alert>
                <AlertTitle>{t('Your redemption code')}</AlertTitle>
                <AlertDescription className='mt-3 flex min-w-0 gap-2'>
                  <Input
                    value={createdCode.code}
                    readOnly
                    aria-label={t('Redemption code')}
                    className='min-w-0 flex-1 font-mono'
                  />
                  <Button
                    type='button'
                    variant='outline'
                    size='icon'
                    aria-label={t('Copy redemption code')}
                    onClick={() => void copyToClipboard(createdCode.code)}
                  >
                    <Copy aria-hidden='true' />
                  </Button>
                </AlertDescription>
              </Alert>
              <Button
                type='button'
                variant='outline'
                className='w-fit'
                onClick={() => setCreatedCode(null)}
              >
                {t('Create another code')}
              </Button>
            </SideDrawerSection>
          ) : (
            <>
              <SideDrawerSection>
                <SideDrawerSectionHeader
                  title={t('Reward type')}
                  description={t('Choose what each recipient receives.')}
                  icon={<Gift />}
                  iconTone='chart-3'
                />
                <div
                  className={`bg-muted grid rounded-md p-1 ${balanceEnabled && subscriptionEnabled ? 'grid-cols-2' : 'grid-cols-1'}`}
                  role='group'
                  aria-label={t('Reward type')}
                >
                  {balanceEnabled ? (
                    <Button
                      type='button'
                      variant={rewardType === 'balance' ? 'secondary' : 'ghost'}
                      aria-pressed={rewardType === 'balance'}
                      onClick={() => setRewardType('balance')}
                    >
                      {t('Balance')}
                    </Button>
                  ) : null}
                  {subscriptionEnabled ? (
                    <Button
                      type='button'
                      variant={
                        rewardType === 'subscription' ? 'secondary' : 'ghost'
                      }
                      aria-pressed={rewardType === 'subscription'}
                      onClick={() => setRewardType('subscription')}
                    >
                      {t('Subscription')}
                    </Button>
                  ) : null}
                </div>
              </SideDrawerSection>

              {rewardType === 'balance' ? (
                <SideDrawerSection>
                  <SideDrawerSectionHeader
                    title={t('Redemption mode')}
                    description={t(
                      'Choose how the reserved balance is distributed.'
                    )}
                    icon={<Gift />}
                    iconTone='chart-3'
                  />
                  <div
                    className={`bg-muted grid rounded-md p-1 ${singleEnabled && randomEnabled ? 'grid-cols-2' : 'grid-cols-1'}`}
                    role='group'
                    aria-label={t('Redemption mode')}
                  >
                    {singleEnabled ? (
                      <Button
                        type='button'
                        variant={kind === 'single' ? 'secondary' : 'ghost'}
                        aria-pressed={kind === 'single'}
                        onClick={() => {
                          setKind('single')
                          setAmount((current) =>
                            Math.max(minimumUnitAmount, current)
                          )
                        }}
                      >
                        {t('Single')}
                      </Button>
                    ) : null}
                    {randomEnabled ? (
                      <Button
                        type='button'
                        variant={kind === 'random' ? 'secondary' : 'ghost'}
                        aria-pressed={kind === 'random'}
                        onClick={() => {
                          setKind('random')
                          setAmount((current) =>
                            Math.max(
                              getMinimumRedemptionAmount(
                                'random',
                                maxClaims,
                                minimumUnitAmount
                              ),
                              current
                            )
                          )
                        }}
                      >
                        {t('Red packet')}
                      </Button>
                    ) : null}
                  </div>
                  <div className='space-y-2'>
                    <Label htmlFor='user-redemption-amount'>
                      {t('Amount')}
                    </Label>
                    <Input
                      id='user-redemption-amount'
                      type='number'
                      min={minimumAmount}
                      step={0.01}
                      value={amount}
                      onChange={(event) =>
                        setAmount(
                          sanitizeRedemptionAmountInput(
                            Number(event.target.value)
                          )
                        )
                      }
                    />
                    <p className='text-muted-foreground text-xs'>
                      {t('Amount uses the same display unit as your wallet.')}
                    </p>
                    {amountValidation ? (
                      <p className='text-destructive text-xs' role='alert'>
                        {t(amountValidation.key, amountValidation.values)}
                      </p>
                    ) : null}
                  </div>
                  {kind === 'random' ? (
                    <div className='space-y-2'>
                      <Label htmlFor='user-redemption-max-claims'>
                        {t('Recipients')}
                      </Label>
                      <Input
                        id='user-redemption-max-claims'
                        type='number'
                        min={2}
                        max={maxClaimsPerCode}
                        value={maxClaims}
                        onChange={(event) => {
                          const nextClaims = Number(event.target.value)
                          setMaxClaims(
                            Math.min(Math.max(nextClaims, 2), maxClaimsPerCode)
                          )
                          setAmount((current) =>
                            normalizeRedemptionAmount(
                              'random',
                              current,
                              nextClaims,
                              minimumUnitAmount,
                              maxClaimsPerCode
                            )
                          )
                        }}
                      />
                    </div>
                  ) : null}
                </SideDrawerSection>
              ) : (
                <SideDrawerSection>
                  <SideDrawerSectionHeader
                    title={t('Subscription package')}
                    description={t(
                      'Each recipient receives one independent subscription.'
                    )}
                    icon={<Layers3 />}
                    iconTone='chart-4'
                  />
                  <div className='space-y-2'>
                    <Label>{t('Plan')}</Label>
                    <Select
                      items={eligiblePlans.map(({ plan }) => ({
                        value: String(plan.id),
                        label: plan.title,
                      }))}
                      value={planId ? String(planId) : null}
                      onValueChange={(value) => setPlanId(Number(value) || 0)}
                    >
                      <SelectTrigger
                        className='h-9 w-full'
                        aria-label={t('Plan')}
                      >
                        <SelectValue
                          placeholder={t('Select a subscription plan')}
                        />
                      </SelectTrigger>
                      <SelectContent alignItemWithTrigger={false}>
                        <SelectGroup>
                          {eligiblePlans.map(({ plan }) => (
                            <SelectItem key={plan.id} value={String(plan.id)}>
                              <span className='flex min-w-0 flex-col'>
                                <span className='truncate'>{plan.title}</span>
                                <span className='text-muted-foreground text-xs'>
                                  {formatSubscriptionPrice(plan.price_amount)}
                                </span>
                              </span>
                            </SelectItem>
                          ))}
                        </SelectGroup>
                      </SelectContent>
                    </Select>
                    {plansQuery.isError ? (
                      <p className='text-destructive text-xs'>
                        {t('Unable to load subscription plans')}
                      </p>
                    ) : null}
                    {!plansQuery.isLoading &&
                    !plansQuery.isError &&
                    eligiblePlans.length === 0 ? (
                      <p className='text-muted-foreground text-xs'>
                        {t(
                          'No balance-purchasable subscription plans are available.'
                        )}
                      </p>
                    ) : null}
                  </div>
                  <div className='space-y-2'>
                    <Label htmlFor='user-redemption-units'>{t('Units')}</Label>
                    <Input
                      id='user-redemption-units'
                      type='number'
                      min={1}
                      max={maxClaimsPerCode}
                      step={1}
                      value={units}
                      onChange={(event) =>
                        setUnits(
                          Math.min(
                            Math.max(
                              Math.trunc(Number(event.target.value)) || 1,
                              1
                            ),
                            maxClaimsPerCode
                          )
                        )
                      }
                    />
                  </div>
                  {selectedPlan ? (
                    <div className='bg-muted/35 grid grid-cols-2 gap-3 rounded-md border px-3 py-3 text-sm'>
                      <div>
                        <div className='text-muted-foreground text-xs'>
                          {t('Per unit')}
                        </div>
                        <div className='mt-1 font-semibold tabular-nums'>
                          {formatQuota(subscriptionCost.unitQuota)}
                        </div>
                      </div>
                      <div>
                        <div className='text-muted-foreground text-xs'>
                          {t('Total cost')}
                        </div>
                        <div className='mt-1 font-semibold tabular-nums'>
                          {formatQuota(subscriptionCost.totalQuota)}
                        </div>
                      </div>
                    </div>
                  ) : null}
                </SideDrawerSection>
              )}

              <SideDrawerSection>
                <div className='space-y-2'>
                  <Label>{t('Validity')}</Label>
                  <Select
                    items={expiryOptions.map((optionHours) => {
                      const translation =
                        getUserRedemptionExpiryTranslation(optionHours)
                      return {
                        value: String(optionHours),
                        label: t(translation.key, translation.values),
                      }
                    })}
                    value={String(hours)}
                    onValueChange={(value) => setHours(Number(value) || 1)}
                  >
                    <SelectTrigger
                      className='h-9 w-full'
                      aria-label={t('Validity')}
                    >
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent alignItemWithTrigger={false}>
                      <SelectGroup>
                        {expiryOptions.map((optionHours) => {
                          const translation =
                            getUserRedemptionExpiryTranslation(optionHours)
                          return (
                            <SelectItem
                              key={optionHours}
                              value={String(optionHours)}
                            >
                              {t(translation.key, translation.values)}
                            </SelectItem>
                          )
                        })}
                      </SelectGroup>
                    </SelectContent>
                  </Select>
                </div>
              </SideDrawerSection>

              <SideDrawerSection>
                <SideDrawerSectionHeader title={t('Important')} />
                <ul className='text-muted-foreground list-disc space-y-2 pl-5 text-xs leading-5'>
                  <li>
                    {t(
                      rewardType === 'subscription'
                        ? 'The full subscription cost is reserved from your wallet immediately.'
                        : 'The full amount is reserved from your wallet immediately.'
                    )}
                  </li>
                  {rewardType === 'subscription' ? (
                    <>
                      <li>
                        {t(
                          'Each user can claim one subscription from this code.'
                        )}
                      </li>
                      <li>
                        {t(
                          'Unused subscription units are refunded automatically after expiry or cancellation.'
                        )}
                      </li>
                    </>
                  ) : (
                    <>
                      <li>
                        {t(
                          'A single code gives all remaining balance to the first recipient.'
                        )}
                      </li>
                      <li>
                        {t(
                          'Each user can claim a red packet only once; the final recipient receives the remainder.'
                        )}
                      </li>
                      <li>
                        {t(
                          'Unused balance is submitted for administrator settlement after expiry or when you cancel an active code.'
                        )}
                      </li>
                    </>
                  )}
                  <li>
                    {t(
                      'Administrators may terminate a code with or without a refund.'
                    )}
                  </li>
                  <li>
                    {t(
                      'Treat the code as a transferable credential and share it carefully.'
                    )}
                  </li>
                </ul>
              </SideDrawerSection>
            </>
          )}

          {error ? (
            <Alert variant='destructive'>
              <AlertTitle>{t('Unable to create redemption code')}</AlertTitle>
              <AlertDescription>{error}</AlertDescription>
            </Alert>
          ) : null}
        </div>

        <SheetFooter className={sideDrawerFooterClassName()}>
          <SheetClose render={<Button variant='outline' />}>
            {t('Close')}
          </SheetClose>
          {!createdCode ? (
            <Button
              type='button'
              disabled={creationDisabled}
              onClick={() => void handleCreate()}
            >
              {busy ? (
                <Loader2 className='animate-spin' aria-hidden='true' />
              ) : null}
              {t('Create')}
            </Button>
          ) : null}
        </SheetFooter>
      </SheetContent>
    </Sheet>
  )
}
