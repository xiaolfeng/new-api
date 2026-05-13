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
import * as z from 'zod'
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
} from '@/components/ui/form'
import { Switch } from '@/components/ui/switch'
import { SettingsSection } from '../components/settings-section'
import { useResetForm } from '../hooks/use-reset-form'
import { useUpdateOption } from '../hooks/use-update-option'

const customizationSchema = z.object({
  'global.responses_to_chat_completions_enabled': z.boolean(),
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

  const form = useForm<CustomizationFormValues>({
    resolver: zodResolver(customizationSchema),
    defaultValues,
  })

  useResetForm(form, defaultValues)

  const onSubmit = async (data: CustomizationFormValues) => {
    if (
      data['global.responses_to_chat_completions_enabled'] !==
      defaultValues['global.responses_to_chat_completions_enabled']
    ) {
      await updateOption.mutateAsync({
        key: 'global.responses_to_chat_completions_enabled',
        value: String(data['global.responses_to_chat_completions_enabled']),
      })
    }
  }

  return (
    <SettingsSection
      title={t('Customization')}
      description={t(
        'Configure customization features and advanced behavior'
      )}
    >
      <Form {...form}>
        <form onSubmit={form.handleSubmit(onSubmit)} className='space-y-6'>
          <FormField
            control={form.control}
            name='global.responses_to_chat_completions_enabled'
            render={({ field }) => (
              <FormItem className='flex flex-row items-center justify-between rounded-lg border p-4'>
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

          <Button type='submit' disabled={updateOption.isPending}>
            {updateOption.isPending ? t('Saving...') : t('Save Changes')}
          </Button>
        </form>
      </Form>
    </SettingsSection>
  )
}
