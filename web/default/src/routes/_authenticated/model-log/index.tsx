import { createFileRoute } from '@tanstack/react-router'
import { ModelLogPage } from '@/features/model-log'

export const Route = createFileRoute('/_authenticated/model-log/')({
  component: ModelLogPage,
})
