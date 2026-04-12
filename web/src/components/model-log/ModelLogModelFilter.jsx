import React from 'react';
import { Tag, Button } from '@douyinfe/semi-ui';
import { modelToColor } from '../../helpers/render';

const ModelLogModelFilter = ({ models, selectedModels, onToggleModel, onSelectAll, onDeselectAll, t }) => (
  <div className='flex flex-wrap items-center gap-2 py-2'>
    <span className='text-xs text-[var(--semi-color-text-2)]'>{t('模型筛选')}：</span>
    <Button size='small' onClick={onSelectAll}>
      {t('全选')}
    </Button>
    <Button size='small' onClick={onDeselectAll}>
      {t('清空')}
    </Button>
    <div className='flex flex-wrap gap-1.5'>
      {models.map((model) => {
        const isSelected = selectedModels.has(model);
        return (
          <Tag
            key={model}
            color={isSelected ? modelToColor(model) : 'grey'}
            onClick={() => onToggleModel(model)}
            style={{
              opacity: isSelected ? 1 : 0.4,
              cursor: 'pointer',
              transition: 'opacity 0.2s',
            }}
          >
            {model}
          </Tag>
        );
      })}
    </div>
  </div>
);

export default ModelLogModelFilter;
