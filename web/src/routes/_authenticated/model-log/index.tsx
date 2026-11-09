import { createFileRoute } from '@tanstack/react-router'

import { ModelObservabilityPage } from '@/features/model-observability'

export const Route = createFileRoute('/_authenticated/model-log/')({
  component: ModelObservabilityPage,
})
