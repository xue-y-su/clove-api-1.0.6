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
import { useEffect, useState, type ComponentType } from 'react'

import {
  loadLobeIconModule,
  type LobeIconModule,
} from '@/lib/lobe-icon-loaders'

function parseValue(raw: string | undefined | null): string | number | boolean {
  if (raw == null) return true

  let value = String(raw).trim()

  if (value.startsWith('{') && value.endsWith('}')) {
    value = value.slice(1, -1).trim()
  }

  if (
    (value.startsWith('"') && value.endsWith('"')) ||
    (value.startsWith("'") && value.endsWith("'"))
  ) {
    return value.slice(1, -1)
  }

  if (value === 'true') return true
  if (value === 'false') return false
  if (/^-?\d+(?:\.\d+)?$/.test(value)) return Number(value)

  return value
}

function IconFallback(props: { label: string; size: number }) {
  return (
    <div
      className='bg-muted text-muted-foreground flex items-center justify-center rounded-full text-xs font-medium'
      style={{ width: props.size, height: props.size }}
    >
      {props.label}
    </div>
  )
}

function isIconComponent(
  value: unknown
): value is ComponentType<Record<string, unknown>> {
  return (
    typeof value === 'function' || (typeof value === 'object' && value !== null)
  )
}

function LobeAvatar(props: {
  IconComponent: ComponentType<Record<string, unknown>>
  iconProps: Record<string, string | number | boolean>
  size: number
}) {
  const componentProps: Record<string, string | number | boolean> = {
    ...props.iconProps,
    size: Math.max(1, Math.round(props.size * 0.68)),
  }
  delete componentProps.type
  delete componentProps.shape

  const roundedClass =
    props.iconProps.shape === 'square' ? 'rounded-[20%]' : 'rounded-full'
  const colorClass =
    props.iconProps.type === 'platform'
      ? 'bg-foreground text-background'
      : 'bg-muted text-foreground'

  return (
    <span
      className={`inline-flex shrink-0 items-center justify-center ${roundedClass} ${colorClass}`}
      style={{ width: props.size, height: props.size }}
    >
      <props.IconComponent {...componentProps} />
    </span>
  )
}

export function LobeIcon(props: { iconName: string; size: number }) {
  const segments = props.iconName.split('.')
  const baseKey = segments[0]
  const variant =
    segments.length > 1 && /^[A-Z]/.test(segments[1]) ? segments[1] : undefined
  const cacheKey = `${baseKey}.${variant || 'Mono'}`
  const [loaded, setLoaded] = useState<
    { cacheKey: string; module: LobeIconModule } | undefined
  >()

  useEffect(() => {
    const promise = loadLobeIconModule(baseKey, variant)
    if (!promise) return

    let active = true
    void promise
      .then((module) => {
        if (active) setLoaded({ cacheKey, module })
      })
      .catch(() => {
        if (active) setLoaded(undefined)
      })

    return () => {
      active = false
    }
  }, [baseKey, cacheKey, variant])

  const loadedIcon =
    loaded?.cacheKey === cacheKey ? loaded.module.default : null
  const IconComponent = isIconComponent(loadedIcon) ? loadedIcon : undefined
  const propStartIndex = variant ? 2 : 1

  if (!IconComponent) {
    return (
      <IconFallback
        label={props.iconName.charAt(0).toUpperCase()}
        size={props.size}
      />
    )
  }

  const iconProps: Record<string, string | number | boolean> = {}

  for (let index = propStartIndex; index < segments.length; index++) {
    const segment = segments[index]
    if (!segment) continue

    const equalsIndex = segment.indexOf('=')
    if (equalsIndex === -1) {
      iconProps[segment.trim()] = true
      continue
    }

    const key = segment.slice(0, equalsIndex).trim()
    const rawValue = segment.slice(equalsIndex + 1).trim()
    iconProps[key] = parseValue(rawValue)
  }

  if (iconProps.size == null) {
    iconProps.size = props.size
  }

  if (variant === 'Avatar') {
    return (
      <LobeAvatar
        IconComponent={IconComponent}
        iconProps={iconProps}
        size={props.size}
      />
    )
  }

  return <IconComponent {...iconProps} />
}
