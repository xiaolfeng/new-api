import React, { useState, useMemo, useEffect, useCallback } from 'react';
import {
  Button,
  Empty,
  Spin,
  Tag,
  Typography,
  Select,
} from '@douyinfe/semi-ui';
import { IconAscend, IconDescend, IconLineChartStroked, IconCoinMoneyStroked, IconLayers } from '@douyinfe/semi-icons';
import { initVChartSemiTheme } from '@visactor/vchart-semi-theme';
import CardPro from '../common/ui/CardPro';
import { renderNumber, modelToColor, modelColorMap } from '../../helpers/render';
import { useModelLogData } from '../../hooks/model-log/useModelLogData';
import ModelLogCharts from './ModelLogCharts';
import ModelLogModelFilter from './ModelLogModelFilter';

const { Text } = Typography;

// ========== 排序 ==========
const sortItems = (items, sortField, sortDirection) => {
  const sorted = [...items];
  const multiplier = sortDirection === 'asc' ? 1 : -1;

  sorted.sort((a, b) => {
    let aVal, bVal;

    switch (sortField) {
      case 'total_tokens':
        aVal = a.summary.completion_tokens || 0;
        bVal = b.summary.completion_tokens || 0;
        break;
      case 'failed_rate':
        aVal = a.summary.failed_rate || 0;
        bVal = b.summary.failed_rate || 0;
        break;
      case 'avg_tps':
        aVal = a.summary.avg_tps || 0;
        bVal = b.summary.avg_tps || 0;
        break;
      default:
        return 0;
    }

    if (aVal === bVal) {
      return a.model_name.localeCompare(b.model_name) * multiplier;
    }
    return (aVal - bVal) * multiplier;
  });

  return sorted;
};

// ========== 摘要卡片 ==========
const SUMMARY_CARD_CONFIG = [
  {
    key: 'total_request_count',
    label: '总请求次数',
    icon: <IconLineChartStroked />,
    valueFormatter: (v) => renderNumber(v || 0),
    tooltipFormatter: (v) => `${Number(v || 0).toLocaleString()} 次`,
    bgColor: 'bg-sky-50/70 dark:bg-sky-950/25',
    borderColor: 'border-sky-200/80 dark:border-sky-500/35',
    labelColor: 'text-sky-700/80 dark:text-sky-300/80',
    valueColor: 'text-sky-900 dark:text-sky-100',
  },
  {
    key: 'total_prompt_tokens',
    label: '输入 Token',
    icon: <IconCoinMoneyStroked />,
    valueFormatter: (v) => renderNumber(v || 0),
    tooltipFormatter: (v) => Number(v || 0).toLocaleString(),
    bgColor: 'bg-sky-50/70 dark:bg-sky-950/25',
    borderColor: 'border-sky-200/80 dark:border-sky-500/35',
    labelColor: 'text-sky-700/80 dark:text-sky-300/80',
    valueColor: 'text-sky-900 dark:text-sky-100',
  },
  {
    key: 'total_output_tokens',
    label: '输出 Token',
    icon: <IconCoinMoneyStroked />,
    valueFormatter: (v) => renderNumber(v || 0),
    tooltipFormatter: (v) => Number(v || 0).toLocaleString(),
    bgColor: 'bg-sky-50/70 dark:bg-sky-950/25',
    borderColor: 'border-sky-200/80 dark:border-sky-500/35',
    labelColor: 'text-sky-700/80 dark:text-sky-300/80',
    valueColor: 'text-sky-900 dark:text-sky-100',
  },
  {
    key: 'active_model_count',
    label: '活跃模型',
    icon: <IconLayers />,
    valueFormatter: (v) => String(v || 0),
    tooltipFormatter: (v) => `${v || 0} 个模型`,
    bgColor: 'bg-emerald-50/70 dark:bg-emerald-950/25',
    borderColor: 'border-emerald-200/80 dark:border-emerald-500/35',
    labelColor: 'text-emerald-700/80 dark:text-emerald-300/80',
    valueColor: 'text-emerald-900 dark:text-emerald-100',
  },
];

