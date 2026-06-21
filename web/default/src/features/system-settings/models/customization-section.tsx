/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { useEffect, useRef, useState } from 'react'
import * as z from 'zod'
import type { Resolver } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { RotateCcw } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
} from '@/components/ui/form'
import { Switch } from '@/components/ui/switch'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { FormDirtyIndicator } from '../components/form-dirty-indicator'
import { FormNavigationGuard } from '../components/form-navigation-guard'
import { useSettingsForm } from '../hooks/use-settings-form'
import { useUpdateOption } from '../hooks/use-update-option'

const customizationSchema = z.object({
  global: z.object({
    responses_to_chat_completions_enabled: z.boolean(),
  }),
  bamboo: z.object({
    enable_bamboo_relay: z.boolean(),
    enable_bamboo_debug_log: z.boolean(),
    smooth_level: z
      .enum(['off', 'gentle', 'smooth', 'typewriter'])
      .optional(),
  }),
  retry_setting: z.object({
    record_consume_log_detail_enabled: z.boolean(),
    full_log_consume_enabled: z.boolean(),
    full_log_consume_expires_at: z.coerce.number(),
    full_log_consume_remaining_seconds: z.coerce.number(),
  }),
})

type CustomizationFormValues = z.infer<typeof customizationSchema>

type CustomizationSectionProps = {
  defaultValues: CustomizationFormValues
}

export function CustomizationSection({
  defaultValues,
}: CustomizationSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()

  const { form, handleSubmit, handleReset, isDirty, isSubmitting } =
    useSettingsForm<CustomizationFormValues>({
      resolver: zodResolver(customizationSchema) as Resolver<
        CustomizationFormValues,
        unknown,
        CustomizationFormValues
      >,
      defaultValues,
      onSubmit: async (_data, changedFields) => {
        const readonlyKeys = new Set([
          'retry_setting.full_log_consume_expires_at',
          'retry_setting.full_log_consume_remaining_seconds',
        ])
        for (const [key, value] of Object.entries(changedFields)) {
          if (readonlyKeys.has(key)) continue
          await updateOption.mutateAsync({
            key,
            value:
              typeof value === 'boolean' ? String(value) : String(value ?? ''),
          })
        }
      },
    })

  const [remainingSeconds, setRemainingSeconds] = useState(
    defaultValues.retry_setting.full_log_consume_remaining_seconds ?? 0
  )
  const fullLogEnabled =
    form.watch('retry_setting.full_log_consume_enabled') ?? false
  const expiresAt = form.watch('retry_setting.full_log_consume_expires_at') ?? 0
  const isFullLogActive = fullLogEnabled && remainingSeconds > 0

  const prevDefaultRemaining = useRef(remainingSeconds)
  useEffect(() => {
    const newRemaining =
      defaultValues.retry_setting.full_log_consume_remaining_seconds ?? 0
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
          form.reset({
            ...form.getValues(),
            retry_setting: {
              ...form.getValues().retry_setting,
              full_log_consume_enabled: false,
              full_log_consume_expires_at: 0,
              full_log_consume_remaining_seconds: 0,
            },
          })
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

  return (
    <>
      <FormNavigationGuard when={isDirty} />
      <Form {...form}>
        <form onSubmit={handleSubmit} className='space-y-6'>
          <FormDirtyIndicator isDirty={isDirty} />

          <FormField
            control={form.control}
            name='global.responses_to_chat_completions_enabled'
            render={({ field }) => (
              <FormItem className='flex flex-row items-center justify-between gap-4 rounded-lg border p-4'>
                <div className='space-y-0.5'>
                  <FormLabel className='text-base'>
                    {t('Convert Responses to Chat Completions')}
                  </FormLabel>
                  <FormDescription>
                    {t(
                      "When enabled, requests to the Responses API will be automatically converted to Chat Completions format for upstream providers that don't support the Responses API."
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
            name='bamboo.enable_bamboo_relay'
            render={({ field }) => (
              <FormItem className='flex flex-row items-center justify-between gap-4 rounded-lg border p-4'>
                <div className='space-y-0.5'>
                  <FormLabel className='text-base'>
                    {t('Bamboo Relay Bridge')}
                  </FormLabel>
                  <FormDescription>
                    {t(
                      'Enable protocol-agnostic relay: route chat requests through the bamboo-messages unified codec, so any inbound format (OpenAI / Claude / Gemini / Responses) can be freely converted to any upstream protocol. Unsupported upstreams automatically fall back to the native relay path.'
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

          {form.watch('bamboo.enable_bamboo_relay') && (
            <>
              <FormField
                control={form.control}
                name='bamboo.enable_bamboo_debug_log'
                render={({ field }) => (
                  <FormItem className='flex flex-row items-center justify-between gap-4 rounded-lg border border-amber-200 bg-amber-50 p-4 dark:border-amber-800 dark:bg-amber-950/50'>
                    <div className='space-y-0.5'>
                      <FormLabel className='text-base'>
                        {t('Bamboo Debug Log')}
                      </FormLabel>
                      <FormDescription>
                        {t(
                          'Output detailed debug logs for bamboo-messages provider layer, including upstream request headers and body (truncated). For development and debugging only — disable in production.'
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
                name='bamboo.smooth_level'
                render={({ field }) => (
                  <FormItem className='flex flex-row items-center justify-between gap-4 rounded-lg border p-4'>
                    <div className='space-y-0.5'>
                      <FormLabel className='text-base'>
                        {t('Streaming Smooth Strategy')}
                      </FormLabel>
                      <FormDescription>
                        {t(
                          'Controls the pacing of streamed SSE chunks to smooth out burst arrivals from upstream. Off disables buffering and passes events through directly.'
                        )}
                      </FormDescription>
                    </div>
                    <Select
                      value={field.value ?? 'off'}
                      onValueChange={field.onChange}
                    >
                      <FormControl>
                        <SelectTrigger className='w-48'>
                          <SelectValue />
                        </SelectTrigger>
                      </FormControl>
                      <SelectContent alignItemWithTrigger={false}>
                        <SelectItem value='off'>
                          {t('Off (direct passthrough)')}
                        </SelectItem>
                        <SelectItem value='gentle'>
                          {t('Gentle')}
                        </SelectItem>
                        <SelectItem value='smooth'>
                          {t('Smooth')}
                        </SelectItem>
                        <SelectItem value='typewriter'>
                          {t('Typewriter')}
                        </SelectItem>
                      </SelectContent>
                    </Select>
                  </FormItem>
                )}
              />
            </>
          )}

          <div className='space-y-4'>
            <FormField
              control={form.control}
              name='retry_setting.record_consume_log_detail_enabled'
              render={({ field }) => (
                <FormItem className='flex flex-row items-center justify-between gap-4 rounded-lg border p-4'>
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

            <FormField
              control={form.control}
              name='retry_setting.full_log_consume_enabled'
              render={({ field }) => (
                <FormItem className='flex flex-row items-center justify-between gap-4 rounded-lg border p-4'>
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

          <div className='flex flex-col gap-2 sm:flex-row'>
            <Button
              type='submit'
              disabled={isSubmitting || updateOption.isPending}
            >
              {isSubmitting || updateOption.isPending
                ? t('Saving...')
                : t('Save Changes')}
            </Button>
            <Button
              type='button'
              variant='outline'
              onClick={handleReset}
              disabled={!isDirty || isSubmitting || updateOption.isPending}
            >
              <RotateCcw className='size-4' />
              {t('Reset')}
            </Button>
          </div>
        </form>
      </Form>
    </>
  )
}
