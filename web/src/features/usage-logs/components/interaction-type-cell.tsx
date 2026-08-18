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

import { getBadgeStyle } from '@/lib/colors'
import { cn } from '@/lib/utils'

import type { UsageLog } from '../data/schema'
import { collectUsageActivityTags, parseLogOther } from '../lib/format'
import {
  resolveInteractionTypeFromLog,
  type InteractionType,
} from '../lib/interaction-parser'
import { isDisplayableLogType } from '../lib/utils'
import { UsageActivityTags } from './usage-activity-tags'

interface InteractionTypeCellProps {
  log: UsageLog
  align?: 'start' | 'center'
  className?: string
}

const INTERACTION_LABEL_KEYS: Record<InteractionType, string> = {
  input: 'Input',
  output: 'Output',
  callback: 'Callback',
}

export function InteractionTypeCell(props: InteractionTypeCellProps) {
  const { t } = useTranslation()
  if (!isDisplayableLogType(props.log.type)) return null

  const other = parseLogOther(props.log.other)
  const interactionType = resolveInteractionTypeFromLog(props.log)
  const activityTags = collectUsageActivityTags(other)
  if (!interactionType && activityTags.length === 0) return null

  const align = props.align ?? 'center'
  const badge = interactionType
    ? getBadgeStyle(`interaction-type-${interactionType}`)
    : null

  return (
    <div
      className={cn(
        'inline-flex max-w-full items-center gap-1 whitespace-nowrap',
        align === 'center' ? 'justify-center' : 'justify-start',
        props.className
      )}
    >
      {interactionType && badge ? (
        <span
          className={`inline-flex items-center justify-center rounded-full px-2 py-0.5 text-center text-xs font-medium ${badge.bg} ${badge.text}`}
        >
          {t(INTERACTION_LABEL_KEYS[interactionType])}
        </span>
      ) : null}
      <UsageActivityTags other={other} />
    </div>
  )
}
