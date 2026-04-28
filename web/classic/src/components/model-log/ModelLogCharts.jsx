import React from 'react';
import { Tabs, TabPane } from '@douyinfe/semi-ui';
import { VChart } from '@visactor/react-vchart';
import { CHART_CONFIG } from '../../constants/dashboard.constants';

const ModelLogCharts = ({ activeTab, onTabChange, outputTokenSpec, tpsSpec, failureRateSpec, t }) => (
  <div>
    <Tabs
      type='slash'
      activeKey={activeTab}
      onChange={onTabChange}
    >
      <TabPane tab={t('输出 Token 趋势')} itemKey='output_tokens' />
      <TabPane tab={t('输出 TPS 趋势')} itemKey='tps' />
      <TabPane tab={t('失败率趋势')} itemKey='failure_rate' />
    </Tabs>
    <div className='h-96 p-2'>
      {activeTab === 'output_tokens' && outputTokenSpec && (
        <VChart spec={outputTokenSpec} option={CHART_CONFIG} />
      )}
      {activeTab === 'tps' && tpsSpec && (
        <VChart spec={tpsSpec} option={CHART_CONFIG} />
      )}
      {activeTab === 'failure_rate' && failureRateSpec && (
        <VChart spec={failureRateSpec} option={CHART_CONFIG} />
      )}
    </div>
  </div>
);

export default ModelLogCharts;
