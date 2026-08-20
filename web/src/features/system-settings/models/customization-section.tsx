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
import { zodResolver } from '@hookform/resolvers/zod'
import { RotateCcw } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'
import type { Resolver } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import * as z from 'zod'

import { Button } from '@/components/ui/button'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'

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
    degraded_reason: z.enum(['stop', 'tool_use']).optional(),
    enable_host_tools: z.boolean(),
    host_tool_mode: z.enum(['loop', 'return']),
    search_backend: z.enum(['off', 'exa', 'parallel', 'searxng']),
    allow_third_party_search_egress: z.boolean(),
    search_fallback: z.string(),
    searxng_base_url: z.string(),
    exa_mcp_url: z.string(),
    parallel_mcp_url: z.string(),
    exa_api_key: z.string(),
    parallel_api_key: z.string(),
    max_search_results: z.coerce.number().int().min(1).max(20),
    max_fetch_bytes: z.coerce.number().int().min(1),
    host_tool_timeout_ms: z.coerce.number().int().min(1).max(30000),
    enable_image_recognize: z.boolean(),
    image_recognize_channel_id: z.coerce.number().int().min(0),
    image_recognize_model: z.string(),
    image_recognize_prompt: z.string(),
    image_recognize_max_images: z.coerce.number().int().min(1).max(8),
    image_recognize_timeout_ms: z.coerce.number().int().min(1).max(60000),
    image_recognize_retry_times: z.coerce.number().int().min(0).max(5),
    image_recognize_fail_open: z.boolean(),
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
        const secretKeys = new Set([
          'bamboo.exa_api_key',
          'bamboo.parallel_api_key',
        ])
        for (const [key, value] of Object.entries(changedFields)) {
          if (readonlyKeys.has(key)) continue
          if (secretKeys.has(key) && !String(value ?? '').trim()) continue
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
                name='bamboo.degraded_reason'
                render={({ field }) => (
                  <FormItem className='flex flex-row items-center justify-between gap-4 rounded-lg border p-4'>
                    <div className='space-y-0.5'>
                      <FormLabel className='text-base'>
                        {t('Stream Interrupt Degraded Reason')}
                      </FormLabel>
                      <FormDescription>
                        {t(
                          'Completion reason to synthesize when an upstream connection drops mid-stream after content has been delivered. "Stop" (default) returns a normal stop; "Tool Use" returns tool_calls when the request carries tools, keeping a ReAct agent loop alive across upstream disconnects.'
                        )}
                      </FormDescription>
                    </div>
                    <Select
                      value={field.value ?? 'stop'}
                      onValueChange={field.onChange}
                    >
                      <FormControl>
                        <SelectTrigger className='w-48'>
                          <SelectValue />
                        </SelectTrigger>
                      </FormControl>
                      <SelectContent alignItemWithTrigger={false}>
                        <SelectItem value='stop'>
                          {t('Stop (default)')}
                        </SelectItem>
                        <SelectItem value='tool_use'>
                          {t('Tool Use (agent loop)')}
                        </SelectItem>
                      </SelectContent>
                    </Select>
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='bamboo.enable_host_tools'
                render={({ field }) => (
                  <FormItem className='flex flex-row items-center justify-between gap-4 rounded-lg border p-4'>
                    <div className='space-y-0.5'>
                      <FormLabel className='text-base'>
                        {t('Bamboo Host Tools')}
                      </FormLabel>
                      <FormDescription>
                        {t(
                          'Intercept WebSearch / WebFetch on the bamboo relay path, execute them on this gateway, and let the same upstream model synthesize the final answer. Native relay is unchanged.'
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

              {form.watch('bamboo.enable_host_tools') && (
                <div className='space-y-4 rounded-lg border p-4'>
                  <FormField
                    control={form.control}
                    name='bamboo.host_tool_mode'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('Host tool mode')}</FormLabel>
                        <FormDescription>
                          {t(
                            'Loop (recommended) runs one extra upstream hop after executing tools. Return folds raw search/fetch text as the final assistant message (debug only).'
                          )}
                        </FormDescription>
                        <Select
                          value={field.value}
                          onValueChange={field.onChange}
                        >
                          <FormControl>
                            <SelectTrigger className='w-56'>
                              <SelectValue />
                            </SelectTrigger>
                          </FormControl>
                          <SelectContent alignItemWithTrigger={false}>
                            <SelectItem value='loop'>
                              {t('Loop (A-thin)')}
                            </SelectItem>
                            <SelectItem value='return'>
                              {t('Return (debug fold)')}
                            </SelectItem>
                          </SelectContent>
                        </Select>
                      </FormItem>
                    )}
                  />
                  <FormField
                    control={form.control}
                    name='bamboo.search_backend'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('Search backend')}</FormLabel>
                        <FormDescription>
                          {t(
                            'Default is off. Exa and Parallel send queries to a third party and also require the egress toggle below. SearXNG is the self-hosted option.'
                          )}
                        </FormDescription>
                        <Select
                          value={field.value}
                          onValueChange={field.onChange}
                        >
                          <FormControl>
                            <SelectTrigger className='w-56'>
                              <SelectValue />
                            </SelectTrigger>
                          </FormControl>
                          <SelectContent alignItemWithTrigger={false}>
                            <SelectItem value='off'>{t('Off')}</SelectItem>
                            <SelectItem value='exa'>Exa MCP</SelectItem>
                            <SelectItem value='parallel'>
                              Parallel MCP
                            </SelectItem>
                            <SelectItem value='searxng'>SearXNG</SelectItem>
                          </SelectContent>
                        </Select>
                      </FormItem>
                    )}
                  />
                  <FormField
                    control={form.control}
                    name='bamboo.allow_third_party_search_egress'
                    render={({ field }) => (
                      <FormItem className='flex flex-row items-center justify-between gap-4'>
                        <div className='space-y-0.5'>
                          <FormLabel>
                            {t('Allow third-party search egress')}
                          </FormLabel>
                          <FormDescription>
                            {t(
                              'Required before Exa or Parallel can run. Leave off unless you accept sending user queries to those hosts. HTTP_PROXY also weakens WebFetch DNS rebinding protection.'
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
                    name='bamboo.search_fallback'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('Search fallback')}</FormLabel>
                        <FormDescription>
                          {t(
                            'JSON array of backends used after a transport/5xx failure, for example ["parallel"].'
                          )}
                        </FormDescription>
                        <FormControl>
                          <Input {...field} placeholder='["parallel"]' />
                        </FormControl>
                      </FormItem>
                    )}
                  />
                  <FormField
                    control={form.control}
                    name='bamboo.searxng_base_url'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('SearXNG base URL')}</FormLabel>
                        <FormControl>
                          <Input
                            {...field}
                            placeholder='https://searx.example.com'
                          />
                        </FormControl>
                      </FormItem>
                    )}
                  />
                  <FormField
                    control={form.control}
                    name='bamboo.exa_mcp_url'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('Exa MCP URL')}</FormLabel>
                        <FormControl>
                          <Input {...field} />
                        </FormControl>
                      </FormItem>
                    )}
                  />
                  <FormField
                    control={form.control}
                    name='bamboo.exa_api_key'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('Exa API key')}</FormLabel>
                        <FormDescription>
                          {t('Write-only. Leave empty to keep the stored key.')}
                        </FormDescription>
                        <FormControl>
                          <Input
                            type='password'
                            autoComplete='new-password'
                            {...field}
                          />
                        </FormControl>
                      </FormItem>
                    )}
                  />
                  <FormField
                    control={form.control}
                    name='bamboo.parallel_mcp_url'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('Parallel MCP URL')}</FormLabel>
                        <FormControl>
                          <Input {...field} />
                        </FormControl>
                      </FormItem>
                    )}
                  />
                  <FormField
                    control={form.control}
                    name='bamboo.parallel_api_key'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('Parallel API key')}</FormLabel>
                        <FormDescription>
                          {t('Write-only. Leave empty to keep the stored key.')}
                        </FormDescription>
                        <FormControl>
                          <Input
                            type='password'
                            autoComplete='new-password'
                            {...field}
                          />
                        </FormControl>
                      </FormItem>
                    )}
                  />
                  <div className='grid gap-4 sm:grid-cols-3'>
                    <FormField
                      control={form.control}
                      name='bamboo.max_search_results'
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel>{t('Max search results')}</FormLabel>
                          <FormControl>
                            <Input type='number' min={1} max={20} {...field} />
                          </FormControl>
                        </FormItem>
                      )}
                    />
                    <FormField
                      control={form.control}
                      name='bamboo.max_fetch_bytes'
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel>{t('Max fetch bytes')}</FormLabel>
                          <FormControl>
                            <Input type='number' min={1} {...field} />
                          </FormControl>
                        </FormItem>
                      )}
                    />
                    <FormField
                      control={form.control}
                      name='bamboo.host_tool_timeout_ms'
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel>{t('Host tool timeout (ms)')}</FormLabel>
                          <FormControl>
                            <Input
                              type='number'
                              min={1}
                              max={30000}
                              {...field}
                            />
                          </FormControl>
                        </FormItem>
                      )}
                    />
                  </div>
                </div>
              )}

              <FormField
                control={form.control}
                name='bamboo.enable_image_recognize'
                render={({ field }) => (
                  <FormItem className='flex flex-row items-center justify-between gap-4 rounded-lg border p-4'>
                    <div className='space-y-0.5'>
                      <FormLabel className='text-base'>
                        {t('Bamboo image recognition')}
                      </FormLabel>
                      <FormDescription>
                        {t(
                          'When the main model has no Vision tag, recognize images in the latest user message on a designated channel and model. Images are stripped before the main hop, and the caption is boxed at the start of the reply. Tag Vision on /models/metadata to skip this.'
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

              {form.watch('bamboo.enable_image_recognize') && (
                <div className='grid gap-4 rounded-lg border p-4 sm:grid-cols-2'>
                  <FormField
                    control={form.control}
                    name='bamboo.image_recognize_channel_id'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>
                          {t('Image recognition channel ID')}
                        </FormLabel>
                        <FormControl>
                          <Input type='number' min={0} {...field} />
                        </FormControl>
                      </FormItem>
                    )}
                  />
                  <FormField
                    control={form.control}
                    name='bamboo.image_recognize_model'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('Image recognition model')}</FormLabel>
                        <FormControl>
                          <Input
                            placeholder={t('Model name on that channel')}
                            {...field}
                          />
                        </FormControl>
                      </FormItem>
                    )}
                  />
                  <FormField
                    control={form.control}
                    name='bamboo.image_recognize_max_images'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>
                          {t('Max images in latest message')}
                        </FormLabel>
                        <FormControl>
                          <Input type='number' min={1} max={8} {...field} />
                        </FormControl>
                      </FormItem>
                    )}
                  />
                  <FormField
                    control={form.control}
                    name='bamboo.image_recognize_timeout_ms'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>
                          {t('Image recognition timeout (ms)')}
                        </FormLabel>
                        <FormControl>
                          <Input
                            type='number'
                            min={1}
                            max={60000}
                            {...field}
                          />
                        </FormControl>
                      </FormItem>
                    )}
                  />
                  <FormField
                    control={form.control}
                    name='bamboo.image_recognize_retry_times'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>
                          {t('Image recognition retry times')}
                        </FormLabel>
                        <FormControl>
                          <Input type='number' min={0} max={5} {...field} />
                        </FormControl>
                      </FormItem>
                    )}
                  />
                  <FormField
                    control={form.control}
                    name='bamboo.image_recognize_fail_open'
                    render={({ field }) => (
                      <FormItem className='sm:col-span-2 flex flex-row items-center justify-between gap-4 rounded-lg border p-4'>
                        <div className='space-y-0.5'>
                          <FormLabel className='text-base'>
                            {t('Image recognition fail open')}
                          </FormLabel>
                          <FormDescription>
                            {t(
                              'Continue the request, with images stripped, when image recognition finally fails.'
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
                    name='bamboo.image_recognize_prompt'
                    render={({ field }) => (
                      <FormItem className='sm:col-span-2'>
                        <FormLabel>{t('Image recognition prompt')}</FormLabel>
                        <FormControl>
                          <Input
                            placeholder={t(
                              'Leave empty to use the built-in prompt'
                            )}
                            {...field}
                          />
                        </FormControl>
                      </FormItem>
                    )}
                  />
                </div>
              )}
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
