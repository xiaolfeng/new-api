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
import { describe, expect, test } from 'vitest'

import {
  getChannelConfigurationSection,
  getChannelConfigurationState,
} from '../../lib/channel-configuration'
import {
  CHANNEL_FORM_DEFAULT_VALUES,
  transformChannelToFormDefaults,
  transformFormDataToCreatePayload,
  transformFormDataToUpdatePayload,
} from '../../lib/channel-form'
import { channelSchema } from '../../types'

describe('Bamboo channel settings', () => {
  test('maps bamboo fields to request section', () => {
    expect(getChannelConfigurationSection('bamboo_upstream_format')).toBe('request')
    expect(getChannelConfigurationSection('bamboo_legacy_compat')).toBe('request')
    expect(getChannelConfigurationSection('bamboo_legacy_cache_key')).toBe('request')
    expect(getChannelConfigurationSection('bamboo_strip_think_tags')).toBe('request')
    expect(getChannelConfigurationSection('bamboo_include_reasoning_content')).toBe('request')
    expect(getChannelConfigurationSection('bamboo_ignore_encrypted_content')).toBe('request')
  })

  test('recognizes configured status when bamboo_upstream_format is set', () => {
    const values = {
      ...CHANNEL_FORM_DEFAULT_VALUES,
      bamboo_upstream_format: 'openai' as const,
    }
    const state = getChannelConfigurationState(values, {}, false)
    expect(state.blocks.bambooSettings).toBe('configured')
    expect(state.sections.request).toBe('configured')
  })

  test.each(['openai', 'anthropic', 'gemini', 'responses'] as const)(
    'preserves bamboo_upstream_format %s through create, update and reload',
    (format) => {
      const channel = channelSchema.parse({
        id: 1,
        name: 'Bamboo Channel',
        key: '',
        type: 24, // Gemini channel
        status: 1,
        created_time: 0,
        test_time: 0,
        response_time: 0,
        balance_updated_time: 0,
        settings: JSON.stringify({
          bamboo_upstream_format: format,
          bamboo_legacy_compat: format === 'openai',
        }),
      })

      const values = transformChannelToFormDefaults(channel)
      expect(values.bamboo_upstream_format).toBe(format)
      if (format === 'openai') {
        expect(values.bamboo_legacy_compat).toBe(true)
      }

      const createPayload = transformFormDataToCreatePayload(values).channel
      const updatePayload = transformFormDataToUpdatePayload(values, channel.id)

      for (const payload of [createPayload, updatePayload]) {
        expect(typeof payload.settings).toBe('string')
        const parsedSettings = JSON.parse(payload.settings as string)
        expect(parsedSettings.bamboo_upstream_format).toBe(format)
        if (format === 'openai') {
          expect(parsedSettings.bamboo_legacy_compat).toBe(true)
        }

        const reloaded = transformChannelToFormDefaults({
          ...channel,
          settings: payload.settings,
        })
        expect(reloaded.bamboo_upstream_format).toBe(format)
      }
    }
  )

  test('auto format deletes bamboo_upstream_format from settings json', () => {
    const values = {
      ...CHANNEL_FORM_DEFAULT_VALUES,
      bamboo_upstream_format: 'auto' as const,
    }
    const payload = transformFormDataToCreatePayload(values).channel
    const parsed = JSON.parse(payload.settings as string)
    expect(parsed.bamboo_upstream_format).toBeUndefined()
  })
})
