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
import { useTranslation } from 'react-i18next'

import { StatusBadge } from '@/components/status-badge'
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { formatLogQuota } from '@/lib/format'

import type { LogOtherData } from '../types'

interface LogCostDisplayProps {
  quota: number
  other: LogOtherData | null
}

function splitQuotaDisplay(value: string): { prefix: string; amount: string } {
  const match = value.match(/^([^0-9+\-.,\s]+)(.+)$/)
  if (!match) return { prefix: '', amount: value }
  return { prefix: match[1], amount: match[2] }
}

function QuotaBadge(props: { quota: number }) {
  const quotaDisplay = splitQuotaDisplay(formatLogQuota(props.quota))

  return (
    <span className='border-border/80 bg-muted/60 inline-flex h-6 w-fit items-center rounded-md border px-2 [font-family:var(--font-body)] text-sm leading-none font-semibold tabular-nums'>
      {quotaDisplay.prefix ? (
        <span className='mr-1'>{quotaDisplay.prefix}</span>
      ) : null}
      <span>{quotaDisplay.amount}</span>
    </span>
  )
}

function SubscriptionBadge(props: { quota: number }) {
  const { t } = useTranslation()

  return (
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
          {t('Deducted by subscription')}: {formatLogQuota(props.quota)}
        </span>
      </TooltipContent>
    </Tooltip>
  )
}

export function LogCostDisplay(props: LogCostDisplayProps) {
  if (props.other?.billing_source !== 'subscription') {
    return <QuotaBadge quota={props.quota} />
  }

  return (
    <TooltipProvider>
      <SubscriptionBadge quota={props.quota} />
    </TooltipProvider>
  )
}
