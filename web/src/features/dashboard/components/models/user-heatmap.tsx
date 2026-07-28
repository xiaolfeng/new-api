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
import { useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Skeleton } from '@/components/ui/skeleton'
import { api } from '@/lib/api'
import dayjs from '@/lib/dayjs'
import { toIntlLocale } from '@/i18n/languages'

interface TokenRecordDailyItem {
  date: string
  prompt_tokens: number
  completion_tokens: number
  total_tokens: number
}

const LIGHT_COLORS = ['#ebedf0', '#9be9a8', '#40c463', '#30a14e', '#216e39']
const DARK_COLORS = ['#161b22', '#0e4429', '#006d32', '#26a641', '#39d353']

function useIsDark() {
  const [isDark, setIsDark] = useState(false)

  useEffect(() => {
    const check = () => {
      setIsDark(document.documentElement.classList.contains('dark'))
    }
    check()
    const observer = new MutationObserver(check)
    observer.observe(document.documentElement, {
      attributes: true,
      attributeFilter: ['class'],
    })
    return () => observer.disconnect()
  }, [])

  return isDark
}

function getColorLevel(
  totalTokens: number,
  thresholds: number[],
  isDark: boolean
): string {
  if (totalTokens <= 0) {
    return isDark ? DARK_COLORS[0] : LIGHT_COLORS[0]
  }
  for (let i = 0; i < thresholds.length; i++) {
    if (totalTokens <= thresholds[i]) {
      return isDark ? DARK_COLORS[i + 1] : LIGHT_COLORS[i + 1]
    }
  }
  return isDark ? DARK_COLORS[4] : LIGHT_COLORS[4]
}

function UserHeatmapDayLabels() {
  const { i18n } = useTranslation()
  const fmt = new Intl.DateTimeFormat(toIntlLocale(i18n.language), {
    weekday: 'short',
  })
  // 2024-01-01 = Monday, 2024-01-03 = Wednesday, 2024-01-05 = Friday
  const mon = fmt.format(new Date(2024, 0, 1))
  const wed = fmt.format(new Date(2024, 0, 3))
  const fri = fmt.format(new Date(2024, 0, 5))
  return (
    <div className='text-muted-foreground flex flex-col gap-[3px] pt-[18px] text-[10px] leading-none'>
      <span className='flex h-[12px] items-center'>{mon}</span>
      <span className='h-[12px]' />
      <span className='flex h-[12px] items-center'>{wed}</span>
      <span className='h-[12px]' />
      <span className='flex h-[12px] items-center'>{fri}</span>
      <span className='h-[12px]' />
    </div>
  )
}

