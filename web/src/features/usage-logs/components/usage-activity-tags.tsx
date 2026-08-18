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
import { cn } from '@/lib/utils'

import {
  collectUsageActivityTags,
  getUsageActivityTagDetails,
  type UsageActivityTag,
  type UsageActivityTagDetails,
} from '../lib/format'
import type { LogOtherData } from '../types'

interface UsageActivityTagsProps {
  other: LogOtherData | null
  className?: string
  align?: 'start' | 'center'
}

function formatDurationMs(ms: number): string {
  if (ms < 1000) return `${Math.round(ms)}ms`
  return `${(ms / 1000).toFixed(2)}s`
}

function hostToolNameLine(details: UsageActivityTagDetails, index: number): string {
  const name = details.names[index] ?? details.names[0]
  const canonical = details.canonicals[index] ?? details.canonicals[0]
  if (!name) return canonical ?? ''
  if (canonical && canonical !== name) return `${name} → ${canonical}`
  return name
}

function UsageActivityTagTooltip(props: { details: UsageActivityTagDetails }) {
  const { t } = useTranslation()
  const details = props.details

  if (details.kind === 'image_recognize') {
    return (
      <div className='space-y-0.5'>
        <p>{t('Image recognition hop')}</p>
        <p className='text-muted-foreground'>
          {t('{{count}} images', { count: details.count })}
        </p>
      </div>
    )
  }

  const lastDuration = details.durationMs.at(-1)
  const meta = [
    t('{{count}} calls', { count: details.count }),
    ...details.backends,
    lastDuration != null ? formatDurationMs(lastDuration) : null,
  ].filter((item): item is string => Boolean(item))

  return (
    <div className='space-y-0.5'>
      <p>{t('Used host tool')}</p>
      {details.names.map((name, index) => {
        const line = hostToolNameLine(details, index)
        return (
          <p key={line || name} className='font-mono text-xs'>
            {line}
          </p>
        )
      })}
      {meta.length > 0 ? (
        <p className='text-muted-foreground'>{meta.join(' · ')}</p>
      ) : null}
    </div>
  )
}

function UsageActivityTagBadge(props: {
  tag: UsageActivityTag
  other: LogOtherData | null
}) {
  const { t } = useTranslation()
  const label = t(props.tag.labelKey)
  const details = getUsageActivityTagDetails(props.other, props.tag)

  return (
    <Tooltip>
      <TooltipTrigger
        render={
          <StatusBadge
            label={label}
            variant='info'
            size='sm'
            copyable={false}
            className='h-5 cursor-help px-1.5'
            data-usage-activity-tag={props.tag.id}
          />
        }
      />
      <TooltipContent>
        <UsageActivityTagTooltip details={details} />
      </TooltipContent>
    </Tooltip>
  )
}

export function UsageActivityTags(props: UsageActivityTagsProps) {
  const tags = collectUsageActivityTags(props.other)
  if (tags.length === 0) return null

  return (
    <TooltipProvider>
      <div
        className={cn(
          'flex flex-wrap gap-1',
          props.align === 'center' && 'justify-center',
          props.className
        )}
      >
        {tags.map((tag) => (
          <UsageActivityTagBadge
            key={tag.id}
            tag={tag}
            other={props.other}
          />
        ))}
      </div>
    </TooltipProvider>
  )
}
