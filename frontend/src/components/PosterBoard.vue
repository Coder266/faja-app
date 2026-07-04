<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'
import InfoBlock from '@/components/InfoBlock.vue'
import PosterBackdrop from '@/components/PosterBackdrop.vue'
import { fetchConditions, type Conditions } from '@/api/conditions'
import {
  dataBlockLines,
  highTideBlockLines,
  lowTideBlockLines,
  temperatureBlockLines,
  uvBlockLines,
  waterBlockLines,
} from '@/lib/formatConditions'

// At least hourly, per S7, so a kiosk/pinned tab rolls over to the new day
// even if it's never brought back to the foreground.
const REFRESH_INTERVAL_MS = 60 * 60 * 1000

// Before the first successful fetch (and on a request error) every block
// renders with no lines, i.e. blank. The polished loading skeleton / `—`
// placeholder / offline notice are S8's job, not this story's.
const conditions = ref<Conditions | null>(null)

async function refresh() {
  try {
    conditions.value = await fetchConditions()
  } catch (err) {
    // S8 adds user-visible error handling and retry; for now, log and keep
    // showing the last known-good values.
    console.error('Failed to fetch /api/conditions', err)
  }
}

function handleVisibilityChange() {
  if (document.visibilityState === 'visible') {
    void refresh()
  }
}

let intervalId: ReturnType<typeof setInterval> | undefined

onMounted(() => {
  void refresh()
  document.addEventListener('visibilitychange', handleVisibilityChange)
  intervalId = setInterval(() => void refresh(), REFRESH_INTERVAL_MS)
})

onUnmounted(() => {
  document.removeEventListener('visibilitychange', handleVisibilityChange)
  if (intervalId !== undefined) clearInterval(intervalId)
})
</script>

<template>
  <main class="poster" aria-label="Fajã dos Padres — condições do dia">
    <PosterBackdrop />

    <InfoBlock
      shape="data"
      label-pt="Data"
      label-en="Date"
      :lines="conditions ? dataBlockLines(conditions) : []"
    />

    <InfoBlock
      shape="temperature"
      label-pt="Temperatura"
      label-en="Temperature"
      :lines="conditions ? temperatureBlockLines(conditions) : []"
    />

    <InfoBlock
      shape="water"
      label-pt="Temperatura da água"
      label-en="Water temperature"
      :lines="conditions ? waterBlockLines(conditions) : []"
    />

    <InfoBlock shape="uv" label-pt="UV" :lines="conditions ? uvBlockLines(conditions) : []" />

    <InfoBlock
      shape="highTide"
      label-pt="Maré cheia"
      label-en="High tide"
      :lines="conditions ? highTideBlockLines(conditions) : []"
    />

    <InfoBlock
      shape="lowTide"
      label-pt="Maré vazia"
      label-en="Low tide"
      :lines="conditions ? lowTideBlockLines(conditions) : []"
    />

    <div class="wordmark">
      <span class="wordmark__glyph" aria-hidden="true">J</span>
      <span class="wordmark__text">Fajã dos<br />Padres</span>
    </div>
  </main>
</template>

<style scoped>
.poster {
  position: relative;
  width: 100%;
  max-width: 30rem;
  background: var(--color-poster-bg);
  border-radius: 1.25rem;
  box-shadow: 0 1.5rem 3rem rgba(20, 40, 65, 0.18);
  padding: 1.75rem 1.5rem;

  display: grid;
  grid-template-columns: 1.15fr 1fr;
  grid-template-rows: repeat(8, 1fr);
  gap: 0.9rem;
  aspect-ratio: 3 / 4.1;
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

.poster > * {
  position: relative;
  z-index: 1;
}

.wordmark {
  grid-area: wordmark;
  align-self: end;
  display: flex;
  align-items: flex-end;
  gap: 0.4rem;
  color: var(--color-text-label);
}

.wordmark__glyph {
  font-family: var(--font-serif);
  font-weight: 700;
  font-size: clamp(1.6rem, 4vw, 2.1rem);
  line-height: 0.8;
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
