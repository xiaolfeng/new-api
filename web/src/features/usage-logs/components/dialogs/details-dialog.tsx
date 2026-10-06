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
import type { TFunction } from 'i18next'
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
import {
  Copy,
  Check,
  Route,
  Settings2,
  AlertTriangle,
  Headphones,
  Monitor,
  Cloud,
  Globe,
  ShieldCheck,
  UserCog,
  Info,
  LogIn,
  ArrowDownToLine,
  Brain,
  MessageSquare,
  Wrench,
  FileText,
  Code,
} from 'lucide-react'
import { useQuery } from '@tanstack/react-query'
import { useEffect, useState, useMemo } from 'react'
import { useTranslation } from 'react-i18next'

import { Dialog } from '@/components/dialog'
import { StatusBadge, type StatusBadgeProps } from '@/components/status-badge'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { MarkdownSourceHighlighter } from '@/components/ui/markdown-source-highlighter'
import { ScrollArea } from '@/components/ui/scroll-area'
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { DynamicPricingBreakdown } from '@/features/pricing/components/dynamic-pricing-breakdown'
import { usePricingData } from '@/features/pricing/hooks/use-pricing-data'
import { BILLING_PRICING_VARS } from '@/features/pricing/lib/billing-expr'
import { pluginUsageSchema } from '@/features/pricing/lib/plugin-pricing'
import { PolicyDecisionRecord } from '@/features/system-settings/request-policies/decision-record'
import { useCopyToClipboard } from '@/hooks/use-copy-to-clipboard'
import { getBadgeStyle, stringToHslColor } from '@/lib/colors'
import { formatBillingCurrencyFromUSD } from '@/lib/currency'
import { formatLogQuota, formatTokens } from '@/lib/format'
import { hasDeveloperToolLogAccess } from '@/lib/log-helpers'
import { cn } from '@/lib/utils'

import { AuditDetailFields } from '../../audit/components/audit-detail-fields'
import { getLogDetail } from '../../api'
import { useOptionalUsageLogsContext } from '../usage-logs-provider'
import type { UsageLog } from '../../data/schema'
import {
  parseLogOther,
  getParamOverrideActionLabel,
  parseAuditLine,
  decodeBillingExprB64,
  getTieredBillingSummary,
  hasAnyCacheTokens,
  isViolationFeeLog,
  getReasoningEffortVariant,
  renderAuditContent,
} from '../../lib/format'
import {
  hasStructuredData,
  parseLogDetailRecord,
  type ParsedSections,
  type ToolUseRow,
} from '../../lib/log-block-parser'
import { buildQuotaAuditOperation } from '../../lib/quota-audit-operation'
import { parseLogSession } from '../../lib/session-parser'
import { parseClientSource } from '../../lib/source-parser'
import {
  getLogTypeConfig,
  isPerCallBilling,
  isTimingLogType,
} from '../../lib/utils'
import { USAGE_BILLING_PATH, type LogOtherData } from '../../types'
import { ResponseModelDetails } from '../model-badge'
import { PluginAuthorLink } from '../plugin-author-link'
import { DetailRow, DetailSection } from './log-detail-layout'
import { LogTimingDetails } from './log-timing-details'

// Maps a channel-update changed-field token (as recorded by the backend audit)
// to its i18n label key for display in the audit details.
const CHANNEL_FIELD_LABELS: Record<string, string> = {
  status: 'Status',
  models: 'Models',
  group: 'Group',
  type: 'Type',
  base_url: 'Base URL',
  key: 'Key',
}

function formatRatio(ratio: number | undefined): string {
  if (ratio == null) return '-'
  return ratio.toFixed(4)
}

function getUsageBillingPathLabel(
  t: TFunction,
  adminInfo: LogOtherData['admin_info']
): string {
  switch (adminInfo?.usage_billing_path) {
    case USAGE_BILLING_PATH.LOCAL:
      return t('Local Billing')
    case USAGE_BILLING_PATH.OPENAI:
      return t('Upstream Response (billing-usage-openai)')
    case USAGE_BILLING_PATH.OPENAI_ESTIMATED:
      return t('Upstream Response (billing-usage-openai-estimated)')
    case USAGE_BILLING_PATH.ANTHROPIC:
      return t('Upstream Response (billing-usage-anthropic)')
    case USAGE_BILLING_PATH.ANTHROPIC_ESTIMATED:
      return t('Upstream Response (billing-usage-anthropic-estimated)')
    case USAGE_BILLING_PATH.GEMINI:
      return t('Upstream Response (billing-usage-gemini)')
    case USAGE_BILLING_PATH.GEMINI_ESTIMATED:
      return t('Upstream Response (billing-usage-gemini-estimated)')
    case USAGE_BILLING_PATH.UPSTREAM:
      return t('Upstream Response')
    default:
      return adminInfo?.local_count_tokens
        ? t('Local Billing')
        : t('Upstream Response')
  }
}

function isUsageBillingPathLocal(
  adminInfo: LogOtherData['admin_info']
): boolean {
  if (adminInfo?.usage_billing_path) {
    return adminInfo.usage_billing_path === USAGE_BILLING_PATH.LOCAL
  }
  return adminInfo?.local_count_tokens === true
}

function quotaSaturationKindLabel(
  kind: 'overflow' | 'underflow' | 'nan',
  t: (key: string) => string
): string {
  if (kind === 'overflow') return t('Overflow')
  if (kind === 'underflow') return t('Underflow')
  return t('Invalid (NaN)')
}