export function UserHeatmap() {
  const { t, i18n } = useTranslation()
  const isDark = useIsDark()
  const [data, setData] = useState<TokenRecordDailyItem[]>([])
  const [loading, setLoading] = useState(true)
  const [hovered, setHovered] = useState<{
    date: string
    record: TokenRecordDailyItem | null
  } | null>(null)
  const [tooltipPos, setTooltipPos] = useState<{
    x: number
    y: number
    showBelow: boolean
  } | null>(null)

  useEffect(() => {
    let cancelled = false
    const fetchData = async () => {
      setLoading(true)
      try {
        const res = await api.get<{
          success: boolean
          data: TokenRecordDailyItem[]
        }>('/api/token_record/daily/self')
        if (!cancelled && res.data?.success !== false) {
          const raw = res.data?.data ?? []
          const normalized = raw.map((item) => ({
            ...item,
            date: item.date ? dayjs(item.date).format('YYYY-MM-DD') : item.date,
          }))
          setData(normalized)
        }
      } catch (err) {
        if (!cancelled) {
          console.warn('Failed to fetch user heatmap data', err)
        }
      } finally {
        if (!cancelled) setLoading(false)
      }
    }
    fetchData()
    return () => {
      cancelled = true
    }
  }, [])

  const dataMap = useMemo(() => {
    const map = new Map<string, TokenRecordDailyItem>()
    for (const item of data) {
      map.set(item.date, item)
    }
    return map
  }, [data])

  const totalTokens = useMemo(
    () => data.reduce((sum, item) => sum + (item.total_tokens || 0), 0),
    [data]
  )

  const { dates, thresholds, hasActivity, monthLabels } = useMemo(() => {
    const today = new Date()
    const allDates: string[] = []
    for (let i = 364; i >= 0; i--) {
      const d = new Date(today)
      d.setDate(d.getDate() - i)
      allDates.push(dayjs(d).format('YYYY-MM-DD'))
    }

    const nonZeroValues = data
      .filter((d) => d.total_tokens > 0)
      .map((d) => d.total_tokens)
      .sort((a, b) => a - b)

    let thresh: number[] = []
    if (nonZeroValues.length > 0) {
      const q1 =
        nonZeroValues[Math.floor(nonZeroValues.length * 0.25)] ??
        nonZeroValues[0]
      const q2 =
        nonZeroValues[Math.floor(nonZeroValues.length * 0.5)] ??
        nonZeroValues[0]
      const q3 =
        nonZeroValues[Math.floor(nonZeroValues.length * 0.75)] ??
        nonZeroValues[0]
      thresh = [q1, q2, q3]
    }

    const weeksCount = Math.ceil(allDates.length / 7)
    const labels: (string | null)[] = []
    const monthFormatter = new Intl.DateTimeFormat(toIntlLocale(i18n.language), {
      month: 'short',
    })
    for (let w = 0; w < weeksCount; w++) {
      const weekDates = allDates.slice(w * 7, w * 7 + 7)
      const monthStart = weekDates.find((dateStr) => {
        return dayjs(dateStr).date() === 1
      })
      if (monthStart) {
        labels.push(monthFormatter.format(dayjs(monthStart).toDate()))
      } else {
        labels.push(null)
      }
    }

    return {
      dates: allDates,
      thresholds: thresh,
      hasActivity: nonZeroValues.length > 0,
      monthLabels: labels,
    }
  }, [data, i18n.language])

  // Localized long date for tooltip header (e.g. "July 25, 2026" / "2026年7月25日")
  const longDateFormatter = new Intl.DateTimeFormat(
    toIntlLocale(i18n.language),
    { year: 'numeric', month: 'long', day: 'numeric' }
  )
  const formatLongDate = (dateStr: string) => {
    const d = dayjs(dateStr).toDate()
    return Number.isNaN(d.getTime()) ? dateStr : longDateFormatter.format(d)
  }

  const legendColors = isDark ? DARK_COLORS : LIGHT_COLORS

  const handleCellEnter = (date: string, e: React.MouseEvent<HTMLDivElement>) => {
    const record = dataMap.get(date) ?? null
    const rect = e.currentTarget.getBoundingClientRect()
    setHovered({ date, record })
    setTooltipPos({ x: rect.left + rect.width / 2, y: rect.top, showBelow: rect.top < 120 })
  }
  const handleCellLeave = () => {
    setHovered(null)
    setTooltipPos(null)
  }

  if (loading) {
    return (
      <div className='overflow-hidden rounded-lg border'>
        <div className='flex items-center justify-between border-b px-4 py-3 sm:px-5'>
          <Skeleton className='h-5 w-32' />
        </div>
        <div className='h-32 p-2 sm:p-4'>
          <Skeleton className='h-full w-full' />
        </div>
      </div>
    )
  }

  return (
    <div
      className='overflow-hidden rounded-lg border'
      data-testid='user-heatmap'
    >
      <div className='flex flex-wrap items-center justify-between gap-3 border-b px-4 py-3 sm:px-5'>
        <div className='flex items-baseline gap-2'>
          <h3 className='text-sm font-medium'>
            {t('dashboard.models.yourActivity')}
          </h3>
          <span className='text-muted-foreground text-xs'>
            {totalTokens.toLocaleString()} {t('tokens')}
          </span>
        </div>
        <div className='text-muted-foreground flex items-center gap-1.5 text-[10px]'>
          <span>{t('Less')}</span>
          {legendColors.map((c, i) => (
            <div
              key={i}
              className='h-[10px] w-[10px] rounded-[3px]'
              style={{ backgroundColor: c }}
            />
          ))}
          <span>{t('More')}</span>
        </div>
      </div>
      <div className='px-4 py-3 sm:px-5'>
        <div className='flex gap-2'>
          <UserHeatmapDayLabels />
          <div className='overflow-x-auto'>
            <div className='min-w-max'>
              <div className='text-muted-foreground mb-1 flex gap-[3px] text-[10px] leading-[12px]'>
                {monthLabels.map((label, i) => (
                  <span
                    key={i}
                    className='w-[12px] overflow-visible whitespace-nowrap'
                  >
                    {label ?? ''}
                  </span>
                ))}
              </div>
              <div className='grid grid-flow-col grid-rows-7 gap-[3px]'>
                {dates.map((date) => {
                  const record = dataMap.get(date)
                  const tokens = record?.total_tokens ?? 0
                  const bg = getColorLevel(tokens, thresholds, isDark)
                  return (
                    <div
                      key={date}
                      className='relative h-[12px] w-[12px] rounded-[3px] transition-[outline] duration-150 hover:z-10 hover:outline-1 hover:outline-foreground/40'
                      style={{ backgroundColor: bg }}
                      role='gridcell'
                      aria-label={`${date}: ${tokens.toLocaleString()} ${t('tokens')}`}
                      onMouseEnter={(e) => handleCellEnter(date, e)}
                      onMouseLeave={handleCellLeave}
                    />
                  )
                })}
              </div>
            </div>
          </div>
        </div>
        {!hasActivity && (
          <p className='text-muted-foreground mt-3 text-center text-xs'>
            {t('dashboard.models.noActivityYet')}
          </p>
        )}
      </div>

      {hovered && tooltipPos && (
        <div
          role='tooltip'
          className='bg-foreground text-background pointer-events-none fixed z-50 min-w-[160px] rounded-md px-3 py-2 text-xs shadow-lg'
          style={{
            left: tooltipPos.x,
            top: tooltipPos.showBelow ? tooltipPos.y + 14 : tooltipPos.y - 10,
            transform: tooltipPos.showBelow
              ? 'translate(-50%, 0)'
              : 'translate(-50%, -100%)',
          }}
        >
          <div className='font-medium'>{formatLongDate(hovered.date)}</div>
          {hovered.record && hovered.record.total_tokens > 0 ? (
            <div className='mt-1.5 space-y-1'>
              <div className='flex items-center justify-between gap-3'>
                <span className='text-background/60'>{t('Total')}</span>
                <span className='font-medium tabular-nums'>
                  {hovered.record.total_tokens.toLocaleString()} {t('tokens')}
                </span>
              </div>
              <div className='flex items-center justify-between gap-3'>
                <span className='text-background/60'>{t('Prompt')}</span>
                <span className='tabular-nums'>
                  {hovered.record.prompt_tokens.toLocaleString()}
                </span>
              </div>
              <div className='flex items-center justify-between gap-3'>
                <span className='text-background/60'>{t('Completion')}</span>
                <span className='tabular-nums'>
                  {hovered.record.completion_tokens.toLocaleString()}
                </span>
              </div>
            </div>
          ) : (
            <div className='text-background/60 mt-1'>
              {t('dashboard.models.noActivityYet')}
            </div>
          )}
        </div>
      )}
    </div>
  )
}
