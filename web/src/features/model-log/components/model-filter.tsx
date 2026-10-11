import { useTranslation } from 'react-i18next'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { formatNumber, formatTokenUnits } from '@/lib/format'

import { formatModelTpsSummary } from '../lib/chart-specs'
import type { SortField, TokenRecordSummary } from '../types'

interface ModelFilterProps {
  models: string[]
  selectedModels: Set<string>
  onToggleModel: (model: string) => void
  onSelectAll: () => void
  onDeselectAll: () => void
  modelColorMap: ReadonlyMap<string, string>
  modelSummaries?: ReadonlyMap<string, TokenRecordSummary>
  sortField?: SortField
  locale?: string
}

export function ModelFilter({
  models,
  selectedModels,
  onToggleModel,
  onSelectAll,
  onDeselectAll,
  modelColorMap,
  modelSummaries,
  sortField,
  locale,
}: ModelFilterProps) {
  const { t } = useTranslation()

  return (
    <div className='flex flex-wrap items-center gap-2 py-2'>
      <span className='text-muted-foreground text-xs'>
        {t('Model Filter')}:
      </span>
      <Button variant='outline' size='sm' onClick={onSelectAll}>
        {t('Select All')}
      </Button>
      <Button variant='outline' size='sm' onClick={onDeselectAll}>
        {t('Deselect All')}
      </Button>
      <div className='flex flex-wrap gap-1.5'>
        {models.map((model) => {
          const isSelected = selectedModels.has(model)
          const color = modelColorMap.get(model)
          const summary = modelSummaries?.get(model)
          const avgTpsLabel = summary
            ? formatModelTpsSummary(summary.avg_tps, summary.tps_sample_count)
            : 'N/A'
          const badgeTitle = summary
            ? `${model} · ${t('Output Tokens')}: ${formatTokenUnits(summary.completion_tokens)} (${formatNumber(summary.completion_tokens, locale)}) · ${t('Average TPS')}: ${avgTpsLabel}`
            : model
          return (
            <Badge
              key={model}
              variant='outline'
              title={badgeTitle}
              data-avg-tps={avgTpsLabel}
              className='cursor-pointer transition-opacity hover:opacity-80'
              style={{
                opacity: isSelected ? 1 : 0.4,
                borderColor: isSelected && color ? color : undefined,
                color: isSelected && color ? color : undefined,
              }}
              onClick={() => onToggleModel(model)}
            >
              <span>{model}</span>
              {summary && sortField === 'avg_tps' && (
                <span className='ml-1 text-[10px] opacity-80'>
                  ({avgTpsLabel})
                </span>
              )}
            </Badge>
          )
        })}
      </div>
    </div>
  )
}
