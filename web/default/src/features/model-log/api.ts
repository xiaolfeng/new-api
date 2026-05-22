import { api } from '@/lib/api'
import type { TokenRecordRecentSnapshot } from './types'

export async function getTokenRecordRecent(): Promise<{
  success: boolean
  message?: string
  data?: TokenRecordRecentSnapshot
}> {
  const res = await api.get('/api/token_record/recent', {
    disableDuplicate: true,
  } as Record<string, unknown>)
  return res.data
}