function BillingBreakdown(props: {
  log: UsageLog
  other: LogOtherData
  isAdmin: boolean
}) {
  const { t } = useTranslation()
  const { log, other, isAdmin } = props
  const isPerCall = isPerCallBilling(other.model_price)
  const isClaude = other.claude === true
  const isTieredExpr = other.billing_mode === 'tiered_expr'
  const tieredSummary = getTieredBillingSummary(other)

  const rows: Array<{ label: string; value: string }> = []
  const priceOpts = { digitsLarge: 4, digitsSmall: 6, abbreviate: false }
  const fmtPrice = (usd: number) => formatBillingCurrencyFromUSD(usd, priceOpts)
  const baseInputUSD = other.model_ratio != null ? other.model_ratio * 2.0 : 0

  if (isTieredExpr) {
    rows.push({
      label: t('Billing Mode'),
      value: t('Dynamic Pricing'),
    })
    if (tieredSummary) {
      if (tieredSummary.tier.label) {
        rows.push({
          label: t('Matched Tier'),
          value: tieredSummary.tier.label,
        })
      }
      for (const entry of tieredSummary.priceEntries) {
        rows.push({
          label: t(entry.shortLabel),
          value: `${fmtPrice(entry.price)}/${entry.unit ? t(entry.unit) : 'M'}`,
        })
      }
    } else {
      rows.push({
        label: t('Matched Tier'),
        value: other.matched_tier || t('No matching results'),
      })
    }
  } else if (isPerCall) {
    rows.push({ label: t('Billing Mode'), value: t('Per-call') })
    if (other.model_price != null) {
      rows.push({
        label: t('Model Price'),
        value: fmtPrice(other.model_price),
      })
    }
  } else {
    rows.push({ label: t('Billing Mode'), value: t('Per-token') })
    if (other.model_ratio != null) {
      rows.push({
        label: t('Input'),
        value: `${fmtPrice(baseInputUSD)}/M`,
      })
    }
    if (other.completion_ratio != null && other.model_ratio != null) {
      rows.push({
        label: t('Output'),
        value: `${fmtPrice(baseInputUSD * other.completion_ratio)}/M`,
      })
    }
  }

  const userGR = other.user_group_ratio
  const isUserGR = userGR != null && Number.isFinite(userGR) && userGR !== -1
  const effectiveGR = isUserGR ? userGR : other.group_ratio
  if (effectiveGR != null && Number.isFinite(effectiveGR)) {
    rows.push({
      label: isUserGR ? t('User Exclusive Ratio') : t('Group Ratio'),
      value: `${formatRatio(effectiveGR)}x`,
    })
  }

  if (!isTieredExpr && isClaude && hasAnyCacheTokens(other)) {
    if (other.cache_ratio != null && other.cache_ratio !== 1) {
      rows.push({
        label: t('Cache Read'),
        value: `${fmtPrice(baseInputUSD * other.cache_ratio)}/M`,
      })
    }
    if (
      other.cache_creation_ratio != null &&
      other.cache_creation_ratio !== 1
    ) {
      rows.push({
        label: t('Cache Creation'),
        value: `${fmtPrice(baseInputUSD * other.cache_creation_ratio)}/M`,
      })
    }
    if (
      other.cache_creation_ratio_5m != null &&
      other.cache_creation_ratio_5m !== 0
    ) {
      rows.push({
        label: t('Cache Creation (5m)'),
        value: `${fmtPrice(baseInputUSD * other.cache_creation_ratio_5m)}/M`,
      })
    }
    if (
      other.cache_creation_ratio_1h != null &&
      other.cache_creation_ratio_1h !== 0
    ) {
      rows.push({
        label: t('Cache Creation (1h)'),
        value: `${fmtPrice(baseInputUSD * other.cache_creation_ratio_1h)}/M`,
      })
    }
  }

  if (!isTieredExpr) {
    if (other.audio_ratio != null && other.audio_ratio !== 1) {
      rows.push({
        label: t('Audio input'),
        value: `${fmtPrice(baseInputUSD * other.audio_ratio)}/M`,
      })
    }

    if (
      other.audio_completion_ratio != null &&
      other.audio_completion_ratio !== 1
    ) {
      rows.push({
        label: t('Audio output'),
        value: `${fmtPrice(baseInputUSD * other.audio_completion_ratio)}/M`,
      })
    }

    if (other.image_ratio != null && other.image_ratio !== 1) {
      rows.push({
        label: t('Image input'),
        value: `${fmtPrice(baseInputUSD * other.image_ratio)}/M`,
      })
    }
  }

  if (other.web_search && other.web_search_call_count) {
    rows.push({
      label: t('Web Search'),
      value: `${other.web_search_call_count}x${other.web_search_price ? ` (${fmtPrice(other.web_search_price)})` : ''}`,
    })
  }

  if (other.file_search && other.file_search_call_count) {
    rows.push({
      label: t('File Search'),
      value: `${other.file_search_call_count}x${other.file_search_price ? ` (${fmtPrice(other.file_search_price)})` : ''}`,
    })
  }

  if (other.image_generation_call && other.image_generation_call_price) {
    rows.push({
      label: t('Image Generation'),
      value: fmtPrice(other.image_generation_call_price),
    })
  }

  if (other.audio_input_seperate_price && other.audio_input_price) {
    rows.push({
      label: t('Audio Input Price'),
      value: fmtPrice(other.audio_input_price),
    })
  }

  if (isAdmin && other.admin_info) {
    rows.push({
      label: t('Billing Path'),
      value: getUsageBillingPathLabel(t, other.admin_info),
    })
  }

  const usageFacts =
    other.usage_facts != null &&
    typeof other.usage_facts === 'object' &&
    !Array.isArray(other.usage_facts)
      ? Object.entries(other.usage_facts)
      : []

  return (
    <DetailSection label={t('Billing Details')}>
      {rows.map((row) => (
        <DetailRow key={row.label} label={row.label} value={row.value} mono />
      ))}
      {usageFacts.length > 0 && (
        <>
          <Label className='text-xs font-semibold'>
            {t('Usage parameters')}
          </Label>
          {usageFacts.map(([key, value]) => (
            <DetailRow
              key={`usage-fact-${key}`}
              label={key}
              value={String(value)}
              mono
            />
          ))}
        </>
      )}
      <DetailRow
        label={t('Total Cost')}
        value={formatLogQuota(log.quota)}
        mono
      />
    </DetailSection>
  )
}

function TokenBreakdown(props: { log: UsageLog; other: LogOtherData }) {
  const { t } = useTranslation()
  const { log, other } = props

  const promptTokens = log.prompt_tokens || 0
  const completionTokens = log.completion_tokens || 0
  const cacheRead = other.cache_tokens || 0
  const cacheWrite = other.cache_creation_tokens || 0
  const cacheWrite5m = other.cache_creation_tokens_5m || 0
  const cacheWrite1h = other.cache_creation_tokens_1h || 0
  const hasTokens = promptTokens > 0 || completionTokens > 0

  if (!hasTokens) return null

  const rows: Array<{ label: string; value: string }> = []

  rows.push({ label: t('Input Tokens'), value: promptTokens.toLocaleString() })
  rows.push({
    label: t('Output Tokens'),
    value: completionTokens.toLocaleString(),
  })

  if (cacheRead > 0) {
    rows.push({
      label: t('Cache Read'),
      value: cacheRead.toLocaleString(),
    })
  }

  if (other.image_cache_tokens !== undefined) {
    rows.push({
      label: t('Image Cache'),
      value: other.image_cache_tokens.toLocaleString(),
    })
  }

  if (cacheWrite > 0 && cacheWrite5m === 0 && cacheWrite1h === 0) {
    rows.push({
      label: t('Cache Write'),
      value: cacheWrite.toLocaleString(),
    })
  }

  if (cacheWrite5m > 0) {
    rows.push({
      label: t('Cache Write (5m)'),
      value: cacheWrite5m.toLocaleString(),
    })
  }

  if (cacheWrite1h > 0) {
    rows.push({
      label: t('Cache Write (1h)'),
      value: cacheWrite1h.toLocaleString(),
    })
  }

  if (other.image && other.image_output) {
    rows.push({
      label: t('Image Tokens'),
      value: other.image_output.toLocaleString(),
    })
  }

  return (
    <DetailSection label={t('Token Breakdown')}>
      {rows.map((row) => (
        <DetailRow key={row.label} label={row.label} value={row.value} mono />
      ))}
      {other.billing_tokens && (
        <div
          role='group'
          aria-label={t('Billable token breakdown')}
          className='space-y-2'
        >
          <Label className='text-xs font-semibold'>
            {t('Billable token breakdown')}
          </Label>
          {BILLING_PRICING_VARS.map((variable) => {
            const count = other.billing_tokens?.[variable.key]
            if (count === undefined || !Number.isFinite(count)) return null
            return (
              <DetailRow
                key={variable.key}
                label={t(variable.shortLabel)}
                value={count.toLocaleString()}
                mono
              />
            )
          })}
        </div>
      )}
    </DetailSection>
  )
}

interface DetailsDialogProps {
  log: UsageLog
  isAdmin: boolean
  isRoot: boolean
  open: boolean
  onOpenChange: (open: boolean) => void
}

