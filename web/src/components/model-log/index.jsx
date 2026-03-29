import React from 'react';
import {
  Button,
  Card,
  Empty,
  Spin,
  Tag,
  Tooltip,
  Typography,
} from '@douyinfe/semi-ui';
import CardPro from '../common/ui/CardPro';
import { renderNumber } from '../../helpers';
import { useModelLogData } from '../../hooks/model-log/useModelLogData';

const { Text } = Typography;

const formatHourRange = (bucketStartAt, bucketEndAt) => {
  const startDate = new Date(bucketStartAt * 1000);
  const endDate = new Date((bucketEndAt + 1) * 1000);

  const formatPart = (date, withDate = true) => {
    const month = `${date.getMonth() + 1}`.padStart(2, '0');
    const day = `${date.getDate()}`.padStart(2, '0');
    const hour = `${date.getHours()}`.padStart(2, '0');
    const minute = `${date.getMinutes()}`.padStart(2, '0');
    if (!withDate) {
      return `${hour}:${minute}`;
    }
    return `${month}-${day} ${hour}:${minute}`;
  };

  return `${formatPart(startDate)} - ${formatPart(endDate, false)}`;
};

const formatAvgTps = (avgTps) => {
  if (!avgTps) {
    return '0';
  }
  return Number(avgTps).toFixed(2);
};

const getCellStyle = (cell, maxTokens) => {
  const ratio =
    maxTokens > 0 && cell.total_tokens > 0 ? cell.total_tokens / maxTokens : 0;
  const backgroundAlpha = ratio > 0 ? 0.18 + ratio * 0.72 : 0.08;

  return {
    background:
      cell.total_tokens > 0
        ? `rgba(59, 130, 246, ${backgroundAlpha})`
        : 'var(--semi-color-fill-1)',
    border: cell.is_current
      ? '2px solid var(--semi-color-info)'
      : '1px solid var(--semi-color-border)',
    color: ratio >= 0.55 ? '#ffffff' : 'var(--semi-color-text-0)',
  };
};

const getCellStyleForTps = (cell, maxTps) => {
  const ratio = maxTps > 0 && cell.avg_tps > 0 ? cell.avg_tps / maxTps : 0;
  const backgroundAlpha = ratio > 0 ? 0.18 + ratio * 0.72 : 0.08;

  return {
    background:
      cell.avg_tps > 0
        ? `rgba(34, 197, 94, ${backgroundAlpha})`
        : 'var(--semi-color-fill-1)',
    border: cell.is_current
      ? '2px solid var(--semi-color-success)'
      : '1px solid var(--semi-color-border)',
    color: ratio >= 0.55 ? '#ffffff' : 'var(--semi-color-text-0)',
  };
};

const getCellAriaLabel = (cell) =>
  `hour-${cell.bucket_start_at}-tokens-${cell.total_tokens || 0}`;

const buildCellTooltip = (cell) => (
  <div className='min-w-[220px] space-y-1 text-sm'>
    <div className='font-semibold'>
      {formatHourRange(cell.bucket_start_at, cell.bucket_end_at)}
    </div>
    <div>输出 Token：{renderNumber(cell.completion_tokens || 0)}</div>
    <div>成功请求：{renderNumber(cell.request_count || 0)}</div>
    <div>累计耗时：{renderNumber(cell.total_use_time || 0)} 秒</div>
    <div>平均 TPS：{formatAvgTps(cell.avg_tps)}</div>
  </div>
);

const buildTpsCellTooltip = (cell) => (
  <div className='min-w-[220px] space-y-1 text-sm'>
    <div className='font-semibold'>
      {formatHourRange(cell.bucket_start_at, cell.bucket_end_at)}
    </div>
    <div>平均 TPS：{formatAvgTps(cell.avg_tps)}</div>
    <div>输出 Token：{renderNumber(cell.completion_tokens || 0)}</div>
    <div>成功请求：{renderNumber(cell.request_count || 0)}</div>
  </div>
);

const buildHeaderStats = (item, t) => [
  {
    label: t('输出 Token'),
    value: renderNumber(item.summary.total_tokens || 0),
  },
  {
    label: t('成功请求'),
    value: renderNumber(item.summary.request_count || 0),
  },
  {
    label: t('平均 TPS'),
    value: formatAvgTps(item.summary.avg_tps),
  },
  {
    label: t('累计耗时'),
    value: `${renderNumber(item.summary.total_use_time || 0)}s`,
  },
];

