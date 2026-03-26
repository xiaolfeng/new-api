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
  const backgroundAlpha = ratio > 0 ? 0.12 + ratio * 0.72 : 0.04;

  return {
    background:
      cell.total_tokens > 0
        ? `rgba(15, 118, 110, ${backgroundAlpha})`
        : 'var(--semi-color-fill-0)',
    border: cell.is_current
      ? '2px solid var(--semi-color-primary)'
      : '1px solid var(--semi-color-border)',
    color: ratio >= 0.55 ? '#ffffff' : 'var(--semi-color-text-0)',
  };
};

const buildCellTooltip = (cell) => (
  <div className='min-w-[220px] space-y-1 text-sm'>
    <div className='font-semibold'>
      {formatHourRange(cell.bucket_start_at, cell.bucket_end_at)}
    </div>
    <div>总 Token：{renderNumber(cell.total_tokens || 0)}</div>
    <div>输入 Token：{renderNumber(cell.prompt_tokens || 0)}</div>
    <div>输出 Token：{renderNumber(cell.completion_tokens || 0)}</div>
    <div>成功请求：{renderNumber(cell.request_count || 0)}</div>
    <div>累计耗时：{renderNumber(cell.total_use_time || 0)} 秒</div>
    <div>平均 TPS：{formatAvgTps(cell.avg_tps)}</div>
  </div>
);

const ModelLogBoard = () => {
  const { t, loading, refreshing, hours, items, lastUpdatedAt, refreshData } =
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
                '展示最近 24 小时各模型的成功请求 Token 聚合、累计耗时与平均 TPS。',
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
        <div className='overflow-x-auto pb-2'>
          <div className='min-w-[1360px] space-y-4'>
            <div className='flex gap-3'>
              <div className='w-[280px] shrink-0 rounded-2xl border border-dashed border-[var(--semi-color-border)] bg-[var(--semi-color-fill-0)] p-3'>
                <div className='text-sm font-semibold'>{t('模型')}</div>
                <div className='mt-1 text-xs text-[var(--semi-color-text-2)]'>
                  {t('左侧显示 24 小时汇总，右侧 24 格代表最近 24 个小时。')}
                </div>
              </div>
              <div className='flex gap-2'>
                {hours.map((hour) => (
                  <div
                    key={hour.bucket_start_at}
                    className='flex h-12 w-12 shrink-0 items-center justify-center rounded-xl border text-xs font-medium'
                    style={{
                      borderColor: hour.is_current
                        ? 'var(--semi-color-primary)'
                        : 'var(--semi-color-border)',
                      color: hour.is_current
                        ? 'var(--semi-color-primary)'
                        : 'var(--semi-color-text-1)',
                      background: hour.is_current
                        ? 'rgba(34, 197, 94, 0.08)'
                        : 'var(--semi-color-fill-0)',
                    }}
                  >
                    {hour.label}
                  </div>
                ))}
              </div>
            </div>

            {items.map((item) => {
              const rowMaxTokens = Math.max(
                ...item.cells.map((cell) => cell.total_tokens || 0),
                0,
              );

              return (
                <div key={item.model_name} className='flex gap-3'>
                  <Card
                    className='w-[280px] shrink-0 !rounded-2xl'
                    bordered
                    bodyStyle={{ padding: 14 }}
                  >
                    <div className='space-y-3'>
                      <div className='break-all text-sm font-semibold'>
                        {item.model_name}
                      </div>
                      <div className='grid grid-cols-2 gap-2 text-xs'>
                        <div className='rounded-xl bg-[var(--semi-color-fill-0)] p-2'>
                          <div className='text-[var(--semi-color-text-2)]'>
                            {t('总 Token')}
                          </div>
                          <div className='mt-1 text-sm font-semibold'>
                            {renderNumber(item.summary.total_tokens || 0)}
                          </div>
                        </div>
                        <div className='rounded-xl bg-[var(--semi-color-fill-0)] p-2'>
                          <div className='text-[var(--semi-color-text-2)]'>
                            {t('成功请求')}
                          </div>
                          <div className='mt-1 text-sm font-semibold'>
                            {renderNumber(item.summary.request_count || 0)}
                          </div>
                        </div>
                        <div className='rounded-xl bg-[var(--semi-color-fill-0)] p-2'>
                          <div className='text-[var(--semi-color-text-2)]'>
                            {t('平均 TPS')}
                          </div>
                          <div className='mt-1 text-sm font-semibold'>
                            {formatAvgTps(item.summary.avg_tps)}
                          </div>
                        </div>
                        <div className='rounded-xl bg-[var(--semi-color-fill-0)] p-2'>
                          <div className='text-[var(--semi-color-text-2)]'>
                            {t('累计耗时')}
                          </div>
                          <div className='mt-1 text-sm font-semibold'>
                            {renderNumber(item.summary.total_use_time || 0)}s
                          </div>
                        </div>
                      </div>
                    </div>
                  </Card>

                  <div className='flex gap-2'>
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
                            className='flex h-14 w-12 shrink-0 flex-col items-center justify-center rounded-xl text-[11px] font-semibold transition-transform hover:-translate-y-0.5'
                            style={cellStyle}
                          >
                            <span>
                              {cell.total_tokens > 0
                                ? renderNumber(cell.total_tokens)
                                : '-'}
                            </span>
                          </button>
                        </Tooltip>
                      );
                    })}
                  </div>
                </div>
              );
            })}
          </div>
        </div>
      )}
    </CardPro>
  );
};

export default ModelLogBoard;
