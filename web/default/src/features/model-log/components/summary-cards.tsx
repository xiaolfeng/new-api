import { useTranslation } from 'react-i18next'
import {
  Activity,
  ArrowDownToLine,
  ArrowUpFromLine,
  Layers,
} from 'lucide-react'
import { formatCompactNumber } from '@/lib/format'
import type { TokenRecordOverallSummary } from '../types'

interface SummaryCardsProps {
  summary: TokenRecordOverallSummary
}

const CARD_STYLES = [
  {
    bg: 'bg-sky-50/70 dark:bg-sky-950/25',
    border: 'border-sky-200/80 dark:border-sky-500/35',
    label: 'text-sky-700/80 dark:text-sky-300/80',
    value: 'text-sky-900 dark:text-sky-100',
  },
  {
    bg: 'bg-sky-50/70 dark:bg-sky-950/25',
    border: 'border-sky-200/80 dark:border-sky-500/35',
    label: 'text-sky-700/80 dark:text-sky-300/80',
    value: 'text-sky-900 dark:text-sky-100',
  },
  {
    bg: 'bg-sky-50/70 dark:bg-sky-950/25',
    border: 'border-sky-200/80 dark:border-sky-500/35',
    label: 'text-sky-700/80 dark:text-sky-300/80',
    value: 'text-sky-900 dark:text-sky-100',
  },
  {
    bg: 'bg-emerald-50/70 dark:bg-emerald-950/25',
    border: 'border-emerald-200/80 dark:border-emerald-500/35',
    label: 'text-emerald-700/80 dark:text-emerald-300/80',
    value: 'text-emerald-900 dark:text-emerald-100',
  },
]

export function SummaryCards({ summary }: SummaryCardsProps) {
  const { t } = useTranslation()

  const cards = [
    {
      key: 'total_request_count',
      label: t('Total Requests'),
      value: formatCompactNumber(summary.total_request_count || 0),
      tooltip: `${(summary.total_request_count || 0).toLocaleString()} ${t('times')}`,
      icon: Activity,
    },
    {
      key: 'total_prompt_tokens',
      label: t('Input Tokens'),
      value: formatCompactNumber(summary.total_prompt_tokens || 0),
      tooltip: (summary.total_prompt_tokens || 0).toLocaleString(),
      icon: ArrowDownToLine,
    },
    {
      key: 'total_output_tokens',
      label: t('Output Tokens'),
      value: formatCompactNumber(summary.total_output_tokens || 0),
      tooltip: (summary.total_output_tokens || 0).toLocaleString(),
      icon: ArrowUpFromLine,
    },
    {
      key: 'active_model_count',
      label: t('Active Models'),
      value: String(summary.active_model_count || 0),
      tooltip: `${summary.active_model_count || 0} ${t('models')}`,
      icon: Layers,
    },
  ]

  return (
    <div className='grid grid-cols-2 gap-3 lg:grid-cols-4'>
      {cards.map((card, idx) => {
        const Icon = card.icon
        const style = CARD_STYLES[idx]
        return (
          <div
            key={card.key}
            className={`rounded-xl border p-3.5 transition-shadow hover:shadow-md ${style.bg} ${style.border}`}
          >
            <div className='flex items-center justify-between'>
              <span className={`text-xs font-medium ${style.label}`}>
                {card.label}
              </span>
              <Icon className={`size-4 ${style.label}`} />
            </div>
            <div
              title={card.tooltip}
              className={`mt-1 text-xl font-bold leading-tight ${style.value}`}
            >
              {card.value}
            </div>
          </div>
        )
      })}
    </div>
  )
}
