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
import { api } from '@/lib/api'
import dayjs from '@/lib/dayjs'
import { Skeleton } from '@/components/ui/skeleton'

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
  const fmt = new Intl.DateTimeFormat(i18n.language, { weekday: 'short' })
  // 2024-01-01 = Monday, 2024-01-03 = Wednesday, 2024-01-05 = Friday
  const mon = fmt.format(new Date(2024, 0, 1))
  const wed = fmt.format(new Date(2024, 0, 3))
  const fri = fmt.format(new Date(2024, 0, 5))
  return (
    <div className='flex flex-col gap-[3px] pt-5 text-[10px] text-muted-foreground'>
      <span className='flex h-[11px] items-center'>{mon}</span>
      <span className='h-[11px]' />
      <span className='flex h-[11px] items-center'>{wed}</span>
      <span className='h-[11px]' />
      <span className='flex h-[11px] items-center'>{fri}</span>
      <span className='h-[11px]' />
    </div>
  )
}

export function UserHeatmap() {
  const { t, i18n } = useTranslation()
  const isDark = useIsDark()
  const [data, setData] = useState<TokenRecordDailyItem[]>([])
  const [loading, setLoading] = useState(true)

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
    const map = new Map<string, number>()
    for (const item of data) {
      map.set(item.date, item.total_tokens)
    }
    return map
  }, [data])

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
    const monthFormatter = new Intl.DateTimeFormat(i18n.language, { month: 'short' })
    for (let w = 0; w < weeksCount; w++) {
      const weekDates = allDates.slice(w * 7, w * 7 + 7)
      const monthStart = weekDates.find((dateStr) => {
        return dayjs(dateStr).date() === 1
      })
      if (monthStart) {
        labels.push(
          monthFormatter.format(dayjs(monthStart).toDate())
        )
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
      <div className='flex items-center justify-between border-b px-4 py-3 sm:px-5'>
        <h3 className='text-sm font-medium'>
          {t('dashboard.models.yourActivity')}
        </h3>
      </div>
      <div className='px-4 py-3 sm:px-5'>
        <div className='flex gap-2'>
          <UserHeatmapDayLabels />
          <div className='overflow-x-auto'>
            <div className='min-w-max'>
              <div className='mb-1 flex gap-[3px] text-[10px] text-muted-foreground'>
                {monthLabels.map((label, i) => (
                  <span
                    key={i}
                    className='w-[11px] overflow-visible whitespace-nowrap'
                  >
                    {label ?? ''}
                  </span>
                ))}
              </div>
              <div className='grid grid-flow-col grid-rows-7 gap-[3px]'>
                {dates.map((date) => {
                  const tokens = dataMap.get(date) ?? 0
                  const bg = getColorLevel(tokens, thresholds, isDark)
                  return (
                    <div
                      key={date}
                      className='h-[11px] w-[11px] rounded-sm'
                      style={{ backgroundColor: bg }}
                      title={`${date}: ${tokens.toLocaleString()} ${t('tokens')}`}
                    />
                  )
                })}
              </div>
            </div>
          </div>
        </div>
        {!hasActivity && (
          <p className='mt-3 text-center text-xs text-muted-foreground'>
            {t('dashboard.models.noActivityYet')}
          </p>
        )}
      </div>
    </div>
  )
}
