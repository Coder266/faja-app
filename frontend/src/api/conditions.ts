/**
 * Typed client for the backend's `GET /api/conditions` endpoint (S5).
 *
 * The `Conditions` interface mirrors the JSON contract exactly, including
 * which fields may be `null` on a partial upstream failure (the endpoint
 * still returns 200 in that case).
 */

export interface Uv {
  index: number
  classPt: string
  classEn: string
}

export interface Tides {
  highTides: string[]
  lowTides: string[]
}

export interface Conditions {
  /** Today's date in `Atlantic/Madeira`, formatted `YYYY-MM-DD`. */
  date: string
  airTempMin: number | null
  airTempMax: number | null
  waterTemp: number | null
  uv: Uv | null
  tides: Tides | null
  /** ISO 8601 timestamp of when the backend last refreshed the cache. */
  updatedAt: string
}

/**
 * Fetches the aggregated conditions payload.
 *
 * Uses a relative path so it works both behind the Vite dev proxy (S9) and
 * in production, where the frontend is served from the same Go binary as
 * the API — never hardcode a host/port here.
 */
export async function fetchConditions(): Promise<Conditions> {
  const response = await fetch('/api/conditions')
  if (!response.ok) {
    throw new Error(`GET /api/conditions failed with status ${response.status}`)
  }
  return (await response.json()) as Conditions
}