export function DetailsDialog(props: DetailsDialogProps) {
  const { t } = useTranslation()
  const { copiedText, copyToClipboard } = useCopyToClipboard({ notify: false })
  const [recordSheetOpen, setRecordSheetOpen] = useState(false)
  const [fullLogSheetOpen, setFullLogSheetOpen] = useState(false)
  // 详情打开期间暂停自动刷新，避免刷新改写正在查看的行；
  // Provider 缺省（如独立复用的弹层）时自动降级为无操作。
  const logsContext = useOptionalUsageLogsContext()
  useEffect(() => {
    logsContext?.setIsDetailOpen(props.open)
    return () => {
      logsContext?.setIsDetailOpen(false)
    }
  }, [props.open, logsContext])
  // record/full_log 大字段已从列表响应剥离，打开详情时按需拉取。
  const { data: detailResponse } = useQuery({
    queryKey: ['log-detail', props.log.request_id],
    queryFn: () => getLogDetail(props.log.request_id),
    enabled: props.open && Boolean(props.log.request_id),
    staleTime: 60_000,
    gcTime: 5 * 60_000,
  })
  const record = detailResponse?.data?.record ?? ''
  const fullLog = detailResponse?.data?.full_log ?? ''
  const other = parseLogOther(props.log.other)
  const hasDetailAccess = props.isAdmin || hasDeveloperToolLogAccess()
  const typeConfig = getLogTypeConfig(props.log.type)

  const isViolation = isViolationFeeLog(other)
  const isRefund = props.log.type === 6
  const isConsume = props.log.type === 2
  const isTopup = props.log.type === 1
  const isManage = props.log.type === 3
  const isSubscription = other?.billing_source === 'subscription'
  const isTieredBilling =
    isConsume &&
    !isViolation &&
    other?.billing_mode === 'tiered_expr' &&
    !!other?.expr_b64
  const pricingData = usePricingData(props.open && isTieredBilling)
  const billingUsageSchema = pluginUsageSchema(
    pricingData.models.find(
      (model) => model.model_name === props.log.model_name
    ),
    other?.admin_info?.task_plugin?.key
  )
  const hasAudioTokens = other?.ws || other?.audio
  const showTiming = isTimingLogType(props.log.type)
  const showAdminIp =
    !!props.log.ip && (showTiming || (props.isAdmin && isTopup))
  const adminInfo = other?.admin_info
  const topupAuditFields =
    isTopup && props.isAdmin && adminInfo
      ? ([
          adminInfo.payment_method && {
            label: t('Order Payment Method'),
            value: adminInfo.payment_method,
          },
          adminInfo.callback_payment_method && {
            label: t('Callback Payment Method'),
            value: adminInfo.callback_payment_method,
          },
          adminInfo.caller_ip && {
            label: t('Callback Caller IP'),
            value: adminInfo.caller_ip,
          },
          adminInfo.server_ip && {
            label: t('Server IP'),
            value: adminInfo.server_ip,
          },
          adminInfo.node_name && {
            label: t('Node Name'),
            value: adminInfo.node_name,
          },
          adminInfo.version && {
            label: t('System Version'),
            value: adminInfo.version,
          },
        ].filter(Boolean) as Array<{ label: string; value: string }>)
      : []
  const showLegacyTopupWarning = isTopup && props.isAdmin && !adminInfo
  const showTopupAuditSection =
    isTopup &&
    props.isAdmin &&
    (topupAuditFields.length > 0 || showLegacyTopupWarning)
  const manageOperator = (() => {
    if (!isManage || !props.isAdmin || !adminInfo) return null
    const username = adminInfo.admin_username
    const id = adminInfo.admin_id
    const hasUsername = username != null && String(username).trim() !== ''
    const hasId = id != null && String(id).trim() !== ''
    if (!hasUsername && !hasId) return null
    if (hasUsername && hasId) return `${username} (ID: ${id})`
    if (hasUsername) return String(username)
    return `ID: ${id}`
  })()
  const authMethodLabel = (() => {
    if (!isManage || !props.isAdmin || !adminInfo?.auth_method) return ''
    if (adminInfo.auth_method === 'access_token') return t('Access Token')
    if (adminInfo.auth_method === 'session') return t('Session')
    return String(adminInfo.auth_method)
  })()

  // Top-up, audit, and login logs share the language-independent descriptor.
  const quotaOperation = isTopup
    ? buildQuotaAuditOperation(
        other?.op?.action ?? '',
        other?.op?.params ?? {},
        true,
        t
      )
    : null
  const operationText = renderAuditContent(other, t)
  const details = (isTopup ? operationText : null) ?? props.log.content ?? ''
  const auditRoute = isManage && props.isAdmin ? other?.audit_info : undefined
  // Channel update records which fields changed (stable field tokens); render
  // them with their localized labels for admins.
  const changedFieldTokens =
    isManage &&
    props.isAdmin &&
    Array.isArray(other?.op?.params?.changed_fields)
      ? (other.op.params.changed_fields as string[])
      : []
  const changedFieldsText = changedFieldTokens
    .map((field) => t(CHANNEL_FIELD_LABELS[field] ?? field))
    .join(', ')
  const showManageAuditSection =
    isManage && props.isAdmin && (operationText != null || auditRoute != null)

  // Login audit (type=7); visible to the log owner, not admin-only.
  const isLogin = props.log.type === 7
  const loginAuditFields = isLogin
    ? ([
        other?.login_method && {
          label: t('Login Method'),
          value: String(other.login_method),
        },
        props.log.ip && {
          label: t('IP Address'),
          value: props.log.ip,
        },
        other?.user_agent && {
          label: t('User Agent'),
          value: String(other.user_agent),
        },
      ].filter(Boolean) as Array<{ label: string; value: string }>)
    : []

  const conversionChain =
    other && Array.isArray(other.request_conversion)
      ? other.request_conversion.filter(Boolean)
      : []
  const conversionLabel =
    conversionChain.length <= 1
      ? t('Native format')
      : conversionChain.join(' -> ')
  const showConversion =
    props.isAdmin &&
    props.log.type !== 6 &&
    (other?.request_path || conversionChain.length > 0)

  const useChannel = other?.admin_info?.use_channel
  const channelChain =
    useChannel && useChannel.length > 0 ? useChannel.join(' → ') : undefined
  const reasoningEffortVariant = getReasoningEffortVariant(
    other?.reasoning_effort
  )

  const session = parseLogSession(props.log)
  const hasParent = Boolean(
    session.parentSessionName || session.parentSessionId
  )
  const mainSessionName = hasParent
    ? session.parentSessionName
    : session.sessionName
  const mainSessionId = hasParent ? session.parentSessionId : session.sessionId
  const subSessionName = hasParent ? session.sessionName : null
  const subSessionId = hasParent ? session.sessionId : null
  const subAgentName = session.agentName
  const subAgentId = session.agentId
  const hasSession = Boolean(
    mainSessionName ||
      mainSessionId ||
      subSessionName ||
      subSessionId ||
      subAgentName ||
      subAgentId ||
      session.parentSessionName ||
      session.parentSessionId
  )

  const userAgentDisplay = (() => {
    for (const raw of [record, fullLog, props.log.content]) {
      if (!raw) continue
      try {
        const parsed = JSON.parse(raw)
        const headers = parsed?.request?.headers || parsed?.headers
        if (headers && typeof headers === 'object' && !Array.isArray(headers)) {
          const key = Object.keys(headers).find(
            (k) => k.toLowerCase() === 'user-agent'
          )
          if (key && headers[key]) return String(headers[key]).trim()
        }
      } catch {
        // continue
      }
    }
    return undefined
  })()

  const clientSourceDisplay = (() => {
    if (other?.client_source && typeof other.client_source === 'string') {
      return other.client_source
    }
    if (userAgentDisplay) {
      const parsed = parseClientSource(userAgentDisplay)
      if (parsed.name !== '-') return parsed.name
    }
    return undefined
  })()
  return (
    <>
    <Dialog
      open={props.open}
      onOpenChange={props.onOpenChange}
      title={
        <>
          {t('Log Details')}
          <StatusBadge
            label={t(typeConfig.label)}
            variant={typeConfig.color as StatusBadgeProps['variant']}
            size='sm'
            copyable={false}
          />
        </>
      }
      description={t('View the complete details for this log entry')}
      contentClassName={cn(
        'min-w-0 overflow-hidden',
        'max-sm:max-h-(--dialog-available-height) max-sm:w-[calc(100vw-1.5rem)] max-sm:max-w-[calc(100vw-1.5rem)] max-sm:p-4',
        isTieredBilling ? 'sm:max-w-4xl lg:max-w-5xl' : 'sm:max-w-lg'
      )}
      headerClassName='max-sm:gap-1'
      titleClassName='flex items-center gap-2 text-base'
      descriptionClassName='sr-only'
      contentHeight='min(72dvh, 720px)'
      bodyClassName='pr-2 sm:pr-4'
    >
      <div className='w-full max-w-full min-w-0 space-y-2.5 overflow-x-hidden py-1 sm:space-y-3'>
        {/* Overview section - key identifiers */}
        <div className='min-w-0 space-y-1'>
          {props.log.request_id && (
            <DetailRow
              label={t('Request ID')}
              value={props.log.request_id}
              mono
            />
          )}
          {props.log.upstream_request_id && (
            <DetailRow
              label={t('Upstream Request ID')}
              value={props.log.upstream_request_id}
              mono
            />
          )}

          {props.isAdmin && props.log.channel > 0 && (
            <DetailRow
              label={t('Channel')}
              value={
                <span>
                  {props.log.channel}
                  {props.log.channel_name && (
                    <span className='text-muted-foreground'>
                      {' '}
                      ({props.log.channel_name})
                    </span>
                  )}
                </span>
              }
              mono
            />
          )}

          {channelChain && props.isAdmin && (
            <DetailRow label={t('Retry Chain')} value={channelChain} mono />
          )}

          {props.log.token_name && (
            <DetailRow label={t('Token')} value={props.log.token_name} mono />
          )}

          {(props.log.group || other?.group) && (
            <DetailRow
              label={t('Group')}
              value={props.log.group || other?.group || ''}
              mono
            />
          )}

          {showAdminIp && (
            <DetailRow
              label={t('IP Address')}
              value={
                <span className='flex items-center gap-1'>
                    <Globe
                      className='size-3 text-amber-500'
                      aria-hidden='true'
                    />
                  {props.log.ip}
                </span>
              }
              mono
            />
          )}

            {showTiming && <LogTimingDetails log={props.log} other={other} />}
          {clientSourceDisplay && (
            <DetailRow
              label={t('Source')}
              value={
                <span
                  className='inline-flex items-center justify-center rounded-full px-2 py-0.5 text-center text-xs font-medium'
                  style={{
                    backgroundColor: `color-mix(in srgb, ${stringToHslColor(clientSourceDisplay)} 15%, transparent)`,
                    color: stringToHslColor(clientSourceDisplay),
                  }}
                >
                  {clientSourceDisplay}
                </span>
              }
            />
          )}
          {userAgentDisplay && (
              <DetailRow label='User-Agent' value={userAgentDisplay} mono />
          )}
        </div>

        {/* Session Info */}
        {hasSession && (
          <DetailSection label={t('Session Info')}>
            {mainSessionName && (
              <DetailRow
                label={t('Session Name')}
                value={
                  <span className='inline-flex items-center gap-1.5'>
                    {(() => {
                        const badge = getBadgeStyle(
                          `session-${mainSessionName}`
                        )
                      return (
                        <span
                          className={`inline-flex items-center justify-center rounded-full px-2 py-0.5 text-xs font-medium ${badge.bg} ${badge.text}`}
                        >
                          {mainSessionName}
                        </span>
                      )
                    })()}
                  </span>
                }
              />
            )}
            {mainSessionId && (
                <DetailRow label={t('Session ID')} value={mainSessionId} mono />
            )}
            {subSessionName && (
                <DetailRow label={t('Sub Session')} value={subSessionName} />
            )}
            {subSessionId && (
              <DetailRow
                label={t('Sub Session ID')}
                value={subSessionId}
                mono
              />
            )}
            {subAgentName && (
                <DetailRow label={t('Agent Name')} value={subAgentName} />
            )}
            {subAgentId && (
                <DetailRow label={t('Agent ID')} value={subAgentId} mono />
            )}
            {session.parentSessionName && (
              <DetailRow
                label={t('Parent Session')}
                value={session.parentSessionName}
              />
            )}
            {session.parentSessionId && (
              <DetailRow
                label={t('Parent Session ID')}
                value={session.parentSessionId}
                mono
              />
            )}
          </DetailSection>
        )}

        {/* Request conversion (admin only, not for refund) */}
        {showConversion && (
          <DetailSection label={t('Request Conversion')}>
            <div className='relative min-w-0'>
              <Button
                variant='ghost'
                size='sm'
                className='absolute top-0 right-0 h-5 w-5 p-0'
                onClick={() => copyToClipboard(conversionLabel)}
                title={t('Copy to clipboard')}
                aria-label={t('Copy to clipboard')}
              >
                {copiedText === conversionLabel ? (
                  <Check className='size-3 text-green-600' />
                ) : (
                  <Copy className='size-3' />
                )}
              </Button>
              <div className='min-w-0 space-y-1 pr-6'>
                {other?.request_path && (
                  <DetailRow
                    label={t('Path')}
                    value={other.request_path}
                    mono
                  />
                )}
                <div className='flex min-w-0 items-center gap-1.5 text-xs'>
                  <Route
                    className='text-muted-foreground size-3'
                    aria-hidden='true'
                  />
                  <span className='min-w-0 break-all sm:wrap-break-word'>
                    {conversionLabel}
                  </span>
                </div>
              </div>
            </div>
          </DetailSection>
        )}

        {/* Quota saturation marker (admin only) */}
        {props.isAdmin && adminInfo?.request_policy?.length ? (
          <DetailSection
            label={t('Request policy decisions')}
            icon={<Route className='size-4' />}
          >
            <PolicyDecisionRecord events={adminInfo.request_policy} />
          </DetailSection>
        ) : null}
        {props.isAdmin && other?.admin_info?.quota_saturation && (
          <DetailSection
            icon={<AlertTriangle className='size-3.5' aria-hidden='true' />}
            label={t('Quota clamped')}
            variant='danger'
          >
            <p className='mb-1 text-xs wrap-break-word'>
              {t('Quota saturation protection triggered')}
            </p>
            <DetailRow
              label={t('Kind')}
              value={quotaSaturationKindLabel(
                other.admin_info.quota_saturation.kind,
                t
              )}
            />
            <DetailRow
              label={t('Original value')}
              value={String(other.admin_info.quota_saturation.original)}
              mono
            />
            <DetailRow
              label={t('Clamped to')}
              value={String(other.admin_info.quota_saturation.clamped)}
              mono
            />
            <DetailRow
              label={t('Operation')}
              value={other.admin_info.quota_saturation.op}
              mono
            />
          </DetailSection>
        )}

        {/* Reject reason (admin only) */}
        {props.isAdmin && adminInfo?.reject_reason && (
          <DetailSection
            icon={<AlertTriangle className='size-3.5' aria-hidden='true' />}
            label={t('Reject Reason')}
            variant='danger'
          >
              <p className='text-xs wrap-break-word'>
                {adminInfo.reject_reason}
              </p>
          </DetailSection>
        )}

        {/* Violation fee info */}
        {isViolation && other && (
          <DetailSection
            icon={<AlertTriangle className='size-3.5' aria-hidden='true' />}
            label={t('Violation Fee')}
            variant='danger'
          >
            {other.violation_fee_code && (
              <DetailRow
                label={t('Violation Code')}
                value={other.violation_fee_code}
                mono
              />
            )}
            {other.violation_fee_marker && (
              <DetailRow
                label={t('Violation Marker')}
                value={other.violation_fee_marker}
              />
            )}
            <DetailRow
              label={t('Fee Amount')}
              value={formatLogQuota(other.fee_quota ?? props.log.quota)}
              mono
            />
          </DetailSection>
        )}

        {/* Refund details (type=6) */}
        {isRefund && other && (other.task_id || other.reason) && (
          <DetailSection label={t('Refund Details')}>
            {other.task_id && (
              <DetailRow label={t('Task ID')} value={other.task_id} mono />
            )}
            {other.reason && (
              <DetailRow label={t('Reason')} value={other.reason} />
            )}
          </DetailSection>
        )}

        {props.isAdmin && adminInfo?.task_plugin ? (
          <DetailSection label={t('Task Plugin')}>
            <DetailRow
              label={t('Plugin key')}
              value={adminInfo.task_plugin.key}
              mono
            />
            <DetailRow label={t('Name')} value={adminInfo.task_plugin.name} />
            {adminInfo.task_plugin.version ? (
              <DetailRow
                label={t('Version')}
                value={adminInfo.task_plugin.version}
                mono
              />
            ) : null}
            {adminInfo.task_plugin.author ? (
              <DetailRow
                label={t('Plugin author')}
                value={
                  <PluginAuthorLink
                    author={adminInfo.task_plugin.author}
                    showUrl
                  />
                }
              />
            ) : null}
          </DetailSection>
        ) : null}

        {props.isRoot && other?.root_info ? (
          <DetailSection label={t('Root Diagnostics')}>
            {other.root_info.task_plugin ? (
              <>
                <DetailRow
                  label={t('API Version')}
                  value={String(other.root_info.task_plugin.api_version)}
                  mono
                />
                <DetailRow
                  label={t('Plugin Generation')}
                  value={String(other.root_info.task_plugin.generation)}
                  mono
                />
              </>
            ) : null}
            {other.root_info.upstream_task_id ? (
              <DetailRow
                label={t('Upstream Task ID')}
                value={other.root_info.upstream_task_id}
                mono
              />
            ) : null}
            {other.root_info.node_name ? (
              <DetailRow
                label={t('Node Name')}
                value={other.root_info.node_name}
                mono
              />
            ) : null}
          </DetailSection>
        ) : null}

        {/* Top-up audit info (type=1, admin only) */}
        {showTopupAuditSection && (
          <DetailSection
            icon={<ShieldCheck className='size-3.5' aria-hidden='true' />}
            iconTone='success'
            label={t('Top-up Audit Info')}
          >
            {topupAuditFields.map((field) => (
              <DetailRow
                key={field.label}
                label={field.label}
                value={field.value}
                mono
              />
            ))}
            {showLegacyTopupWarning && (
              <div className='flex items-start gap-1.5 text-xs text-amber-600 dark:text-amber-400'>
                  <Info
                    className='mt-0.5 size-3.5 shrink-0'
                    aria-hidden='true'
                  />
                <span>
                  {t(
                    'This historical record predates audit-info tracking and cannot be backfilled. The current instance already records server IP, callback IP, payment method, and system version for new top-ups going forward.'
                  )}
                </span>
              </div>
            )}
          </DetailSection>
        )}

        {quotaOperation && (
          <DetailSection label={t('Quota adjustment details')}>
            <AuditDetailFields fields={quotaOperation.fields} />
          </DetailSection>
        )}

        {/* Manage operator (type=3, admin only) */}
        {manageOperator && (
          <DetailRow
            label={
              <span className='flex items-center gap-1.5'>
                <UserCog
                  className='text-muted-foreground size-3.5'
                  aria-hidden='true'
                />
                {t('Operator Admin')}
              </span>
            }
            value={manageOperator}
            mono
          />
        )}

        {/* Operation audit info (type=3, admin only) */}
        {showManageAuditSection && (
          <DetailSection
            icon={<ShieldCheck className='size-3.5' aria-hidden='true' />}
            iconTone='info'
            label={t('Operation Audit Info')}
          >
            {operationText != null && (
              <DetailRow label={t('Operation')} value={operationText} />
            )}
            {authMethodLabel !== '' && (
              <DetailRow
                label={t('Authentication Method')}
                value={authMethodLabel}
              />
            )}
            {changedFieldsText !== '' && (
              <DetailRow
                label={t('Changed Fields')}
                value={changedFieldsText}
              />
            )}
            {auditRoute?.method && auditRoute?.route && (
              <DetailRow
                label={t('Request')}
                value={`${auditRoute.method} ${auditRoute.route}`}
                mono
              />
            )}
            {auditRoute?.status != null && (
              <DetailRow
                label={t('Result')}
                value={
                  auditRoute.success
                    ? `${t('Success')} (${auditRoute.status})`
                    : `${t('Failed')} (${auditRoute.status})`
                }
                mono
              />
            )}
          </DetailSection>
        )}

        {/* Login audit info (type=7) */}
        {isLogin && loginAuditFields.length > 0 && (
          <DetailSection
            icon={<LogIn className='size-3.5' aria-hidden='true' />}
            iconTone='info'
            label={t('Login Info')}
          >
            {operationText != null && (
              <DetailRow label={t('Operation')} value={operationText} />
            )}
            {loginAuditFields.map((field) => (
              <DetailRow
                key={field.label}
                label={field.label}
                value={field.value}
                mono
              />
            ))}
          </DetailSection>
        )}

        {/* Audio/WebSocket token breakdown */}
        {hasAudioTokens && other && (
          <DetailSection
            icon={<Headphones className='size-3.5' aria-hidden='true' />}
            iconTone='chart-4'
            label={t('Audio Tokens')}
          >
            {other.audio_input != null && other.audio_input > 0 && (
              <DetailRow
                label={t('Audio Input')}
                value={formatTokens(other.audio_input)}
                mono
              />
            )}
            {other.audio_output != null && other.audio_output > 0 && (
              <DetailRow
                label={t('Audio Output')}
                value={formatTokens(other.audio_output)}
                mono
              />
            )}
            {other.text_input != null && other.text_input > 0 && (
              <DetailRow
                label={t('Text Input')}
                value={formatTokens(other.text_input)}
                mono
              />
            )}
            {other.text_output != null && other.text_output > 0 && (
              <DetailRow
                label={t('Text Output')}
                value={formatTokens(other.text_output)}
                mono
              />
            )}
          </DetailSection>
        )}

        {/* Reasoning effort */}
        {other?.reasoning_effort && (
          <DetailRow
            label={t('Reasoning Effort')}
            value={
              <StatusBadge
                label={other.reasoning_effort}
                variant={reasoningEffortVariant}
                size='sm'
                copyable={false}
              />
            }
          />
        )}

        {/* System prompt override */}
        {other?.is_system_prompt_overwritten && (
          <DetailRow
            label={t('System Prompt')}
            value={
              <StatusBadge
                label={t('Overwritten')}
                variant='orange'
                size='sm'
                copyable={false}
              />
            }
          />
        )}

        {other?.response_model && (
          <DetailSection label={t('Response Model')}>
            <ResponseModelDetails observation={other.response_model} />
          </DetailSection>
        )}
        {/* Model mapping for logs without response observations */}
        {!other?.response_model &&
          other?.is_model_mapped &&
          other?.upstream_model_name && (
            <DetailSection label={t('Model Mapping')}>
              <DetailRow
                label={t('Request Model')}
                value={props.log.model_name}
                mono
              />
              <DetailRow
                label={t('Actual Model')}
                value={other.upstream_model_name}
                mono
              />
            </DetailSection>
          )}

        {/* Token breakdown (for consume/error types with token data) */}
        {isDisplayableType(props.log.type) && other && (
          <TokenBreakdown log={props.log} other={other} />
        )}

        {/* Billing breakdown (consume type) */}
        {isConsume && other && !isViolation && (
          <BillingBreakdown
            log={props.log}
            other={other}
            isAdmin={props.isAdmin}
          />
        )}

        {/* Tiered pricing breakdown (when billing_mode is tiered_expr) */}
        {isTieredBilling && other?.expr_b64 && (
          <DetailSection label={t('Dynamic Pricing')}>
            {other.image_count !== undefined && (
              <DetailRow
                label={t('Billable image count')}
                value={other.image_count}
              />
            )}
            <DynamicPricingBreakdown
              compact
              billingExpr={decodeBillingExprB64(other.expr_b64)}
              matchedTierLabel={other.matched_tier}
              matchedBillingUnit={other.billing_unit}
              matchedFixedPrice={other.fixed_price}
              requestRules={other.request_rules}
              hideCacheColumns={!hasAnyCacheTokens(other)}
              usageSchema={billingUsageSchema}
              usageFacts={other.usage_facts}
            />
          </DetailSection>
        )}

        {/* Admin billing mode indicator for non-consume */}
        {props.isAdmin &&
          !isConsume &&
          props.log.type !== 6 &&
          other?.admin_info && (
            <DetailRow
              label={t('Billing Path')}
              value={
                <span className='flex items-center gap-1'>
                  {isUsageBillingPathLocal(other.admin_info) ? (
                    <Monitor className='size-3 text-blue-500' />
                  ) : (
                    <Cloud className='size-3 text-emerald-500' />
                  )}
                  <span className='text-xs'>
                    {getUsageBillingPathLabel(t, other.admin_info)}
                  </span>
                </span>
              }
            />
          )}

        {/* Stream status details */}
        {other?.stream_status && other.stream_status.status !== 'ok' && (
          <DetailSection label={t('Stream Status')}>
            <DetailRow
              label={t('Status')}
              value={
                <StatusBadge
                  label={other.stream_status.status || t('Error')}
                  variant='red'
                  size='sm'
                  copyable={false}
                />
              }
            />
            {other.stream_status.end_reason && (
              <DetailRow
                label={t('End Reason')}
                value={other.stream_status.end_reason}
              />
            )}
            {(other.stream_status.error_count ?? 0) > 0 && (
              <DetailRow
                label={t('Soft Errors')}
                value={String(other.stream_status.error_count)}
              />
            )}
            {other.stream_status.end_error && (
              <DetailRow
                label={t('End Error')}
                value={other.stream_status.end_error}
              />
            )}
            {Array.isArray(other.stream_status.errors) &&
              other.stream_status.errors.length > 0 && (
                <pre className='bg-background/60 mt-1 max-h-32 overflow-y-auto rounded border p-2 font-mono text-[11px] leading-relaxed wrap-break-word whitespace-pre-wrap'>
                  {other.stream_status.errors.join('\n')}
                </pre>
              )}
          </DetailSection>
        )}

        {/* Subscription billing details */}
        {isSubscription && other && (
          <DetailSection label={t('Subscription Billing')}>
            {other.subscription_plan_id && (
              <DetailRow
                label={t('Plan')}
                value={`#${other.subscription_plan_id} ${other.subscription_plan_title || ''}`.trim()}
              />
            )}
            {other.subscription_id && (
              <DetailRow
                label={t('Instance')}
                value={`#${other.subscription_id}`}
                mono
              />
            )}
            {other.subscription_pre_consumed != null && (
              <DetailRow
                label={t('Pre-consumed')}
                value={formatLogQuota(other.subscription_pre_consumed)}
                mono
              />
            )}
            {other.subscription_post_delta != null &&
              other.subscription_post_delta !== 0 && (
                <DetailRow
                  label={t('Post Delta')}
                  value={formatLogQuota(other.subscription_post_delta)}
                  mono
                />
              )}
            {other.subscription_consumed != null && (
              <DetailRow
                label={t('Final Consumed')}
                value={formatLogQuota(other.subscription_consumed)}
                mono
              />
            )}
            {other.subscription_remain != null && (
              <DetailRow
                label={t('Remaining')}
                value={`${formatLogQuota(other.subscription_remain)}${other.subscription_total != null ? ` / ${formatLogQuota(other.subscription_total)}` : ''}`}
                mono
              />
            )}
          </DetailSection>
        )}

        {/* Param override */}
        {other?.po && Array.isArray(other.po) && other.po.length > 0 && (
          <DetailSection
            icon={<Settings2 className='size-3.5' aria-hidden='true' />}
            iconTone='chart-3'
            label={`${t('Param Override')} (${other.po.length})`}
          >
            {other.po.filter(Boolean).map((line) => {
              const parsed = parseAuditLine(line)
              if (!parsed) return null
              return (
                <div
                  key={`${parsed.action}-${parsed.content}`}
                  className='bg-background/60 flex min-w-0 flex-col gap-1.5 rounded border p-2 sm:flex-row sm:items-start sm:gap-2'
                >
                  <StatusBadge
                    variant='neutral'
                    label={getParamOverrideActionLabel(parsed.action, t)}
                    className='shrink-0 font-medium'
                    copyable={false}
                  />
                  <span className='min-w-0 font-mono text-[11px] leading-relaxed break-all sm:wrap-break-word'>
                    {parsed.content}
                  </span>
                </div>
              )
            })}
          </DetailSection>
        )}

        {/* Detailed logs */}
          {hasDetailAccess &&
            Boolean(record || fullLog) && (
          <div className='flex flex-wrap items-center gap-2 border-t pt-3'>
            {Boolean(record) && (
              <Button
                variant='outline'
                size='sm'
                className='text-xs'
                onClick={() => setRecordSheetOpen(true)}
              >
                <FileText className='mr-1.5 size-3.5' />
                {t('View Record')}
              </Button>
            )}
            {Boolean(fullLog) && (
              <Button
                variant='outline'
                size='sm'
                className='text-xs'
                onClick={() => setFullLogSheetOpen(true)}
              >
                <Code className='mr-1.5 size-3.5' />
                {t('View Full Log')}
              </Button>
            )}
          </div>
        )}

        {/* Content */}
        {details && (
          <div className='space-y-1.5'>
            <Label className='text-xs font-semibold'>{t('Content')}</Label>
            <div className='bg-muted/30 relative min-w-0 overflow-hidden rounded-md border p-2.5'>
              <Button
                variant='ghost'
                size='sm'
                className='absolute top-1.5 right-1.5 h-5 w-5 p-0'
                onClick={() => copyToClipboard(details)}
                title={t('Copy to clipboard')}
                aria-label={t('Copy to clipboard')}
              >
                {copiedText === details ? (
                  <Check className='size-3 text-green-600' />
                ) : (
                  <Copy className='size-3' />
                )}
              </Button>
              <p className='min-w-0 pr-6 text-xs leading-relaxed break-all whitespace-pre-wrap sm:wrap-break-word'>
                {details}
              </p>
            </div>
          </div>
        )}
      </div>
    </Dialog>
    {Boolean(record) && (
      <LogDetailSheet
        open={recordSheetOpen}
        onOpenChange={setRecordSheetOpen}
        title={t('Consumption Record Details')}
          description={t(
            'Formatted view of the request and response lifecycle'
          )}
        content={record}
        structured
      />
    )}
    {Boolean(fullLog) && (
      <LogDetailSheet
        open={fullLogSheetOpen}
        onOpenChange={setFullLogSheetOpen}
        title={t('Full Log Payload')}
        description={t('Raw upstream payloads and timing breakdown')}
        content={fullLog}
      />
    )}
  </>
  )
}

