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
import { useState, useCallback } from 'react'
import { type ColumnDef } from '@tanstack/react-table'
import { CircleAlert, GitBranch, Sparkles, KeyRound } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { getUserAvatarFallback, getUserAvatarStyle } from '@/lib/avatar'
import { getBadgeStyle, stringToHslColor } from '@/lib/colors'
import { formatBillingCurrencyFromUSD } from '@/lib/currency'
import {
  formatUseTime,
  formatLogQuota,
  formatTimestampToDate,
} from '@/lib/format'
import { cn } from '@/lib/utils'
import { Avatar, AvatarFallback } from '@/components/ui/avatar'
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from '@/components/ui/popover'
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { StatusBadge, type StatusBadgeProps } from '@/components/status-badge'
import { DataTableColumnHeader } from '@/components/data-table/core/column-header'
import { LOG_TYPE_ALL_VALUE } from '../../constants'
import type { UsageLog } from '../../data/schema'
import {
  formatModelName,
  getFirstResponseTimeColor,
  getResponseTimeColor,
  getTieredBillingSummary,
  hasAnyCacheTokens,
  parseLogOther,
  isViolationFeeLog,
  renderAuditContent,
} from '../../lib/format'
import {
  parseInteractionType,
  type InteractionType,
} from '../../lib/interaction-parser'
import { parseClientSource } from '../../lib/source-parser'
import {
  isDisplayableLogType,
  isTimingLogType,
  getLogTypeConfig,
  isPerCallBilling,
} from '../../lib/utils'
import type { LogOtherData } from '../../types'
import { DetailsDialog } from '../dialogs/details-dialog'
import { ModelBadge } from '../model-badge'
import { useUsageLogsContext } from '../usage-logs-provider'

interface DetailSegment {
  text: string
  muted?: boolean
  danger?: boolean
}

function formatRatioCompact(ratio: number | undefined): string {
  if (ratio == null || !Number.isFinite(ratio)) return '-'
  return ratio % 1 === 0
    ? String(ratio)
    : ratio.toFixed(4).replace(/\.?0+$/, '')
}

function getGroupRatioText(other: LogOtherData | null): string | null {
  const userGroupRatio = other?.user_group_ratio
  if (
    userGroupRatio != null &&
    userGroupRatio !== -1 &&
    Number.isFinite(userGroupRatio)
  ) {
    return `${formatRatioCompact(userGroupRatio)}x`
  }

  const groupRatio = other?.group_ratio
  if (groupRatio != null && groupRatio !== 1 && Number.isFinite(groupRatio)) {
    return `${formatRatioCompact(groupRatio)}x`
  }

  return null
}

function splitQuotaDisplay(value: string): { prefix: string; amount: string } {
  const match = value.match(/^([^0-9+\-.,\s]+)(.+)$/)
  if (!match) return { prefix: '', amount: value }
  return { prefix: match[1], amount: match[2] }
}