const ModelLogBoard = () => {
  const { t, loading, refreshing, items, lastUpdatedAt, refreshData } =
    useModelLogData();

  return (
    <CardPro
      type='type2'
      statsArea={
        <div className='flex flex-col gap-3 md:flex-row md:items-center md:justify-between'>
          <div className='space-y-1'>
            <div className='text-lg font-semibold'>{t('模型日志')}</div>
            <Text type='secondary'>
              {t(
                '展示最近 24 小时各模型的成功请求输出 Token 聚合、累计耗时与平均 TPS。',
              )}
            </Text>
            {lastUpdatedAt > 0 && (
              <div className='text-xs text-[var(--semi-color-text-2)]'>
                {t('最近刷新')}：
                {new Date(lastUpdatedAt * 1000).toLocaleString()}
              </div>
            )}
          </div>
          <div className='flex items-center gap-2'>
            <Tag color='blue' shape='circle'>
              {t('模型数')} {items.length}
            </Tag>
            <Button onClick={refreshData} loading={refreshing}>
              {t('刷新')}
            </Button>
          </div>
        </div>
      }
      t={t}
    >
      {loading ? (
        <div className='flex justify-center py-16'>
          <Spin size='large' />
        </div>
      ) : items.length === 0 ? (
        <div className='py-16'>
          <Empty description={t('最近 24 小时暂无模型日志数据')} />
        </div>
      ) : (
        <div className='space-y-5 pb-2'>
          <div className='space-y-1 rounded-2xl border border-dashed border-sky-200/80 bg-sky-50/70 p-3 dark:border-sky-500/35 dark:bg-sky-950/25'>
            <div className='text-sm font-semibold'>{t('模型')}</div>
            <div className='text-xs text-sky-700/80 dark:text-sky-300/70'>
              {t(
                '每个模型显示最近 24 小时的 24 个格子，颜色越深表示该小时输出 Token 用量越高。',
              )}
            </div>
          </div>

          <div className='space-y-1'>
            {items.map((item) => {
              const rowMaxTokens = Math.max(
                ...item.cells.map((cell) => cell.total_tokens || 0),
                0,
              );
              const rowMaxTps = Math.max(
                ...item.cells.map((cell) => cell.avg_tps || 0),
                0,
              );

              return (
                <Card
                  key={item.model_name}
                  className='w-full !rounded-2xl border border-sky-200/80 bg-gradient-to-br from-sky-50 to-sky-100/70 dark:border-sky-500/35 dark:from-sky-950/35 dark:to-sky-900/20'
                  bordered
                  bodyStyle={{ padding: 12 }}
                >
                  <div className='flex flex-col space-y-4'>
                    <div className='flex flex-wrap items-start justify-between gap-2'>
                      <div className='min-w-0 flex-1'>
                        <div className='break-all text-sm font-semibold leading-5 text-sky-900 dark:text-sky-100'>
                          {item.model_name}
                        </div>
                      </div>
                      <div className='flex flex-wrap items-center justify-end gap-2 text-right'>
                        {buildHeaderStats(item, t).map((stat) => (
                          <div
                            key={`${item.model_name}-${stat.label}`}
                            className='rounded-lg border border-sky-200/80 bg-sky-100/55 px-2 py-1 dark:border-sky-500/30 dark:bg-sky-900/35'
                          >
                            <div className='text-[11px] leading-4 text-sky-700/80 dark:text-sky-300/80'>
                              {stat.label}
                            </div>
                            <div className='text-xs font-semibold leading-4 text-sky-900 dark:text-sky-100'>
                              {stat.value}
                            </div>
                          </div>
                        ))}
                      </div>
                    </div>

                    <div className='flex flex-wrap gap-1.5 sm:gap-2'>
                      {item.cells.map((cell) => {
                        const cellStyle = getCellStyle(cell, rowMaxTokens);
                        return (
                          <Tooltip
                            key={`${item.model_name}-${cell.bucket_start_at}`}
                            content={buildCellTooltip(cell)}
                            position='top'
                          >
                            <button
                              type='button'
                              aria-label={getCellAriaLabel(cell)}
                              className='h-4 w-4 shrink-0 rounded-[5px] transition-transform hover:-translate-y-0.5 sm:h-[18px] sm:w-[18px] md:h-5 md:w-5'
                              style={cellStyle}
                            ></button>
                          </Tooltip>
                        );
                      })}
                    </div>

                    <div className='flex flex-wrap gap-1.5 sm:gap-2'>
                      {item.cells.map((cell) => {
                        const cellStyle = getCellStyleForTps(cell, rowMaxTps);
                        return (
                          <Tooltip
                            key={`tps-${item.model_name}-${cell.bucket_start_at}`}
                            content={buildTpsCellTooltip(cell)}
                            position='top'
                          >
                            <button
                              type='button'
                              aria-label={`tps-${cell.bucket_start_at}-${cell.avg_tps || 0}`}
                              className='h-4 w-4 shrink-0 rounded-[5px] transition-transform hover:-translate-y-0.5 sm:h-[18px] sm:w-[18px] md:h-5 md:w-5'
                              style={cellStyle}
                            ></button>
                          </Tooltip>
                        );
                      })}
                    </div>
                  </div>
                </Card>
              );
            })}
          </div>
        </div>
      )}
    </CardPro>
  );
};

export default ModelLogBoard;
