/**
 * Pure formatting/mapping helpers that turn a `Conditions` payload (S5) into
 * the `ValueLine[]` shape each `InfoBlock` expects. Kept out of the template
 * so the date/temperature/UV/tide formatting rules are plain, type-checked,
 * unit-testable functions.
 *
 * A null field (partial upstream failure, S5 still returns 200) maps to a
 * single `—` line, so that block reads as "unavailable" while the rest of
 * the poster keeps showing real values (S8).
 */
import type { ValueLine } from '@/components/InfoBlock.vue'
import type { Conditions } from '@/api/conditions'

/** Single-line placeholder used whenever a field/block has no value to show. */
export const DASH_LINE: ValueLine[] = [{ text: '—' }]

/** Formats an already Madeira-local `YYYY-MM-DD` date string as `DD/MM`. */
export function formatDate(date: string): string {
  const [, month, day] = date.split('-')
  if (!month || !day) return date
  return `${day}/${month}`
}

/**
 * Formats the air temperature range as `min–max °C`, collapsing to a single
 * value when they're equal. Returns `null` if both bounds are missing.
 */
export function formatAirTemp(min: number | null, max: number | null): string | null {
  if (min == null && max == null) return null
  if (min == null) return `${max} °C`
  if (max == null) return `${min} °C`
  if (min === max) return `${min} °C`
  return `${min}–${max} °C`
}

/** Formats the single-value water temperature as `N °C`. */
export function formatWaterTemp(temp: number | null): string | null {
  if (temp == null) return null
  return `${temp} °C`
}

export function dataBlockLines(conditions: Conditions): ValueLine[] {
  return [{ text: formatDate(conditions.date) }]
}

export function temperatureBlockLines(conditions: Conditions): ValueLine[] {
  const text = formatAirTemp(conditions.airTempMin, conditions.airTempMax)
  return text == null ? DASH_LINE : [{ text }]
}

export function waterBlockLines(conditions: Conditions): ValueLine[] {
  const text = formatWaterTemp(conditions.waterTemp)
  return text == null ? DASH_LINE : [{ text }]
}

export function uvBlockLines(conditions: Conditions): ValueLine[] {
  const uv = conditions.uv
  if (!uv) return DASH_LINE
  return [
    { text: String(uv.index) },
    { text: uv.classPt, size: 'sm' },
    { text: uv.classEn, size: 'sm' },
  ]
}

export function highTideBlockLines(conditions: Conditions): ValueLine[] {
  const tides = conditions.tides?.highTides
  return tides && tides.length > 0 ? tides.map((text) => ({ text })) : DASH_LINE
}

export function lowTideBlockLines(conditions: Conditions): ValueLine[] {
  const tides = conditions.tides?.lowTides
  return tides && tides.length > 0 ? tides.map((text) => ({ text })) : DASH_LINE
}