function buildDetailSegments(
  log: UsageLog,
  other: LogOtherData | null,
  t: (key: string, opts?: Record<string, unknown>) => string
): DetailSegment[] {
  // Audit (type=3) and login (type=7) logs: render localized content from the
  // structured op descriptor instead of the raw (English-fallback) content.
  if (log.type === 3 || log.type === 7) {
    const text = renderAuditContent(other, t)
    return text ? [{ text }] : []
  }

  if (log.type === 6) {
    return [{ text: t('Async task refund') }]
  }

  if (log.type !== 2) return []

  const isViolation = isViolationFeeLog(other)
  if (isViolation) {
    const segments: DetailSegment[] = []
    segments.push({ text: t('Violation Fee'), danger: true })
    if (other?.violation_fee_code) {
      segments.push({
        text: other.violation_fee_code,
        muted: true,
      })
    }
    segments.push({
      text: `${t('Fee')}: ${formatLogQuota(other?.fee_quota ?? log.quota)}`,
      muted: true,
    })
    return segments
  }

  if (!other) return []

  const segments: DetailSegment[] = []

  const priceOpts = { digitsLarge: 4, digitsSmall: 6, abbreviate: false }
  const formatPrice = (price: number) =>
    `${formatBillingCurrencyFromUSD(price, priceOpts)}/M`
  const formatPriceCompact = (price: number) =>
    formatBillingCurrencyFromUSD(price, priceOpts)
  const formatPriceList = (prices: string[], showUnit: boolean) => {
    const text = prices.join(' / ')
    return showUnit ? `${text}/M` : text
  }
  const isTieredExpr = other.billing_mode === 'tiered_expr'
  const tieredSummary = getTieredBillingSummary(other)
  if (isTieredExpr) {
    if (tieredSummary) {
      const baseEntries = tieredSummary.priceEntries
        .filter((entry) => ['inputPrice', 'outputPrice'].includes(entry.field))
        .map((entry) => formatPriceCompact(entry.price))
      if (baseEntries.length > 0) {
        const tierLabel = tieredSummary.tier.label || t('Default')
        segments.push({
          text: `${tierLabel} · ${formatPriceList(baseEntries, true)}`,
        })
      }

      const cacheEntries = tieredSummary.priceEntries
        .filter((entry) =>
          ['cacheReadPrice', 'cacheCreatePrice', 'cacheCreate1hPrice'].includes(
            entry.field
          )
        )
        .map((entry) => {
          return formatPriceCompact(entry.price)
        })
      if (cacheEntries.length > 0) {
        segments.push({
          text: `${t('Cache')} ${formatPriceList(cacheEntries, false)}`,
          muted: true,
        })
      }

      const otherEntries = tieredSummary.priceEntries
        .filter(
          (entry) =>
            ![
              'inputPrice',
              'outputPrice',
              'cacheReadPrice',
              'cacheCreatePrice',
              'cacheCreate1hPrice',
            ].includes(entry.field)
        )
        .map((entry) => `${t(entry.shortLabel)} ${formatPrice(entry.price)}`)
      if (otherEntries.length > 0) {
        segments.push({
          text: otherEntries.join(' · '),
          muted: true,
        })
      }
    } else {
      segments.push({
        text: `${t('Dynamic Pricing')} · ${t('No matching results')}`,
        muted: true,
      })
    }
  } else {
    const isPerCall = isPerCallBilling(other.model_price)
    if (isPerCall) {
      segments.push({
        text: `${t('Per-call')} · ${formatBillingCurrencyFromUSD(other.model_price!, priceOpts)}`,
      })
    } else if (other.model_ratio != null) {
      const inputPriceUSD = other.model_ratio * 2.0
      const baseEntries = [formatPriceCompact(inputPriceUSD)]
      if (other.completion_ratio != null) {
        baseEntries.push(
          formatPriceCompact(inputPriceUSD * other.completion_ratio)
        )
      }
      segments.push({
        text: `${t('Standard')} · ${formatPriceList(baseEntries, true)}`,
      })

      if (hasAnyCacheTokens(other)) {
        const cacheEntries = [
          other.cache_ratio != null && other.cache_ratio !== 1
            ? formatPriceCompact(inputPriceUSD * other.cache_ratio)
            : null,
          other.cache_creation_ratio != null && other.cache_creation_ratio !== 1
            ? formatPriceCompact(inputPriceUSD * other.cache_creation_ratio)
            : null,
          other.cache_creation_ratio_1h != null &&
          other.cache_creation_ratio_1h !== 0
            ? formatPriceCompact(inputPriceUSD * other.cache_creation_ratio_1h)
            : null,
        ].filter(Boolean) as string[]

        if (cacheEntries.length > 0) {
          segments.push({
            text: `${t('Cache')} ${formatPriceList(cacheEntries, false)}`,
            muted: true,
          })
        }
      }
    } else {
      const userGroupRatio = other.user_group_ratio
      const groupRatio = other.group_ratio
      const isUserGroup =
        userGroupRatio != null &&
        Number.isFinite(userGroupRatio) &&
        userGroupRatio !== -1
      const effectiveRatio = isUserGroup ? userGroupRatio : groupRatio
      const ratioLabel = isUserGroup
        ? t('User Exclusive Ratio')
        : t('Group Ratio')

      if (effectiveRatio != null && Number.isFinite(effectiveRatio)) {
        segments.push({
          text: `${ratioLabel} ${formatRatioCompact(effectiveRatio)}x`,
        })
      }
    }
  }

  if (other.is_system_prompt_overwritten) {
    segments.push({
      text: t('System Prompt Override'),
      danger: true,
    })
  }

  return segments
}

