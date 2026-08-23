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

import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip'
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
  single_turn: 'Single Turn',
}

// host-tool builtin 请求（网关本地合成，未请求上游模型）的统一解释文案，
// 徽章与 TPS / 缓存率列的占位符共用。
export const HOST_TOOL_INTERNAL_HINT =
  'Gateway executed this host-tool request locally without calling the upstream model, so tokens, TPS and cache rate do not apply'

function InternalToolBadge() {
  const { t } = useTranslation()
  return (
    <TooltipProvider>
      <Tooltip>
        <TooltipTrigger
          render={
            <span className='bg-muted text-muted-foreground inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium'>
              {t('Internal Tool')}
            </span>
          }
        />
        <TooltipContent side='top' className='max-w-[240px] p-2'>
          <p className='text-xs'>{t(HOST_TOOL_INTERNAL_HINT)}</p>
        </TooltipContent>
      </Tooltip>
    </TooltipProvider>
  )
}

// TPS / 缓存率列对内部工具请求显示的「—」，悬停可查看原因。
export function HostToolInternalDash() {
  const { t } = useTranslation()
  return (
    <TooltipProvider>
      <Tooltip>
        <TooltipTrigger
          render={<span className='text-muted-foreground/60'>—</span>}
        />
        <TooltipContent side='top' className='max-w-[240px] p-2'>
          <p className='text-xs'>{t(HOST_TOOL_INTERNAL_HINT)}</p>
        </TooltipContent>
      </Tooltip>
    </TooltipProvider>
  )
}

export function InteractionTypeCell(props: InteractionTypeCellProps) {
  const { t } = useTranslation()
  if (!isDisplayableLogType(props.log.type)) return null

  const other = parseLogOther(props.log.other)
  const interactionType = resolveInteractionTypeFromLog(props.log)
  const activityTags = collectUsageActivityTags(other)
  const isInternalTool = Boolean(other?.host_tool_internal)
  if (!interactionType && activityTags.length === 0 && !isInternalTool) {
    return null
  }

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
      {isInternalTool ? <InternalToolBadge /> : null}
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
