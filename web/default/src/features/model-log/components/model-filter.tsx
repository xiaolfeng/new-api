import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { stringToColor } from '@/lib/format'

interface ModelFilterProps {
  models: string[]
  selectedModels: Set<string>
  onToggleModel: (model: string) => void
  onSelectAll: () => void
  onDeselectAll: () => void
}

export function ModelFilter({
  models,
  selectedModels,
  onToggleModel,
  onSelectAll,
  onDeselectAll,
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
          return (
            <Badge
              key={model}
              variant='outline'
              className='cursor-pointer transition-opacity hover:opacity-80'
              style={{
                opacity: isSelected ? 1 : 0.4,
                borderColor: isSelected ? stringToColor(model) : undefined,
                color: isSelected ? stringToColor(model) : undefined,
              }}
              onClick={() => onToggleModel(model)}
            >
              {model}
            </Badge>
          )
        })}
      </div>
    </div>
  )
}
