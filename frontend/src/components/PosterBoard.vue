<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'
import InfoBlock, { type ValueLine } from '@/components/InfoBlock.vue'
import PosterBackdrop from '@/components/PosterBackdrop.vue'
import logoUrl from '@/assets/imgs/logo.png'
import { fetchConditions, type Conditions } from '@/api/conditions'
import {
  DASH_LINE,
  dataBlockLines,
  highTideBlockLines,
  lowTideBlockLines,
  temperatureBlockLines,
  uvBlockLines,
  waterBlockLines,
} from '@/lib/formatConditions'

/**
 * Explicit UI status instead of inferring it from `conditions === null`, so
 * "no data yet" (loading) and "the fetch failed" (error) can render
 * differently: loading shows shimmer placeholders, error shows `—`
 * everywhere plus a bilingual note. A successful fetch that returns partial
 * nulls is `ready` — per-field `—` is handled inside the formatConditions
 * helpers, not here.
 */
type Status = 'loading' | 'ready' | 'error'

// At least hourly, per S7, so a kiosk/pinned tab rolls over to the new day
// even if it's never brought back to the foreground.
const REFRESH_INTERVAL_MS = 60 * 60 * 1000

// While in the error state, retry sooner than the hourly refresh so a
// transient network blip clears itself without the visitor doing anything.
const RETRY_INTERVAL_MS = 30 * 1000

const status = ref<Status>('loading')
const conditions = ref<Conditions | null>(null)

let intervalId: ReturnType<typeof setInterval> | undefined
let retryIntervalId: ReturnType<typeof setInterval> | undefined

function stopRetryTimer() {
  if (retryIntervalId !== undefined) {
    clearInterval(retryIntervalId)
    retryIntervalId = undefined
  }
}

function startRetryTimer() {
  if (retryIntervalId !== undefined) return
  retryIntervalId = setInterval(() => void refresh(), RETRY_INTERVAL_MS)
}

async function refresh() {
  try {
    const data = await fetchConditions()
    conditions.value = data
    status.value = 'ready'
    stopRetryTimer()
  } catch (err) {
    console.error('Failed to fetch /api/conditions', err)
    // Whole-request failure: don't keep showing stale values next to the
    // "data unavailable" note — every block goes to `—` until a retry (or
    // the hourly refresh, or a visibility-change refresh) succeeds.
    conditions.value = null
    status.value = 'error'
    startRetryTimer()
  }
}

/** Maps the current conditions through `formatter`, falling back to a
 * single `—` line when there's no usable response at all (loading/error). */
function linesFor(formatter: (c: Conditions) => ValueLine[]): ValueLine[] {
  return conditions.value ? formatter(conditions.value) : DASH_LINE
}

function handleVisibilityChange() {
  if (document.visibilityState === 'visible') {
    void refresh()
  }
}

onMounted(() => {
  void refresh()
  document.addEventListener('visibilitychange', handleVisibilityChange)
  intervalId = setInterval(() => void refresh(), REFRESH_INTERVAL_MS)
})

onUnmounted(() => {
  document.removeEventListener('visibilitychange', handleVisibilityChange)
  if (intervalId !== undefined) clearInterval(intervalId)
  stopRetryTimer()
})
</script>