const buildSummaryCards = (summaryData, t) => {
  if (!summaryData) return null;

  return (
    <div className='grid grid-cols-2 gap-3 lg:grid-cols-4'>
      {SUMMARY_CARD_CONFIG.map((cfg) => {
        const rawValue = summaryData[cfg.key];
        const displayValue = cfg.valueFormatter(rawValue);

        return (
          <div
            key={cfg.key}
            className={`rounded-xl border p-3.5 transition-shadow hover:shadow-md ${cfg.bgColor} ${cfg.borderColor}`}
          >
            <div className='flex items-center justify-between'>
              <span className={`text-xs font-medium ${cfg.labelColor}`}>
                {t(cfg.label)}
              </span>
              <span className={`${cfg.labelColor} text-sm`}>{cfg.icon}</span>
            </div>
            <div title={cfg.tooltipFormatter(rawValue)} className={`mt-1 text-xl font-bold leading-tight ${cfg.valueColor}`}>
              {displayValue}
            </div>
          </div>
        );
      })}
    </div>
  );
};

// ========== 时间格式化 ==========
const formatTimeLabel = (bucketStartAt) => {
  const date = new Date(bucketStartAt * 1000);
  const month = `${date.getMonth() + 1}`.padStart(2, '0');
  const day = `${date.getDate()}`.padStart(2, '0');
  const hour = `${date.getHours()}`.padStart(2, '0');
  return `${month}-${day} ${hour}:00`;
};

// ========== 构建折线图 Spec ==========
const buildLineChartSpec = (dataId, dataValues, colorMap, t) => ({
  type: 'line',
  data: [{ id: dataId, values: dataValues }],
  xField: 'Time',
  yField: 'Value',
  seriesField: 'Model',
  legends: {
    visible: true,
  },
  tooltip: {
    mark: {
      content: [
        {
          key: (datum) => datum['Model'],
          value: (datum) => renderNumber(datum['Value']),
        },
      ],
    },
  },
  color: {
    specified: colorMap,
    fallback: (modelName) => modelToColor(modelName),
  },
  point: {
    visible: false,
  },
  crosshair: {
    visible: true,
  },
});

