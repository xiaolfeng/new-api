import * as z from 'zod'
import { useEffect, useRef, useState } from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import { SettingsSection } from '../components/settings-section'
import { useResetForm } from '../hooks/use-reset-form'
import { useUpdateOption } from '../hooks/use-update-option'

const retrySchema = z.object({
  'retry_setting.empty_response_retry_enabled': z.boolean(),
  'retry_setting.empty_response_retry_delay_seconds': z.coerce.number().min(0),
  'retry_setting.record_consume_log_detail_enabled': z.boolean(),
  'retry_setting.full_log_consume_enabled': z.boolean(),
  'retry_setting.full_log_consume_expires_at': z.coerce.number(),
  'retry_setting.full_log_consume_remaining_seconds': z.coerce.number(),
})

type RetryFormValues = z.infer<typeof retrySchema>

type EmptyResponseRetrySectionProps = {
  defaultValues: RetryFormValues
}

export function EmptyResponseRetrySection({
  defaultValues,
}: EmptyResponseRetrySectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()

  const form = useForm({
    resolver: zodResolver(retrySchema),
    defaultValues,
  })

  useResetForm(form, defaultValues)

  const [remainingSeconds, setRemainingSeconds] = useState(
    defaultValues['retry_setting.full_log_consume_remaining_seconds'] ?? 0
  )
  const expiresAt = form.watch('retry_setting.full_log_consume_expires_at') ?? 0
  const fullLogEnabled = form.watch('retry_setting.full_log_consume_enabled') ?? false
  const isFullLogActive = fullLogEnabled && remainingSeconds > 0

  const prevDefaultRemaining = useRef(remainingSeconds)
  useEffect(() => {
    const newRemaining = defaultValues['retry_setting.full_log_consume_remaining_seconds'] ?? 0
    if (newRemaining !== prevDefaultRemaining.current) {
      setRemainingSeconds(newRemaining)
      prevDefaultRemaining.current = newRemaining
    }
  }, [defaultValues])

  useEffect(() => {
    if (!fullLogEnabled || remainingSeconds <= 0) {
      return undefined
    }

    const timer = window.setInterval(() => {
      setRemainingSeconds((prev) => {
        const next = prev - 1
        if (next <= 0) {
          const resetValues = {
            ...form.getValues(),
            'retry_setting.full_log_consume_enabled': false,
            'retry_setting.full_log_consume_expires_at': 0,
            'retry_setting.full_log_consume_remaining_seconds': 0,
          }
          form.reset(resetValues)
          return 0
        }
        return next
      })
    }, 1000)

    return () => window.clearInterval(timer)
  }, [fullLogEnabled, remainingSeconds, form])

  const formatExpireTime = (timestamp: number) => {
    if (!timestamp) return '-'
    return new Date(timestamp * 1000).toLocaleString()
  }

  const onSubmit = async (data: RetryFormValues) => {
    const excludedKeys: (keyof RetryFormValues)[] = [
      'retry_setting.full_log_consume_expires_at',
      'retry_setting.full_log_consume_remaining_seconds',
    ]

    const updates = Object.entries(data).filter(
      ([key, value]) =>
        !excludedKeys.includes(key as keyof RetryFormValues) &&
        value !== defaultValues[key as keyof RetryFormValues]
    )

    for (const [key, value] of updates) {
      await updateOption.mutateAsync({
        key,
        value: typeof value === 'boolean' ? String(value) : value,
      })
    }
  }

  return (
    <SettingsSection
      title={t('Empty Response Retry')}
      description={t('Configure empty response retry and logging behavior')}
    >
      <Form {...form}>
        <form onSubmit={form.handleSubmit(onSubmit)} className='space-y-6'>
          <div className='space-y-4'>
            <FormField
              control={form.control}
              name='retry_setting.empty_response_retry_enabled'
              render={({ field }) => (
                <FormItem className='flex flex-row items-center justify-between rounded-lg border p-4'>
                  <div className='space-y-0.5'>
                    <FormLabel className='text-base'>
                      {t('Enable Empty Response Retry')}
                    </FormLabel>
                    <FormDescription>
                      {t(
                        'Automatically retry when upstream returns HTTP 2xx but response content is empty (completion_tokens=0)'
                      )}
                    </FormDescription>
                  </div>
                  <FormControl>
                    <Switch
                      checked={field.value}
                      onCheckedChange={field.onChange}
                    />
                  </FormControl>
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='retry_setting.empty_response_retry_delay_seconds'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Retry Delay (seconds)')}</FormLabel>
                  <FormControl>
                    <Input
                      type='number'
                      min='0'
                      step='1'
                      value={field.value as number}
                      onChange={(e) => field.onChange(e.target.valueAsNumber)}
                      name={field.name}
                      onBlur={field.onBlur}
                      ref={field.ref}
                    />
                  </FormControl>
                  <FormDescription>
                    {t(
                      'Seconds to wait before retrying empty responses, 0 for immediate retry'
                    )}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
          </div>

          <FormField
            control={form.control}
            name='retry_setting.record_consume_log_detail_enabled'
            render={({ field }) => (
              <FormItem className='flex flex-row items-center justify-between rounded-lg border p-4'>
                <div className='space-y-0.5'>
                  <FormLabel className='text-base'>
                    {t('Enable Record Logging')}
                  </FormLabel>
                  <FormDescription>
                    {t(
                      'Record summary request content, response content, tool calls, and filtered HTTP headers'
                    )}
                  </FormDescription>
                </div>
                <FormControl>
                  <Switch
                    checked={field.value}
                    onCheckedChange={field.onChange}
                  />
                </FormControl>
              </FormItem>
            )}
          />

          <div className='space-y-4'>
            <FormField
              control={form.control}
              name='retry_setting.full_log_consume_enabled'
              render={({ field }) => (
                <FormItem className='flex flex-row items-center justify-between rounded-lg border p-4'>
                  <div className='space-y-0.5'>
                    <FormLabel className='text-base'>
                      {t('Enable 5-Minute Full Logging')}
                    </FormLabel>
                    <FormDescription>
                      {t(
                        'Fully record request content, response content, and HTTP headers (excluding sensitive info), only allowed for 5 minutes'
                      )}
                    </FormDescription>
                  </div>
                  <FormControl>
                    <Switch
                      checked={field.value}
                      onCheckedChange={field.onChange}
                    />
                  </FormControl>
                </FormItem>
              )}
            />

            {isFullLogActive && (
              <div className='rounded-lg border border-green-200 bg-green-50 p-3 dark:border-green-800 dark:bg-green-950'>
                <p className='text-sm text-green-700 dark:text-green-400'>
                  {t('Full logging remaining {{count}} seconds', {
                    count: remainingSeconds,
                  })}
                </p>
                <p className='text-muted-foreground mt-1 text-xs'>
                  {t('Expires at')}: {formatExpireTime(expiresAt)}
                </p>
              </div>
            )}
          </div>

          <Button type='submit' disabled={updateOption.isPending}>
            {updateOption.isPending ? t('Saving...') : t('Save Changes')}
          </Button>
        </form>
      </Form>
    </SettingsSection>
  )
}