function isDisplayableType(type: number): boolean {
  return [0, 2, 5, 6].includes(type)
}

function parseJsonRecord(content: string): Record<string, unknown> | null {
  if (!content) return null
  try {
    const parsed = JSON.parse(content)
    return parsed && typeof parsed === 'object' && !Array.isArray(parsed)
      ? (parsed as Record<string, unknown>)
      : null
  } catch {
    return null
  }
}

function ToolUseTable(props: { rows: ToolUseRow[] }) {
  const { t } = useTranslation()
  if (props.rows.length === 0) return null
  return (
    <div className='overflow-x-auto'>
      <table className='w-full text-xs'>
        <thead>
          <tr className='border-b text-left'>
            <th className='px-2 py-1.5 font-medium'>#</th>
            <th className='px-2 py-1.5 font-medium'>{t('Tool')}</th>
            <th className='px-2 py-1.5 font-medium'>{t('Call ID')}</th>
            <th className='px-2 py-1.5 font-medium'>{t('Arguments')}</th>
          </tr>
        </thead>
        <tbody>
          {props.rows.map((row) => (
            <tr key={row.id} className='border-b last:border-b-0'>
              <td className='px-2 py-1.5 font-mono'>{row.order}</td>
              <td className='px-2 py-1.5 font-semibold'>{row.name || '-'}</td>
              <td className='max-w-[200px] px-2 py-1.5 font-mono break-all'>
                {row.callId || row.id || '-'}
              </td>
              <td className='max-w-[300px] px-2 py-1.5'>
                {row.arguments != null || row.input != null ? (
                  <pre className='m-0 font-mono text-[11px] leading-relaxed wrap-break-word whitespace-pre-wrap'>
                    {JSON.stringify(row.arguments ?? row.input, null, 2)}
                  </pre>
                ) : (
                  '-'
                )}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}

function ToolResponseTable(props: {
  rows: Array<{ order: number; name: string; callId: string; type?: string }>
}) {
  const { t } = useTranslation()
  if (props.rows.length === 0) return null
  return (
    <div className='overflow-x-auto'>
      <table className='w-full text-xs'>
        <thead>
          <tr className='border-b text-left'>
            <th className='px-2 py-1.5 font-medium'>#</th>
            <th className='px-2 py-1.5 font-medium'>{t('Tool')}</th>
            <th className='px-2 py-1.5 font-medium'>{t('Call ID')}</th>
          </tr>
        </thead>
        <tbody>
          {props.rows.map((row) => (
            <tr
              key={`${row.callId}-${row.order}`}
              className='border-b last:border-b-0'
            >
              <td className='px-2 py-1.5 font-mono'>{row.order}</td>
              <td className='px-2 py-1.5 font-semibold'>{row.name || '-'}</td>
              <td className='max-w-[300px] px-2 py-1.5 font-mono break-all'>
                {row.callId || '-'}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}

function StructuredLogContent(props: {
  sections: ParsedSections
  rawContent: string
}) {
  const { t } = useTranslation()
  const { copiedText, copyToClipboard } = useCopyToClipboard({ notify: false })
  const { sections } = props

  const hasThinking = sections.thinking.trim() !== ''
  const hasAnswer = sections.answer.trim() !== ''
  const hasToolUses = sections.toolUses.length > 0
  const hasRequestBlocks = sections.requestBlocks.length > 0
  const hasToolResponses = sections.toolResponses.length > 0
  const hasHeaders = Object.keys(sections.headers).length > 0

  const headersJson = useMemo(
    () => JSON.stringify(sections.headers, null, 2),
    [sections.headers]
  )

  const copyAllText = JSON.stringify(
    {
      headers: sections.headers,
      request: sections.requestBlocks,
      thinking: sections.thinking,
      answer: sections.answer,
      toolUses: sections.toolUses,
      toolResponses: sections.toolResponses,
    },
    null,
    2
  )

  return (
    <div className='space-y-2.5'>
      {hasHeaders && (
        <DetailSection
          icon={<FileText className='size-3.5' aria-hidden='true' />}
          label={t('Request Headers')}
        >
          <div className='relative'>
            <Button
              variant='ghost'
              size='sm'
              className='absolute -top-0.5 right-0 h-5 w-5 p-0'
              onClick={() => copyToClipboard(headersJson)}
              title={t('Copy to clipboard')}
              aria-label={t('Copy to clipboard')}
            >
              {copiedText === headersJson ? (
                <Check className='size-3 text-green-600' />
              ) : (
                <Copy className='size-3' />
              )}
            </Button>
            <div className='max-h-64 overflow-y-auto pr-5'>
              <dl className='space-y-1'>
                {Object.entries(sections.headers).map(([key, value]) => (
                  <div
                    key={key}
                    className='border-border/40 flex items-start gap-2 border-b border-dashed pb-1 last:border-0 last:pb-0'
                  >
                    <dt className='text-muted-foreground w-1/3 shrink-0 font-mono text-[11px] font-semibold break-all'>
                      {key}
                    </dt>
                    <dd className='min-w-0 flex-1 font-mono text-[11px] leading-relaxed break-all'>
                      {value}
                    </dd>
                  </div>
                ))}
              </dl>
            </div>
          </div>
        </DetailSection>
      )}

      {hasRequestBlocks && (
        <DetailSection
          icon={<ArrowDownToLine className='size-3.5' aria-hidden='true' />}
          label={t('Input')}
        >
          <div className='relative space-y-2'>
            <Button
              variant='ghost'
              size='sm'
              className='absolute -top-0.5 right-0 h-5 w-5 p-0'
              onClick={() =>
                copyToClipboard(
                  sections.requestBlocks.map((b) => b.text).join('\n\n')
                )
              }
              title={t('Copy to clipboard')}
              aria-label={t('Copy to clipboard')}
            >
              {copiedText ===
              sections.requestBlocks.map((b) => b.text).join('\n\n') ? (
                <Check className='size-3 text-green-600' />
              ) : (
                <Copy className='size-3' />
              )}
            </Button>
            {sections.requestBlocks.map((block, index) => (
              <div
                key={`${block.role || 'user'}-${block.type}-${block.text.slice(0, 32)}`}
                className='bg-background/60 rounded-md border p-2'
              >
                <div className='mb-1 flex items-center justify-between gap-2'>
                  <span className='text-xs font-semibold'>
                    {t('Input')} #{index + 1}
                  </span>
                  <span className='text-muted-foreground text-[11px]'>
                    {[block.role, block.type].filter(Boolean).join(' · ') ||
                      block.type}
                  </span>
                </div>
                <MarkdownSourceHighlighter
                  content={block.text}
                  fontSize={12}
                  className='pr-5'
                />
              </div>
            ))}
          </div>
        </DetailSection>
      )}

      {hasToolResponses && (
        <DetailSection
          icon={<Wrench className='size-3.5' aria-hidden='true' />}
          label={t('Tool Responses')}
        >
          <div className='relative'>
            <Button
              variant='ghost'
              size='sm'
              className='absolute -top-0.5 right-0 h-5 w-5 p-0'
              onClick={() =>
                copyToClipboard(JSON.stringify(sections.toolResponses, null, 2))
              }
              title={t('Copy to clipboard')}
              aria-label={t('Copy to clipboard')}
            >
              {copiedText ===
              JSON.stringify(sections.toolResponses, null, 2) ? (
                <Check className='size-3 text-green-600' />
              ) : (
                <Copy className='size-3' />
              )}
            </Button>
            <ToolResponseTable rows={sections.toolResponses} />
          </div>
        </DetailSection>
      )}

      {hasThinking && (
        <DetailSection
          icon={<Brain className='size-3.5' aria-hidden='true' />}
          label={t('Thinking')}
        >
          <details className='bg-background/60 rounded-md border'>
            <summary className='cursor-pointer px-2.5 py-1.5 text-xs font-medium select-none'>
              {t('Show thinking content')}
            </summary>
            <div className='relative border-t px-2.5 py-2'>
              <Button
                variant='ghost'
                size='sm'
                className='absolute top-1.5 right-1 h-5 w-5 p-0'
                onClick={() => copyToClipboard(sections.thinking)}
                title={t('Copy to clipboard')}
                aria-label={t('Copy to clipboard')}
              >
                {copiedText === sections.thinking ? (
                  <Check className='size-3 text-green-600' />
                ) : (
                  <Copy className='size-3' />
                )}
              </Button>
              <MarkdownSourceHighlighter
                content={sections.thinking}
                fontSize={11}
                className='max-h-64 overflow-y-auto pr-6'
              />
            </div>
          </details>
        </DetailSection>
      )}

      {hasAnswer && (
        <DetailSection
          icon={<MessageSquare className='size-3.5' aria-hidden='true' />}
          label={t('Answer')}
        >
          <div className='relative'>
            <Button
              variant='ghost'
              size='sm'
              className='absolute -top-0.5 right-0 h-5 w-5 p-0'
              onClick={() => copyToClipboard(sections.answer)}
              title={t('Copy to clipboard')}
              aria-label={t('Copy to clipboard')}
            >
              {copiedText === sections.answer ? (
                <Check className='size-3 text-green-600' />
              ) : (
                <Copy className='size-3' />
              )}
            </Button>
            <MarkdownSourceHighlighter
              content={sections.answer}
              fontSize={12}
              className='max-h-64 overflow-y-auto pr-6'
            />
          </div>
        </DetailSection>
      )}

      {hasToolUses && (
        <DetailSection
          icon={<Wrench className='size-3.5' aria-hidden='true' />}
          label={`${t('Tool Calls')} (${sections.toolUses.length})`}
        >
          <div className='relative'>
            <Button
              variant='ghost'
              size='sm'
              className='absolute -top-0.5 right-0 h-5 w-5 p-0'
              onClick={() =>
                copyToClipboard(JSON.stringify(sections.toolUses, null, 2))
              }
              title={t('Copy to clipboard')}
              aria-label={t('Copy to clipboard')}
            >
              {copiedText === JSON.stringify(sections.toolUses, null, 2) ? (
                <Check className='size-3 text-green-600' />
              ) : (
                <Copy className='size-3' />
              )}
            </Button>
            <ToolUseTable rows={sections.toolUses} />
          </div>
        </DetailSection>
      )}

      <div className='flex justify-end'>
        <Button
          variant='ghost'
          size='sm'
          className='h-6 gap-1 px-2 text-[11px]'
          onClick={() => copyToClipboard(copyAllText)}
        >
          {copiedText === copyAllText ? (
            <Check className='size-3 text-green-600' />
          ) : (
            <Copy className='size-3' />
          )}
          {t('Copy all')}
        </Button>
      </div>
    </div>
  )
}

function LogDetailSheet(props: {
  open: boolean
  onOpenChange: (open: boolean) => void
  title: string
  description: string
  content: string
  structured?: boolean
}) {
  const { t } = useTranslation()
  const { copiedText, copyToClipboard } = useCopyToClipboard({ notify: false })
  const parsedRecord = useMemo(
    () => parseJsonRecord(props.content),
    [props.content]
  )
  const sections = useMemo(
    () =>
      props.structured
        ? parseLogDetailRecord(
            parsedRecord as Parameters<typeof parseLogDetailRecord>[0]
          )
        : null,
    [parsedRecord, props.structured]
  )
  const isStructured = Boolean(sections && hasStructuredData(sections))
  const rawJson = useMemo(() => {
    if (!parsedRecord) return props.content
    return JSON.stringify(parsedRecord, null, 2)
  }, [parsedRecord, props.content])

  return (
    <Sheet open={props.open} onOpenChange={props.onOpenChange}>
      <SheetContent
        side='right'
        className='h-[100dvh] w-[92vw] gap-0 p-0 sm:max-w-2xl lg:max-w-3xl'
      >
        <SheetHeader className='flex-shrink-0 border-b pr-12'>
          <SheetTitle>{props.title}</SheetTitle>
          <SheetDescription>{props.description}</SheetDescription>
        </SheetHeader>
        <ScrollArea className='min-h-0 flex-1 overflow-hidden'>
          <div className='space-y-3 p-4'>
            {isStructured && sections ? (
              <StructuredLogContent sections={sections} rawContent={rawJson} />
            ) : (
              <div className='bg-muted/30 relative min-w-0 overflow-hidden rounded-md border p-2.5'>
                <Button
                  variant='ghost'
                  size='sm'
                  className='absolute top-1.5 right-1.5 h-5 w-5 p-0'
                  onClick={() => copyToClipboard(rawJson)}
                  title={t('Copy to clipboard')}
                  aria-label={t('Copy to clipboard')}
                >
                  {copiedText === rawJson ? (
                    <Check className='size-3 text-green-600' />
                  ) : (
                    <Copy className='size-3' />
                  )}
                </Button>
                <MarkdownSourceHighlighter
                  content={rawJson}
                  fontSize={12}
                  className='pr-6'
                />
              </div>
            )}

            {isStructured && (
              <DetailSection label={t('Raw JSON')}>
                <div className='relative min-w-0'>
                  <Button
                    variant='ghost'
                    size='sm'
                    className='absolute top-1 right-1 h-5 w-5 p-0'
                    onClick={() => copyToClipboard(rawJson)}
                    title={t('Copy to clipboard')}
                    aria-label={t('Copy to clipboard')}
                  >
                    {copiedText === rawJson ? (
                      <Check className='size-3 text-green-600' />
                    ) : (
                      <Copy className='size-3' />
                    )}
                  </Button>
                  <MarkdownSourceHighlighter
                    content={rawJson}
                    fontSize={11}
                    className='max-h-96 overflow-y-auto pr-6'
                  />
                </div>
              </DetailSection>
            )}
          </div>
        </ScrollArea>
      </SheetContent>
    </Sheet>
  )
}