<template>
  <main class="poster-shell" aria-label="Fajã dos Padres — condições do dia">
    <p v-if="status === 'error'" class="status-note" role="status">
      Dados indisponíveis · Data unavailable
    </p>

    <div class="poster">
      <PosterBackdrop />

      <InfoBlock
        shape="data"
        label-pt="Data"
        label-en="Date"
        :loading="status === 'loading'"
        :lines="linesFor(dataBlockLines)"
      />

      <InfoBlock
        shape="temperature"
        label-pt="Temperatura"
        label-en="Temperature"
        :loading="status === 'loading'"
        :lines="linesFor(temperatureBlockLines)"
      />

      <InfoBlock
        shape="water"
        label-pt="Temperatura da água"
        label-en="Water temperature"
        :loading="status === 'loading'"
        :lines="linesFor(waterBlockLines)"
      />

      <InfoBlock
        shape="uv"
        label-pt="UV"
        :loading="status === 'loading'"
        :placeholder-sizes="['lg', 'sm', 'sm']"
        :lines="linesFor(uvBlockLines)"
      />

      <InfoBlock
        shape="highTide"
        label-pt="Maré cheia"
        label-en="High tide"
        :loading="status === 'loading'"
        :lines="linesFor(highTideBlockLines)"
      />

      <InfoBlock
        shape="lowTide"
        label-pt="Maré vazia"
        label-en="Low tide"
        :loading="status === 'loading'"
        :placeholder-sizes="['lg', 'lg']"
        :lines="linesFor(lowTideBlockLines)"
      />

      <div class="wordmark">
        <img class="wordmark__glyph" :src="logoUrl" alt="" aria-hidden="true" />
        <span class="wordmark__text">Fajã dos<br />Padres</span>
      </div>
    </div>
  </main>
</template>

<style scoped>
.poster-shell {
  display: flex;
  flex-direction: column;
  align-items: center;
  width: 100%;
  max-width: 30rem;
  gap: 0.6rem;
}

.status-note {
  font-family: var(--font-serif);
  font-size: 0.75rem;
  line-height: 1.3;
  color: var(--color-text-label-soft);
  background: rgba(255, 255, 255, 0.6);
  padding: 0.3rem 0.9rem;
  border-radius: 999px;
  text-align: center;
}

.poster {
  position: relative;
  width: 100%;
  background: var(--color-poster-bg);
  border-radius: 1.25rem;
  box-shadow: 0 1.5rem 3rem rgba(20, 40, 65, 0.18);
  padding: 1.75rem 1.5rem;

  display: grid;
  grid-template-columns: 1.15fr 1fr;
  grid-template-rows: repeat(8, 1fr);
  gap: 0.9rem;
  aspect-ratio: 3 / 4.1;
  /* Without this, the grid's automatic min-height (its content min-size,
     which the tall pebble blocks inflate) overrides `aspect-ratio`, letting
     the poster stretch to ~2400px instead of honoring the 3:4.1 ratio. */
  min-height: 0;
  grid-template-areas:
    'data         .          '
    'data         temperature'
    'water        temperature'
    'water        uv         '
    'highTide     uv         '
    'highTide     lowTide    '
    'wordmark     lowTide    '
    'wordmark     lowTide    ';
}

/* Keep the content above the decorative backdrop, but don't turn the
   backdrop itself into a grid item: `position: relative` here used to
   override PosterBackdrop's `position: absolute`, adding a phantom row on
   mobile and stretching the poster far past its content. */
.poster > :not(.backdrop) {
  position: relative;
  z-index: 1;
}

.wordmark {
  grid-area: wordmark;
  align-self: end;
  display: flex;
  align-items: center;
  gap: 0.45rem;
  color: var(--color-text-label);
}

.wordmark__glyph {
  height: clamp(2.6rem, 6.5vw, 3.5rem);
  width: auto;
  display: block;
}

.wordmark__text {
  font-family: var(--font-serif);
  font-weight: 700;
  font-size: clamp(0.95rem, 2.6vw, 1.2rem);
  line-height: 1.05;
}

@media (max-width: 640px) {
  .poster {
    grid-template-columns: 1fr;
    grid-template-rows: none;
    aspect-ratio: auto;
    gap: 1rem;
    grid-template-areas:
      'data'
      'temperature'
      'water'
      'uv'
      'highTide'
      'lowTide'
      'wordmark';
  }

  .poster :deep(.block) {
    min-height: 6.5rem;
  }

  .wordmark {
    justify-content: center;
    padding-top: 0.5rem;
  }
}
</style>
