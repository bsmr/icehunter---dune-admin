import * as React from 'react'
import { Switch, Description } from '@heroui/react'
import { useTranslation } from 'react-i18next'
import type { CapabilityGridProps } from './types'

// CapabilityGrid renders the switch matrix for one principal (role, pseudo
// role, or local user). Capabilities in `inherited` are granted by a higher
// link in the cascade (the Default row) — they show locked-on and can only be
// changed where they originate.
export const CapabilityGrid: React.FC<CapabilityGridProps> = ({ capabilities, selected, inherited = [], onToggle }) => {
  const { t } = useTranslation()
  return (
    <div className="grid grid-cols-1 sm:grid-cols-2 xl:grid-cols-3 gap-x-6 gap-y-2">
      {capabilities.map((cap) => {
        const isInherited = inherited.includes(cap.id)
        return (
          <Switch
            key={cap.id}
            size="sm"
            isSelected={isInherited || selected.includes(cap.id)}
            isDisabled={isInherited}
            onChange={(on: boolean) => onToggle(cap.id, on)}
          >
            <Switch.Content>
              <Switch.Control><Switch.Thumb /></Switch.Control>
              <span className="font-mono text-xs">
                {cap.id}
                {isInherited && <span className="ml-1 text-[10px] text-muted">{t('permissions.inherited')}</span>}
              </span>
            </Switch.Content>
            <Description>
              {/* Localized description; the backend's English text is the
                  fallback for capabilities added after this translation set. */}
              {t(`permissions.caps.${cap.id.replace(':', '_')}` as never, { defaultValue: cap.description })}
            </Description>
          </Switch>
        )
      })}
    </div>
  )
}