export function useCommonLogsColumns(isAdmin: boolean): ColumnDef<UsageLog>[] {
  const { t } = useTranslation()
  const columns: ColumnDef<UsageLog>[] = [
    {
      accessorKey: 'created_at',
      header: t('Time'),
      cell: ({ row }) => {
        const log = row.original
        const timestamp = row.getValue('created_at') as number
        const config = getLogTypeConfig(log.type)

        return (
          <div className='flex min-w-0 flex-col gap-0.5'>
            <span className='truncate font-mono text-xs tabular-nums'>
              {formatTimestampToDate(timestamp)}
            </span>
            <StatusBadge
              label={t(config.label)}
              variant={config.color as StatusBadgeProps['variant']}
              size='sm'
              copyable={false}
              className='!text-xs [&_span]:!text-xs'
            />
          </div>
        )
      },
      filterFn: (row, _id, value) => {
        if (!Array.isArray(value) || value.length === 0) return true
        if (value.includes(LOG_TYPE_ALL_VALUE)) return true
        return value.includes(String(row.original.type))
      },
      enableHiding: false,
      size: 180,
    },
  ]

  if (isAdmin) {
    columns.push(
      {
        id: 'channel',
        header: t('Channel'),
        accessorFn: (row) => row.channel,
        cell: function ChannelCell({ row }) {
          const { sensitiveVisible, setAffinityTarget, setAffinityDialogOpen } =
            useUsageLogsContext()
          const log = row.original

          if (!isDisplayableLogType(log.type)) return null

          const other = parseLogOther(log.other)
          const affinity = other?.admin_info?.channel_affinity
          const rawUseChannel = other?.admin_info?.use_channel ?? []
          const useChannel = Array.isArray(rawUseChannel)
            ? rawUseChannel.map(String).filter(Boolean)
            : []
          const hasRetryChain = useChannel.length > 1
          const channelChain =
            hasRetryChain ? useChannel.join(' → ') : undefined
          const channelDisplay = log.channel_name
            ? `${log.channel_name} #${log.channel}`
            : `#${log.channel}`
          const channelIdDisplay = `#${log.channel}`
          const channelName = sensitiveVisible ? log.channel_name : '••••'
          const multiKeyIndex = other?.admin_info?.multi_key_index
          const showMultiKeyIndex =
            other?.admin_info?.is_multi_key === true &&
            typeof multiKeyIndex === 'number' &&
            Number.isFinite(multiKeyIndex)

          return (
            <TooltipProvider>
              <Tooltip>
                <TooltipTrigger
                  render={
                    <div className='flex max-w-[130px] flex-col gap-0.5' />
                  }
                >
                  <div className='relative inline-flex w-fit items-center gap-1'>
                    <StatusBadge
                      label={channelIdDisplay}
                      autoColor={String(log.channel)}
                      copyText={String(log.channel)}
                      size='sm'
                      showDot={false}
                      className='font-mono'
                    />
                    {showMultiKeyIndex && (
                      <StatusBadge
                        label={String(multiKeyIndex)}
                        size='sm'
                        showDot={false}
                        copyable={false}
                        variant='neutral'
                        className='h-5 min-w-5 justify-center rounded-full px-1 font-mono text-xs'
                        aria-label={`${t('Key')} ${multiKeyIndex}`}
                      />
                    )}
                    {hasRetryChain && (
                      <Popover>
                        <PopoverTrigger
                          render={
                            <button
                              type='button'
                              className='text-muted-foreground hover:text-foreground focus-visible:ring-ring inline-flex size-5 shrink-0 items-center justify-center rounded-full transition-colors focus-visible:ring-2 focus-visible:outline-none'
                              aria-label={t('Retry Chain')}
                              onClick={(e) => e.stopPropagation()}
                            />
                          }
                        >
                          <GitBranch
                            className='size-3.5 text-amber-500'
                            aria-hidden='true'
                          />
                        </PopoverTrigger>
                        <PopoverContent
                          side='top'
                          align='start'
                          className='w-64 text-xs'
                        >
                          <div className='flex flex-col gap-1'>
                            <p className='font-medium'>{t('Retry Chain')}</p>
                            <p className='text-muted-foreground font-mono break-all'>
                              {channelChain}
                            </p>
                          </div>
                        </PopoverContent>
                      </Popover>
                    )}
                    {affinity && (
                      <button
                        type='button'
                        className='absolute -top-1 -right-1 leading-none text-amber-500'
                        onClick={(e) => {
                          e.stopPropagation()
                          setAffinityTarget({
                            rule_name: affinity.rule_name || '',
                            using_group:
                              affinity.using_group ||
                              affinity.selected_group ||
                              '',
                            key_hint: affinity.key_hint || '',
                            key_fp: affinity.key_fp || '',
                          })
                          setAffinityDialogOpen(true)
                        }}
                      >
                        <Sparkles className='size-3 fill-current' />
                      </button>
                    )}
                  </div>
                  {log.channel_name && (
                    <span className='text-muted-foreground/70 truncate [font-family:var(--font-body)] !text-xs'>
                      {channelName}
                    </span>
                  )}
                </TooltipTrigger>
                <TooltipContent>
                  <div className='space-y-1'>
                    <p>
                      {sensitiveVisible ? channelDisplay : channelIdDisplay}
                    </p>
                    {channelChain && (
                      <p className='text-muted-foreground text-xs'>
                        {t('Chain')}: {channelChain}
                      </p>
                    )}
                    {showMultiKeyIndex && (
                      <p className='text-muted-foreground text-xs'>
                        {t('Key')}: {multiKeyIndex}
                      </p>
                    )}
                    {affinity && (
                      <div className='border-t pt-1 text-xs'>
                        <p className='font-medium'>{t('Channel Affinity')}</p>
                        <p>
                          {t('Rule')}: {affinity.rule_name || '-'}
                        </p>
                        <p>
                          {t('Group')}:{' '}
                          {sensitiveVisible
                            ? affinity.using_group ||
                              affinity.selected_group ||
                              '-'
                            : '••••'}
                        </p>
                      </div>
                    )}
                  </div>
                </TooltipContent>
              </Tooltip>
            </TooltipProvider>
          )
        },
        size: 130,
      },
      {
        id: 'user',
        header: t('User'),
        accessorFn: (row) => row.username,
        cell: function UserCell({ row }) {
          const { sensitiveVisible, setSelectedUserId, setUserInfoDialogOpen } =
            useUsageLogsContext()
          const log = row.original

          if (!log.username) return null

          return (
            <button
              type='button'
              className='flex items-center gap-1.5 text-left'
              onClick={(e) => {
                e.stopPropagation()
                setSelectedUserId(log.user_id)
                setUserInfoDialogOpen(true)
              }}
            >
              <Avatar className='ring-border/60 size-6 ring-1 max-sm:hidden'>
                <AvatarFallback
                  className={cn(
                    'text-[11px] font-semibold',
                    !sensitiveVisible && 'bg-muted text-muted-foreground'
                  )}
                  style={
                    sensitiveVisible
                      ? getUserAvatarStyle(log.username)
                      : undefined
                  }
                >
                  {sensitiveVisible ? getUserAvatarFallback(log.username) : '•'}
                </AvatarFallback>
              </Avatar>
              <TooltipProvider delay={300}>
                <Tooltip>
                  <TooltipTrigger
                    render={
                      <span className='text-muted-foreground max-w-[100px] truncate text-sm hover:underline' />
                    }
                  >
                    {sensitiveVisible ? log.username : '••••'}
                  </TooltipTrigger>
                  {sensitiveVisible && log.username.length > 12 && (
                    <TooltipContent side='top'>{log.username}</TooltipContent>
                  )}
                </Tooltip>
              </TooltipProvider>
            </button>
          )
        },
        size: 120,
      }
    )
  }

  columns.push({
    accessorKey: 'token_name',
    header: t('Token'),
    cell: function TokenNameCell({ row }) {
      const { sensitiveVisible } = useUsageLogsContext()
      const log = row.original
      if (!isDisplayableLogType(log.type)) return null

      const tokenName = log.token_name
      if (!tokenName) return null

      const other = parseLogOther(log.other)
      const displayName = sensitiveVisible ? tokenName : '••••'
      let group = log.group
      if (!group) group = other?.group || ''

      const metaParts: string[] = []
      const groupRatioText = getGroupRatioText(other)
      if (group) {
        metaParts.push(sensitiveVisible ? group : '••••')
      }
      if (groupRatioText) metaParts.push(groupRatioText)

      return (
        <div className='flex max-w-[200px] flex-col gap-0.5'>
          <TooltipProvider delay={300}>
            <Tooltip>
              <TooltipTrigger render={<div className='max-w-full' />}>
                <StatusBadge
                  label={displayName}
                  icon={KeyRound}
                  copyText={sensitiveVisible ? tokenName : undefined}
                  size='sm'
                  showDot={false}
                  className='border-border/60 bg-muted/30 text-foreground h-6 max-w-full gap-1.5 overflow-hidden rounded-md border px-2 py-0.5 [font-family:var(--font-body)]'
                />
              </TooltipTrigger>
              {sensitiveVisible && tokenName.length > 16 && (
                <TooltipContent side='top' className='max-w-xs break-all'>
                  {tokenName}
                </TooltipContent>
              )}
            </Tooltip>
          </TooltipProvider>
          {metaParts.length > 0 && (
            <span className='text-muted-foreground/60 truncate [font-family:var(--font-body)] !text-xs'>
              {metaParts.join(' · ')}
            </span>
          )}
        </div>
      )
    },
    size: 160,
  })
  columns.push(
    {
      accessorKey: 'model_name',
      header: t('Model'),
      cell: function ModelCell({ row }) {
        const log = row.original
        if (!isDisplayableLogType(log.type)) return null

        const modelInfo = formatModelName(log)

        return (
          <div className='flex w-fit flex-col gap-0.5'>
            <ModelBadge
              modelName={modelInfo.name}
              actualModel={modelInfo.actualModel}
            />
          </div>
        )
      },
      meta: { mobileTitle: true },
      size: 160,
    },
    {
      id: 'source',
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title={t('Source')} />
      ),
      cell: ({ row }) => {
        const log = row.original
        if (!isDisplayableLogType(log.type)) return null

        try {
          const other = parseLogOther(log.other)
          if (other?.client_source && typeof other.client_source === 'string') {
            const color = stringToHslColor(other.client_source)
            return (
              <div className='flex justify-center'>
                <span
                  className='inline-flex items-center justify-center rounded-full px-2 py-0.5 text-center text-xs font-medium'
                  style={{
                    backgroundColor: `color-mix(in srgb, ${color} 15%, transparent)`,
                    color: color,
                  }}
                >
                  {other.client_source}
                </span>
              </div>
            )
          }

          const recordData =
            typeof log.content === 'string'
              ? JSON.parse(log.content)
              : log.content
          const headers =
            recordData?.request?.headers || recordData?.headers || {}
          const uaKey = Object.keys(headers).find(
            (k) => k.toLowerCase() === 'user-agent'
          )
          const userAgent = uaKey ? headers[uaKey] : ''
          const source = parseClientSource(userAgent)

          if (source.name === '-') return null
          const color = stringToHslColor(source.name)
          return (
            <div className='flex justify-center'>
              <span
                className='inline-flex items-center justify-center rounded-full px-2 py-0.5 text-center text-xs font-medium'
                style={{
                  backgroundColor: `color-mix(in srgb, ${color} 15%, transparent)`,
                  color: color,
                }}
              >
                {source.name}
              </span>
            </div>
          )
        } catch {
          return null
        }
      },
      meta: { label: t('Source'), mobileHidden: true },
      size: 100,
    },

    {
      id: 'session',
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title={t('Session')} />
      ),
      cell: ({ row }) => {
        const log = row.original
        if (!isDisplayableLogType(log.type)) return null

        const other = parseLogOther(log.other)
        if (
          !other?.session_name &&
          !other?.agent_name &&
          !other?.parent_session_id
        )
          return null

        // When parent_session_id exists, the parent is the main conversation
        // and the current session is the sub-agent/child session.
        // When no parent, session_name is the main conversation and
        // agent_name is the sub-agent.
        const hasParent = !!other.parent_session_name
        const mainSessionName = hasParent
          ? other.parent_session_name
          : other.session_name
        const subSessionName = hasParent ? other.session_name : null
        const subAgentName = other.agent_name

        return (
          <div className='flex flex-col items-start gap-0.5'>
            {mainSessionName &&
              (() => {
                const badge = getBadgeStyle(`session-${mainSessionName}`)
                return (
                  <span
                    className={`inline-flex items-center justify-center rounded-full px-2 py-0.5 text-xs font-medium ${badge.bg} ${badge.text}`}
                  >
                    {mainSessionName}
                  </span>
                )
              })()}
            {subSessionName && (
              <span className='text-muted-foreground/60 truncate pl-2 text-[11px]'>
                ↳ {subSessionName}
              </span>
            )}
            {subAgentName && (
              <span className='text-muted-foreground/60 truncate pl-2 text-[11px]'>
                ↳ {subAgentName}
              </span>
            )}
          </div>
        )
      },
      meta: { label: t('Session'), mobileHidden: true },
      size: 120,
    },

    {
      id: 'interaction_type',
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title={t('Interaction')} />
      ),
      cell: ({ row }) => {
        const log = row.original
        if (!isDisplayableLogType(log.type)) return null

        const other = parseLogOther(log.other)
        const normalizeInteractionType = (
          value: unknown
        ): InteractionType | undefined => {
          if (value === 'input' || value === '输入') return 'input'
          if (value === 'output' || value === '输出') return 'output'
          if (value === 'callback' || value === '回调') return 'callback'
          return undefined
        }
        const precomputed = normalizeInteractionType(other?.interaction_type)
        const interactionType = precomputed || parseInteractionType(log.content)
        if (!interactionType) return null

        const labelMap: Record<string, string> = {
          input: t('Input'),
          output: t('Output'),
          callback: t('Callback'),
        }

        const badge = getBadgeStyle(`interaction-type-${interactionType}`)

        return (
          <div className='flex justify-center'>
            <span
              className={`inline-flex items-center justify-center rounded-full px-2 py-0.5 text-center text-xs font-medium ${badge.bg} ${badge.text}`}
            >
              {labelMap[interactionType] || interactionType}
            </span>
          </div>
        )
      },
      meta: { label: t('Interaction'), mobileHidden: true },
      size: 90,
    },

    {
      id: 'tps',
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title={t('TPS')} />
      ),
      cell: ({ row }) => {
        const log = row.original
        if (!isDisplayableLogType(log.type)) return null

        const other = parseLogOther(log.other)
        const tps = other?.tps
        const hasValidTps = typeof tps === 'number' && tps > 0

        const bt = other?.bamboo_timing
        const thinkingTps =
          typeof bt?.thinking_tps === 'number' && bt.thinking_tps > 0
            ? bt.thinking_tps
            : null
        const outputTps =
          typeof bt?.output_tps === 'number' && bt.output_tps > 0
            ? bt.output_tps
            : null
        const toolTps =
          typeof bt?.tool_tps === 'number' && bt.tool_tps > 0
            ? bt.tool_tps
            : null
        const hasBambooRates =
          thinkingTps != null || outputTps != null || toolTps != null

        const useTime = log.use_time
        const ttftSec = (() => {
          const btTtft = bt?.ttft_ms
          if (typeof btTtft === 'number' && btTtft > 0) return btTtft / 1000
          const frtVal = other?.frt
          if (typeof frtVal === 'number' && frtVal > 0) return frtVal / 1000
          return 0
        })()

        const completionTokens =
          log.completion_tokens > 0
            ? log.completion_tokens
            : typeof bt?.output_tokens === 'number' && bt.output_tokens > 0
              ? bt.output_tokens
              : 0

        const genTime =
          log.is_stream && ttftSec > 0 ? useTime - ttftSec : useTime
        const avgTps =
          genTime > 0 && completionTokens > 0
            ? completionTokens / genTime
            : null

        if (!hasValidTps && avgTps == null && !hasBambooRates) return null

        const displayTps = hasValidTps ? (tps as number) : avgTps

        let colorClass = 'text-red-600 dark:text-red-400'
        if (displayTps != null) {
          if (displayTps >= 81)
            colorClass = 'text-green-600 dark:text-green-400'
          else if (displayTps >= 51)
            colorClass = 'text-lime-600 dark:text-lime-400'
          else if (displayTps >= 11)
            colorClass = 'text-yellow-600 dark:text-yellow-400'
        }

        return (
          <div className='flex flex-col gap-0.5'>
            {displayTps != null && (
              <span
                className={`font-mono text-sm font-medium tabular-nums ${colorClass}`}
              >
                {displayTps.toFixed(1)}
              </span>
            )}
            {hasBambooRates ? (
              <div className='flex items-center gap-2 text-[11px]'>
                {thinkingTps != null && (
                  <span className='flex items-center gap-0.5'>
                    <span className='text-violet-500/80 dark:text-violet-400/70'>
                      ◆
                    </span>
                    <span className='font-mono tabular-nums text-violet-600/80 dark:text-violet-400/70'>
                      {thinkingTps.toFixed(1)}
                    </span>
                  </span>
                )}
                {outputTps != null && (
                  <span className='flex items-center gap-0.5'>
                    <span className='text-sky-500/80 dark:text-sky-400/70'>
                      ◆
                    </span>
                    <span className='font-mono tabular-nums text-sky-600/80 dark:text-sky-400/70'>
                      {outputTps.toFixed(1)}
                    </span>
                  </span>
                )}
                {toolTps != null && (
                  <span className='flex items-center gap-0.5'>
                    <span className='text-amber-500/80 dark:text-amber-400/70'>
                      ◆
                    </span>
                    <span className='font-mono tabular-nums text-amber-600/80 dark:text-amber-400/70'>
                      {toolTps.toFixed(1)}
                    </span>
                  </span>
                )}
              </div>
            ) : avgTps != null ? (
              <span className='text-muted-foreground/60 font-mono text-[11px] tabular-nums'>
                {Math.round(avgTps)}
              </span>
            ) : null}
          </div>
        )
      },
      meta: { label: t('TPS'), mobileHidden: true },
      size: 80,
    },

    {
      accessorKey: 'use_time',
      header: t('Timing'),
      cell: ({ row }) => {
        const log = row.original
        if (!isTimingLogType(log.type)) return null

        const useTime = row.getValue('use_time') as number
        const other = parseLogOther(log.other)
        const frt = other?.frt
        const timeVariant = getResponseTimeColor(useTime, log.completion_tokens)

        const timingBgMap: Record<string, string> = {
          success:
            'border border-emerald-200/40 bg-emerald-50/35 !text-emerald-600 dark:border-emerald-900/40 dark:bg-emerald-950/15 dark:!text-emerald-400',
          warning:
            'border border-amber-200/45 bg-amber-50/35 !text-amber-600 dark:border-amber-900/40 dark:bg-amber-950/15 dark:!text-amber-400',
          danger:
            'border border-rose-200/50 bg-rose-50/35 !text-red-600 dark:border-rose-900/40 dark:bg-rose-950/15 dark:!text-red-400',
          neutral:
            'border border-border/60 bg-muted/30 dark:border-border/40 dark:bg-muted/20',
        }

        const bt = other?.bamboo_timing
        const ttftMs =
          typeof bt?.ttft_ms === 'number' && bt.ttft_ms > 0
            ? bt.ttft_ms
            : typeof frt === 'number'
              ? frt
              : null

        const thinkingMs =
          typeof bt?.thinking_ms === 'number' && bt.thinking_ms > 0
            ? bt.thinking_ms
            : null
        const contentMs =
          typeof bt?.content_ms === 'number' && bt.content_ms > 0
            ? bt.content_ms
            : null
        const toolMs =
          typeof bt?.tool_ms === 'number' && bt.tool_ms > 0
            ? bt.tool_ms
            : null
        const hasPhaseTiming =
          thinkingMs != null || contentMs != null || toolMs != null

        const ttftBadge = log.is_stream ? (
          ttftMs != null && ttftMs > 0 ? (
            <StatusBadge
              label={formatUseTime(ttftMs / 1000)}
              variant={
                getFirstResponseTimeColor(ttftMs / 1000) as StatusBadgeProps['variant']
              }
              size='sm'
              showDot={false}
              copyable={false}
              className={cn(
                'rounded-md font-mono',
                timingBgMap[getFirstResponseTimeColor(ttftMs / 1000)]
              )}
            />
          ) : (
            <StatusBadge
              label='N/A'
              variant='neutral'
              size='sm'
              showDot={false}
              copyable={false}
              className={cn('rounded-md font-mono', timingBgMap.neutral)}
            />
          )
        ) : null

        const totalBadge = (
          <StatusBadge
            label={formatUseTime(useTime)}
            variant={timeVariant as StatusBadgeProps['variant']}
            size='sm'
            copyable={false}
            className={cn('rounded-md font-mono', timingBgMap[timeVariant])}
          />
        )

        return (
          <div className='flex flex-col gap-1'>
            {hasPhaseTiming ? (
              <TooltipProvider>
                <Tooltip>
                  <TooltipTrigger
                    render={
                      <div className='flex items-center gap-1.5'>
                        {totalBadge}
                        {ttftBadge}
                      </div>
                    }
                  ></TooltipTrigger>
                  <TooltipContent
                    side='bottom'
                    className='max-w-[220px] p-2'
                  >
                    <div className='space-y-1 text-xs'>
                      {thinkingMs != null && (
                        <div className='flex items-center gap-1.5'>
                          <span className='text-violet-500 dark:text-violet-400'>
                            ◆
                          </span>
                          <span className='text-muted-foreground'>
                            {t('Thinking')}
                          </span>
                          <span className='ml-auto font-mono tabular-nums text-violet-600 dark:text-violet-400'>
                            {(thinkingMs / 1000).toFixed(2)}s
                          </span>
                        </div>
                      )}
                      {contentMs != null && (
                        <div className='flex items-center gap-1.5'>
                          <span className='text-sky-500 dark:text-sky-400'>
                            ◆
                          </span>
                          <span className='text-muted-foreground'>
                            {t('Output')}
                          </span>
                          <span className='ml-auto font-mono tabular-nums text-sky-600 dark:text-sky-400'>
                            {(contentMs / 1000).toFixed(2)}s
                          </span>
                        </div>
                      )}
                      {toolMs != null && (
                        <div className='flex items-center gap-1.5'>
                          <span className='text-amber-500 dark:text-amber-400'>
                            ◆
                          </span>
                          <span className='text-muted-foreground'>
                            {t('Tool')}
                          </span>
                          <span className='ml-auto font-mono tabular-nums text-amber-600 dark:text-amber-400'>
                            {(toolMs / 1000).toFixed(2)}s
                          </span>
                        </div>
                      )}
                    </div>
                  </TooltipContent>
                </Tooltip>
              </TooltipProvider>
            ) : (
              <div className='flex items-center gap-1.5'>
                {totalBadge}
                {ttftBadge}
              </div>
            )}
            {log.is_stream &&
              other?.stream_status &&
              other.stream_status.status !== 'ok' && (
                <div className='flex items-center gap-1 [font-family:var(--font-body)] !text-xs leading-none'>
                  <TooltipProvider>
                    <Tooltip>
                      <TooltipTrigger
                        render={
                          <CircleAlert className='size-3 shrink-0 text-red-500' />
                        }
                      ></TooltipTrigger>
                      <TooltipContent>
                        <div className='space-y-0.5 text-xs'>
                          <p>
                            {t('Stream Status')}: {t('Error')}
                          </p>
                          <p>{other.stream_status.end_reason || 'unknown'}</p>
                          {(other.stream_status.error_count ?? 0) > 0 && (
                            <p>
                              {t('Soft Errors')}:{' '}
                              {other.stream_status.error_count}
                            </p>
                          )}
                        </div>
                      </TooltipContent>
                    </Tooltip>
                  </TooltipProvider>
                </div>
              )}
          </div>
        )
      },
      size: 110,
    },

    {
      accessorKey: 'prompt_tokens',
      header: 'Tokens',
      cell: ({ row }) => {
        const log = row.original
        if (!isDisplayableLogType(log.type)) return null

        let promptTokens = log.prompt_tokens || 0
        let completionTokens = log.completion_tokens || 0

        // Fallback to bamboo_timing token counts when log fields are 0
        if (promptTokens === 0 || completionTokens === 0) {
          const other = parseLogOther(log.other)
          const bt = other?.bamboo_timing
          if (promptTokens === 0) {
            const thinkingTokens =
              typeof bt?.thinking_tokens === 'number'
                ? bt.thinking_tokens
                : 0
            const outputTokens =
              typeof bt?.output_tokens === 'number' ? bt.output_tokens : 0
            const toolTokens =
              typeof bt?.tool_tokens === 'number' ? bt.tool_tokens : 0
            const bambooTotal =
              thinkingTokens + outputTokens + toolTokens
            if (bambooTotal > 0 && completionTokens === 0) {
              completionTokens = outputTokens + toolTokens
            }
          }
        }

        if (promptTokens === 0 && completionTokens === 0) {
          return <span className='text-muted-foreground text-xs'>-</span>
        }

        return (
          <div className='flex flex-col gap-0.5'>
            <span className='font-mono text-xs font-medium tabular-nums'>
              {promptTokens.toLocaleString()} /{' '}
              {completionTokens.toLocaleString()}
            </span>
          </div>
        )
      },
      size: 110,
    },

    {
      id: 'cache_rate',
      header: t('Cache Rate'),
      cell: ({ row }) => {
        const log = row.original
        if (!isDisplayableLogType(log.type)) return null

        const other = parseLogOther(log.other)

        const cacheReadTokens = other?.cache_tokens || 0
        const cacheWrite5m = other?.cache_creation_tokens_5m || 0
        const cacheWrite1h = other?.cache_creation_tokens_1h || 0
        const hasSplitCache = cacheWrite5m > 0 || cacheWrite1h > 0
        const cacheWriteTokens = hasSplitCache
          ? cacheWrite5m + cacheWrite1h
          : other?.cache_creation_tokens || 0

        const promptTokens = log.prompt_tokens || 0

        // prompt_tokens semantics differ by provider:
        // - Claude (anthropic semantic): prompt_tokens is text-only, EXCLUDES cache tokens
        // - OpenAI (openai semantic): prompt_tokens is the TOTAL, INCLUDES cache_read tokens
        // input_tokens_total (written by backend) is the authoritative denominator;
        // fallback must respect the claude flag to avoid double-counting cache tokens.
        const isClaudeSemantic = other?.claude === true
        const totalInput =
          other?.input_tokens_total && other.input_tokens_total > 0
            ? other.input_tokens_total
            : isClaudeSemantic
              ? promptTokens + cacheReadTokens + cacheWriteTokens
              : promptTokens

        const cacheRate =
          totalInput > 0 && cacheReadTokens > 0
            ? Math.min((cacheReadTokens / totalInput) * 100, 100)
            : null

        if (cacheRate === null || cacheRate === 0) {
          return <span className='text-muted-foreground text-xs'>-</span>
        }

        const rateText = `${cacheRate.toFixed(1)}%`

        return (
          <TooltipProvider>
            <Tooltip>
              <TooltipTrigger
                render={
                  <span className='font-mono text-xs font-medium tabular-nums text-emerald-600 dark:text-emerald-400'>
                    {rateText}
                  </span>
                }
              />
              <TooltipContent side='top' className='max-w-[200px] p-2'>
                <div className='flex flex-col gap-0.5 text-xs'>
                  {cacheReadTokens > 0 && (
                    <div className='flex items-center justify-between gap-3'>
                      <span className='text-muted-foreground'>
                        {t('Cache Read')}
                      </span>
                      <span className='font-mono tabular-nums font-medium'>
                        ↓ {cacheReadTokens.toLocaleString()}
                      </span>
                    </div>
                  )}
                  {cacheWriteTokens > 0 && (
                    <div className='flex items-center justify-between gap-3'>
                      <span className='text-muted-foreground'>
                        {t('Cache Write')}
                      </span>
                      <span className='font-mono tabular-nums font-medium'>
                        ↑ {cacheWriteTokens.toLocaleString()}
                      </span>
                    </div>
                  )}
                  {hasSplitCache && (
                    <>
                      {cacheWrite5m > 0 && (
                        <div className='flex items-center justify-between gap-3 text-[11px] text-muted-foreground/70'>
                          <span>{t('Cache Creation (5m)')}</span>
                          <span className='font-mono tabular-nums'>
                            {cacheWrite5m.toLocaleString()}
                          </span>
                        </div>
                      )}
                      {cacheWrite1h > 0 && (
                        <div className='flex items-center justify-between gap-3 text-[11px] text-muted-foreground/70'>
                          <span>{t('Cache Creation (1h)')}</span>
                          <span className='font-mono tabular-nums'>
                            {cacheWrite1h.toLocaleString()}
                          </span>
                        </div>
                      )}
                    </>
                  )}
                </div>
              </TooltipContent>
            </Tooltip>
          </TooltipProvider>
        )
      },
      size: 90,
    },

    {
      accessorKey: 'quota',
      header: t('Cost'),
      cell: ({ row }) => {
        const log = row.original
        if (!isDisplayableLogType(log.type)) return null

        const quota = row.getValue('quota') as number
        const other = parseLogOther(log.other)
        const isSubscription = other?.billing_source === 'subscription'

        if (isSubscription) {
          return (
            <TooltipProvider>
              <Tooltip>
                <TooltipTrigger
                  render={
                    <StatusBadge
                      label={t('Subscription')}
                      variant='success'
                      size='sm'
                      copyable={false}
                      className='cursor-help'
                    />
                  }
                />
                <TooltipContent>
                  <span>
                    {t('Deducted by subscription')}: {formatLogQuota(quota)}
                  </span>
                </TooltipContent>
              </Tooltip>
            </TooltipProvider>
          )
        }

        const quotaStr = formatLogQuota(quota)
        const quotaDisplay = splitQuotaDisplay(quotaStr)

        return (
          <div className='flex flex-col gap-0.5'>
            <span className='border-border/80 bg-muted/60 inline-flex h-6 w-fit items-center rounded-md border px-2 [font-family:var(--font-body)] text-sm leading-none font-semibold tabular-nums'>
              {quotaDisplay.prefix && (
                <span className='mr-1'>{quotaDisplay.prefix}</span>
              )}
              <span>{quotaDisplay.amount}</span>
            </span>
          </div>
        )
      },
      size: 90,
    },

    {
      accessorKey: 'content',
      header: t('Details'),
      cell: function DetailsCell({ row }) {
        const [dialogOpen, setDialogOpen] = useState(false)
        const { setIsDetailOpen } = useUsageLogsContext()
        const log = row.original
        const other = parseLogOther(log.other)

        const segments = buildDetailSegments(log, other, t)
        const primary = segments[0]
        const hasMore = segments.length > 1

        const handleOpenChange = useCallback(
          (open: boolean) => {
            setDialogOpen(open)
            setIsDetailOpen(open)
          },
          [setIsDetailOpen]
        )

        return (
          <>
            <button
              type='button'
              className='group flex max-w-[200px] items-center gap-1 text-left text-xs'
              onClick={() => handleOpenChange(true)}
              title={t('Click to view full details')}
            >
              {primary ? (
                <span
                  className={cn(
                    'truncate leading-snug group-hover:underline',
                    primary.muted
                      ? 'text-muted-foreground/60'
                      : primary.danger
                        ? 'text-red-600 dark:text-red-400'
                        : 'text-foreground'
                  )}
                >
                  {primary.text}
                  {hasMore && (
                    <span className='text-muted-foreground/40 ml-0.5'>
                      +{segments.length - 1}
                    </span>
                  )}
                </span>
              ) : log.content ? (
                <span className='text-muted-foreground truncate group-hover:underline'>
                  {log.content}
                </span>
              ) : (
                <span className='text-muted-foreground/40'>—</span>
              )}
            </button>
            <DetailsDialog
              log={log}
              isAdmin={isAdmin}
              open={dialogOpen}
              onOpenChange={handleOpenChange}
            />
          </>
        )
      },
      size: 180,
      maxSize: 200,
    }
  )

  return columns
}