// ========== 主组件 ==========
const ModelLogBoard = () => {
  const { t, loading, refreshing, items, lastUpdatedAt, refreshData, summary } =
    useModelLogData();

  // 初始化 VChart Semi 主题
  useEffect(() => {
    initVChartSemiTheme({ isWatchingThemeSwitch: true });
  }, []);

  // 排序状态
  const [sortField, setSortField] = useState('total_tokens');
  const [sortDirection, setSortDirection] = useState('desc');

  // 图表 Tab 状态
  const [activeChartTab, setActiveChartTab] = useState('output_tokens');

  // 模型筛选状态
  const [selectedModels, setSelectedModels] = useState(null);

  // 排序后的模型列表
  const sortedItems = useMemo(
    () => sortItems(items, sortField, sortDirection),
    [items, sortField, sortDirection]
  );

  // 模型名称列表
  const modelNames = useMemo(
    () => sortedItems.map((item) => item.model_name),
    [sortedItems]
  );

  // 初始化 selectedModels（首次加载时全选）
  const effectiveSelected = useMemo(() => {
    if (selectedModels !== null) return selectedModels;
    return new Set(modelNames);
  }, [selectedModels, modelNames]);

  // 动态颜色映射
  const dynamicColorMap = useMemo(() => {
    const map = { ...modelColorMap };
    modelNames.forEach((name) => {
      if (!map[name]) {
        map[name] = modelToColor(name);
      }
    });
    return map;
  }, [modelNames]);

  // ========== 模型筛选操作 ==========
  const handleToggleModel = useCallback((model) => {
    setSelectedModels((prev) => {
      const base = prev || new Set(modelNames);
      const next = new Set(base);
      if (next.has(model)) {
        next.delete(model);
      } else {
        next.add(model);
      }
      return next;
    });
  }, [modelNames]);

  const handleSelectAll = useCallback(() => {
    setSelectedModels(new Set(modelNames));
  }, [modelNames]);

  const handleDeselectAll = useCallback(() => {
    setSelectedModels(new Set());
  }, []);

  // ========== 数据转换 ==========
  const transformedData = useMemo(() => {
    const outputTokenData = [];
    const tpsData = [];
    const failureRateData = [];

    sortedItems.forEach((item) => {
      if (!effectiveSelected.has(item.model_name)) return;

      (item.cells || []).forEach((cell) => {
        const timeLabel = formatTimeLabel(cell.bucket_start_at);
        const base = { Time: timeLabel, Model: item.model_name };

        // 输出 Token
        outputTokenData.push({
          ...base,
          Value: cell.completion_tokens || 0,
        });

        // 输出 TPS
        tpsData.push({
          ...base,
          Value: Number(Number(cell.avg_tps || 0).toFixed(2)),
        });

        // 失败率 (%)
        const totalRequests = (cell.request_count || 0) + (cell.failed_count || 0);
        const rate = totalRequests > 0
          ? Number(((cell.failed_count / totalRequests) * 100).toFixed(2))
          : 0;
        failureRateData.push({
          ...base,
          Value: rate,
        });
      });
    });

    return { outputTokenData, tpsData, failureRateData };
  }, [sortedItems, effectiveSelected]);

  // ========== Chart Specs ==========
  const outputTokenSpec = useMemo(
    () => buildLineChartSpec('outputTokenData', transformedData.outputTokenData, dynamicColorMap, t),
    [transformedData.outputTokenData, dynamicColorMap, t]
  );

  const tpsSpec = useMemo(
    () => buildLineChartSpec('tpsData', transformedData.tpsData, dynamicColorMap, t),
    [transformedData.tpsData, dynamicColorMap, t]
  );

  const failureRateSpec = useMemo(
    () => buildLineChartSpec('failureRateData', transformedData.failureRateData, dynamicColorMap, t),
    [transformedData.failureRateData, dynamicColorMap, t]
  );

  // ========== 排序操作 ==========
  const handleSort = (field) => {
    if (field === sortField) {
      setSortDirection((prev) => (prev === 'asc' ? 'desc' : 'asc'));
    } else {
      setSortField(field);
      setSortDirection('desc');
    }
  };

  return (
    <CardPro
      type='type2'
      statsArea={
        <div className='flex flex-col gap-4'>
          <div className='flex flex-col gap-3 md:flex-row md:items-center md:justify-between'>
            <div className='space-y-1'>
              <div className='text-lg font-semibold'>{t('模型日志')}</div>
              <Text type='secondary'>
                {t('展示最近 24 小时各模型的成功请求输出 Token 聚合、累计耗时与平均 TPS。')}
              </Text>
              {lastUpdatedAt > 0 && (
                <div className='text-xs text-[var(--semi-color-text-2)]'>
                  {t('最近刷新')}：
                  {new Date(lastUpdatedAt * 1000).toLocaleString()}
                </div>
              )}
            </div>
            <div className='flex items-center gap-2'>
              <Select
                value={sortField}
                onChange={(val) => handleSort(val)}
                size='small'
                style={{ width: 120 }}
              >
                <Select.Option value='total_tokens'>{t('输出 Token')}</Select.Option>
                <Select.Option value='failed_rate'>{t('失败率')}</Select.Option>
                <Select.Option value='avg_tps'>{t('输出 TPS')}</Select.Option>
              </Select>
              <Button
                size='small'
                icon={sortDirection === 'asc' ? <IconAscend /> : <IconDescend />}
                onClick={() => setSortDirection((prev) => (prev === 'asc' ? 'desc' : 'asc'))}
                theme={sortDirection === 'asc' ? 'light' : 'dark'}
              />
              <Tag color='blue' shape='circle'>
                {t('模型数')} {items.length}
              </Tag>
              <Button onClick={refreshData} loading={refreshing}>
                {t('刷新')}
              </Button>
            </div>
          </div>

          {!loading && summary && buildSummaryCards(summary, t)}
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
        <div className='space-y-3 pb-2'>
          <ModelLogModelFilter
            models={modelNames}
            selectedModels={effectiveSelected}
            onToggleModel={handleToggleModel}
            onSelectAll={handleSelectAll}
            onDeselectAll={handleDeselectAll}
            t={t}
          />
          <ModelLogCharts
            activeTab={activeChartTab}
            onTabChange={setActiveChartTab}
            outputTokenSpec={outputTokenSpec}
            tpsSpec={tpsSpec}
            failureRateSpec={failureRateSpec}
            t={t}
          />
        </div>
      )}
    </CardPro>
  );
};

export default ModelLogBoard;
