import * as React from 'react'
import { useTranslation } from 'react-i18next'
import { Button, Spinner } from '@heroui/react'
import { MASKED } from '../../../api/client'
import type { AppConfig } from '../../../api/client'
import { Icon, Panel, SectionLabel } from '../../../dune-ui'
import { FieldRow } from '../fields/FieldRow'
import { TextInput } from '../fields/TextInput'
import { CheckboxField } from '../fields/CheckboxField'
import { TwoColumnGrid } from '../fields/TwoColumnGrid'
import { RolePicker } from '../fields/RolePicker'
import type { DiscordRole } from '../../types'

export interface DiscordPanelProps {
  cfg: AppConfig
  set: (key: keyof AppConfig) => (v: string) => void
  setBool: (key: keyof AppConfig) => (v: boolean) => void
  discordRoles: DiscordRole[]
  rolesLoading: boolean
  loadDiscordRoles: () => void
}

export const DiscordPanel: React.FC<DiscordPanelProps> = ({
  cfg, set, setBool, discordRoles, rolesLoading, loadDiscordRoles,
}) => {
  const { t } = useTranslation()
  return (
    <div className="overflow-y-auto flex-1 pr-1 flex flex-col gap-4">
      <input type="text" autoComplete="username" aria-hidden="true" tabIndex={-1} readOnly className="sr-only" />
      <Panel>
        <SectionLabel>{t('settings.sections.discordBot')}</SectionLabel>
        <div className="flex flex-col gap-1 -mt-1">
          <p className="text-sm text-muted">{t('settings.discord.hint')}</p>
          <p className="text-sm text-muted">{t('settings.discord.setupStep1')}</p>
          <p className="text-sm text-muted">{t('settings.discord.setupStep2')}</p>
          <p className="text-sm text-muted">{t('settings.discord.setupStep3')}</p>
        </div>
        <TwoColumnGrid>
          <div className="sm:col-span-2">
            <CheckboxField
              label={t('settings.discord.enabled')}
              checked={cfg.discord_bot_enabled}
              onChange={setBool('discord_bot_enabled')}
            />
          </div>
          <FieldRow label={t('settings.discord.token')} hint={t('settings.discord.tokenHint')}>
            <TextInput value={cfg.discord_bot_token} onChange={set('discord_bot_token')} type="password" placeholder={MASKED} />
          </FieldRow>
          <FieldRow label={t('settings.discord.guildId')} hint={t('settings.discord.guildIdHint')}>
            <TextInput value={cfg.discord_guild_id} onChange={set('discord_guild_id')} placeholder="123456789012345678" />
          </FieldRow>
        </TwoColumnGrid>
      </Panel>

      <Panel>
        <div className="flex items-center justify-between">
          <SectionLabel>{t('settings.sections.discordRoles')}</SectionLabel>
          <Button size="sm" variant="ghost" onPress={loadDiscordRoles} isDisabled={rolesLoading}>
            {rolesLoading ? <Spinner size="sm" color="current" /> : <Icon name="refresh-cw" />}
            {' '}
            {t('common.refresh')}
          </Button>
        </div>
        <div className="flex flex-col gap-1 -mt-1">
          <p className="text-xs text-muted">{t('settings.discord.rolesHint')}</p>
          <p className="text-sm text-muted">{t('settings.discord.rolesRefreshNote')}</p>
        </div>
        <TwoColumnGrid>
          <RolePicker
            label={t('settings.discord.rolesViewer')}
            hint={t('settings.discord.rolesViewerHint')}
            value={cfg.discord_roles_viewer}
            onChange={set('discord_roles_viewer')}
            roles={discordRoles}
          />
          <RolePicker
            label={t('settings.discord.rolesEconomy')}
            hint={t('settings.discord.rolesEconomyHint')}
            value={cfg.discord_roles_economy}
            onChange={set('discord_roles_economy')}
            roles={discordRoles}
          />
          <RolePicker
            label={t('settings.discord.rolesAdmin')}
            hint={t('settings.discord.rolesAdminHint')}
            value={cfg.discord_roles_admin}
            onChange={set('discord_roles_admin')}
            roles={discordRoles}
          />
          <FieldRow label={t('settings.discord.announceChannel')} hint={t('settings.discord.announceChannelHint')}>
            <TextInput value={cfg.discord_announce_channel_id} onChange={set('discord_announce_channel_id')} placeholder="444444444444444444" />
          </FieldRow>
        </TwoColumnGrid>
      </Panel>

      <Panel>
        <SectionLabel>{t('settings.sections.discordStatus')}</SectionLabel>
        <div className="flex flex-col gap-1 -mt-1">
          <p className="text-sm text-muted">{t('settings.discord.statusHint')}</p>
        </div>
        <TwoColumnGrid>
          <div className="sm:col-span-2">
            <CheckboxField
              label={t('settings.discord.statusEnabled')}
              hint={t('settings.discord.statusEnabledHint')}
              checked={cfg.discord_status_enabled}
              onChange={setBool('discord_status_enabled')}
            />
          </div>
          <FieldRow label={t('settings.discord.statusChannel')} hint={t('settings.discord.statusChannelHint')}>
            <TextInput value={cfg.discord_status_channel_id} onChange={set('discord_status_channel_id')} placeholder="555555555555555555" />
          </FieldRow>
          <FieldRow label={t('settings.discord.statusInterval')} hint={t('settings.discord.statusIntervalHint')}>
            <TextInput
              value={cfg.discord_status_interval_seconds ? String(cfg.discord_status_interval_seconds) : ''}
              onChange={set('discord_status_interval_seconds')}
              placeholder="60"
              type="number"
            />
          </FieldRow>
        </TwoColumnGrid>
      </Panel>
    </div>
  )
}
