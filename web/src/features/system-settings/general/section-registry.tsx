import { useUpdateOption } from '../hooks/use-update-option'
import type { SiteSettings as GeneralSettings } from '../types'
import { createSectionRegistry } from '../utils/section-registry'
import { ChannelAffinitySection } from './channel-affinity'
import { CheckinSettingsSection } from './checkin-settings-section'
import { EmptyResponseRetrySection } from './empty-response-retry-section'
import { PricingSection } from './pricing-section'
import { QuotaSettingsSection } from './quota-settings-section'
import { SystemBehaviorSection } from './system-behavior-section'
import { SystemInfoSection } from './system-info-section'

const GENERAL_SECTIONS = [
  {
    id: 'system-info',
    titleKey: 'System Information',
    descriptionKey: 'Configure basic system information and branding',
    build: (settings: GeneralSettings) => (
      <SystemInfoSection
        defaultValues={{
          theme: {
            frontend: settings['theme.frontend'] as 'default' | 'classic',
          },
          SystemName: settings.SystemName,
          Logo: settings.Logo,
          Footer: settings.Footer,
          About: settings.About,
          HomePageContent: settings.HomePageContent,
          ServerAddress: settings.ServerAddress,
          TaskPublicAddress: settings.TaskPublicAddress,
          general_setting: {
            docs_link: settings['general_setting.docs_link'],
          },
          legal: {
            user_agreement: settings['legal.user_agreement'],
            privacy_policy: settings['legal.privacy_policy'],
          },
        }}
      />
    ),
  },
  {
    id: 'quota',
    titleKey: 'Quota Settings',
    descriptionKey: 'Configure user quota allocation and rewards',
    build: (settings: GeneralSettings) => (
      <QuotaSettingsSection
        defaultValues={{
          QuotaForNewUser: settings.QuotaForNewUser,
          QuotaForInviter: settings.QuotaForInviter,
          QuotaForInvitee: settings.QuotaForInvitee,
          TopUpLink: settings.TopUpLink,
          quota_setting: {
            enable_free_model_pre_consume:
              settings['quota_setting.enable_free_model_pre_consume'],
            trust_quota_usd:
              (settings as Record<string, any>)['quota_setting.trust_quota_usd'] ?? 10,
            pre_consume_multiplier:
              (settings as Record<string, any>)['quota_setting.pre_consume_multiplier'] ?? 1,
          },
        }}
      />
    ),
  },
  {
    id: 'pricing',
    titleKey: 'Pricing & Display',
    descriptionKey: 'Configure pricing model and display options',
    build: (
      settings: GeneralSettings,
      quotaDisplayType: 'USD' | 'CNY' | 'TOKENS' | 'CUSTOM'
    ) => (
      <PricingSection
        defaultValues={{
          QuotaPerUnit: settings.QuotaPerUnit,
          USDExchangeRate: settings.USDExchangeRate,
          DisplayInCurrencyEnabled: settings.DisplayInCurrencyEnabled,
          DisplayTokenStatEnabled: settings.DisplayTokenStatEnabled,
          general_setting: {
            quota_display_type: quotaDisplayType,
            custom_currency_symbol:
              settings['general_setting.custom_currency_symbol'] ?? '¤',
            custom_currency_exchange_rate:
              settings['general_setting.custom_currency_exchange_rate'] ?? 1,
          },
        }}
      />
    ),
  },
  {
    id: 'checkin',
    titleKey: 'Check-in Settings',
    descriptionKey: 'Configure daily check-in rewards for users',
    build: (settings: GeneralSettings) => (
      <CheckinSettingsSection
        defaultValues={{
          enabled: settings['checkin_setting.enabled'],
          minQuota: settings['checkin_setting.min_quota'],
          maxQuota: settings['checkin_setting.max_quota'],
        }}
      />
    ),
  },
  {
    id: 'empty-response-retry',
    titleKey: 'Empty Response Retry',
    descriptionKey: 'Configure empty response retry and logging behavior',
    build: (settings: GeneralSettings) => (
      <EmptyResponseRetrySection
        defaultValues={{
          'retry_setting.empty_response_retry_enabled':
            settings['retry_setting.empty_response_retry_enabled'] ?? false,
          'retry_setting.empty_response_retry_delay_seconds':
            settings['retry_setting.empty_response_retry_delay_seconds'] ?? 0,
          'retry_setting.record_consume_log_detail_enabled':
            settings['retry_setting.record_consume_log_detail_enabled'] ??
            false,
          'retry_setting.full_log_consume_enabled':
            settings['retry_setting.full_log_consume_enabled'] ?? false,
          'retry_setting.full_log_consume_expires_at':
            settings['retry_setting.full_log_consume_expires_at'] ?? 0,
          'retry_setting.full_log_consume_remaining_seconds':
            settings['retry_setting.full_log_consume_remaining_seconds'] ?? 0,
        }}
      />
    ),
  },
  {
    id: 'behavior',
    titleKey: 'System Behavior',
    descriptionKey: 'Configure system-wide behavior and defaults',
    build: (settings: GeneralSettings) => (
      <SystemBehaviorSection
        defaultValues={{
          DefaultCollapseSidebar: settings.DefaultCollapseSidebar,
          DemoSiteEnabled: settings.DemoSiteEnabled,
          SelfUseModeEnabled: settings.SelfUseModeEnabled,
        }}
      />
    ),
  },
  {
    id: 'channel-affinity',
    titleKey: 'Channel Affinity',
    descriptionKey: 'Configure channel affinity (sticky routing) rules',
    build: (settings: GeneralSettings) => (
      <ChannelAffinitySectionWrapper settings={settings} />
    ),
  },
] as const

function ChannelAffinitySectionWrapper(props: { settings: GeneralSettings }) {
  const updateOption = useUpdateOption()
  return (
    <ChannelAffinitySection
      rulesJson={props.settings['channel_affinity_setting.rules'] || '[]'}
      onRulesChange={(rules) => {
        updateOption.mutate({
          key: 'channel_affinity_setting.rules',
          value: rules,
        })
      }}
      enabled={props.settings['channel_affinity_setting.enabled']}
    />
  )
}

export type GeneralSectionId = (typeof GENERAL_SECTIONS)[number]['id']

const generalRegistry = createSectionRegistry<
  GeneralSectionId,
  GeneralSettings,
  ['USD' | 'CNY' | 'TOKENS' | 'CUSTOM']
>({
  sections: GENERAL_SECTIONS,
  defaultSection: 'system-info',
  basePath: '/system-settings/general',
  urlStyle: 'path',
})

export const GENERAL_SECTION_IDS = generalRegistry.sectionIds
export const GENERAL_DEFAULT_SECTION = generalRegistry.defaultSection
export const getGeneralSectionNavItems = generalRegistry.getSectionNavItems
export const getGeneralSectionContent = generalRegistry.getSectionContent
