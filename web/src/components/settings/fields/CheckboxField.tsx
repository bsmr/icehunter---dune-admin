import * as React from 'react'
import { Switch, Description } from '@heroui/react'
import type { CheckboxFieldProps } from '../../interfaces'

export const CheckboxField: React.FC<CheckboxFieldProps> = ({ label, checked, onChange, hint }) => {
  return (
    <Switch isSelected={!!checked} onChange={onChange} size="sm">
      <Switch.Content>
        <Switch.Control><Switch.Thumb /></Switch.Control>
        {label}
      </Switch.Content>
      {hint && <Description>{hint}</Description>}
    </Switch>
  )
}
