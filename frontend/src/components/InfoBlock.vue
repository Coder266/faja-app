<script setup lang="ts">
/**
 * One pebble-shaped info panel on the poster (e.g. "Data / Date — 03/07").
 * `shape` both places the block in the poster grid and picks its blob
 * silhouette/tint, so the six blocks read as distinct organic pebbles.
 *
 * Values are passed in as `lines` rather than hardcoded here so a later
 * story (S7) can bind real API data without touching this component.
 */
export type BlockShape = 'data' | 'temperature' | 'water' | 'uv' | 'highTide' | 'lowTide'

export interface ValueLine {
  /** Text to render for this line. */
  text: string
  /** Relative size of the handwritten line; defaults to 'lg'. */
  size?: 'lg' | 'sm'
}

withDefaults(
  defineProps<{
    shape: BlockShape
    labelPt: string
    labelEn?: string
    lines: ValueLine[]
    /** True before the first fetch resolves; renders shimmer placeholders
     * instead of `lines`, sized to match what will render so the layout
     * doesn't jump once real values arrive. */
    loading?: boolean
    /** Sizes/count of the placeholder bars shown while `loading`; should
     * mirror the typical shape of this block's real `lines` output. */
    placeholderSizes?: Array<'lg' | 'sm'>
  }>(),
  {
    loading: false,
    placeholderSizes: () => ['lg'],
  },
)
</script>

<template>
  <div class="block" :class="`block--${shape}`" :aria-busy="loading || undefined">
    <div class="block__content">
      <div class="block__label">
        <p class="block__label-pt">{{ labelPt }}</p>
        <p v-if="labelEn" class="block__label-en">{{ labelEn }}</p>
      </div>
      <div class="block__value">
        <template v-if="loading">
          <span
            v-for="(size, i) in placeholderSizes"
            :key="i"
            class="block__placeholder"
            :class="`block__placeholder--${size}`"
            aria-hidden="true"
          />
        </template>
        <template v-else>
          <p
            v-for="(line, i) in lines"
            :key="i"
            class="block__line"
            :class="`block__line--${line.size ?? 'lg'}`"
          >
            {{ line.text }}
          </p>
        </template>
      </div>
    </div>
  </div>
</template>

<style scoped>
.block {
  position: relative;
  display: flex;
  flex-direction: column;
  /* Center the label+value content box on the pebble (vertically via
     justify-content, horizontally via align-items) so it sits in the
     ellipse's wide middle band rather than spilling past the curved edges.
     The text inside stays left-aligned (see .block__content) — centering the
     box, not the text, keeps it off the left curve without leaving a big gap
     on the right. */
  justify-content: center;
  align-items: center;
  padding: 0.9rem 1.6rem;
  color: var(--color-text-label);
  background: var(--color-blob);
  /* Organic pebble silhouette shared by all blocks; each shape variant below
     nudges the corner radii so no two pebbles look identical. */
  border-radius: 63% 37% 54% 46% / 43% 51% 49% 57%;
}

/* Shrink-wraps to its widest line and is centered by .block; everything
   inside is left-aligned, so all lines share one left edge nudged toward the
   pebble's centre. */
.block__content {
  display: flex;
  flex-direction: column;
  gap: 0.15em;
  text-align: left;
}

.block__label-pt {
  font-family: var(--font-serif);
  font-weight: 700;
  font-size: clamp(0.95rem, 2.3vw, 1.2rem);
  line-height: 1.15;
}

.block__label-en {
  font-family: var(--font-serif);
  font-weight: 400;
  font-size: clamp(0.7rem, 1.5vw, 0.85rem);
  color: var(--color-text-label-soft);
  margin-top: 0.1em;
}

.block__value {
  margin-top: 0.15em;
  display: flex;
  flex-direction: column;
  align-items: flex-start;
}

.block__line {
  font-family: var(--font-hand);
  color: var(--color-text-hand);
  line-height: 1.05;
}

.block__line--lg {
  font-size: clamp(1.5rem, 4.2vw, 2.15rem);
}

.block__line--sm {
  font-size: clamp(1rem, 2.6vw, 1.3rem);
}

/* Loading placeholder bars: sized to match block__line--lg/--sm so a block
   doesn't change height once real values replace them. */
.block__placeholder {
  display: block;
  width: 55%;
  border-radius: 0.35em;
  background: linear-gradient(
    100deg,
    rgba(255, 255, 255, 0.12) 30%,
    rgba(255, 255, 255, 0.32) 50%,
    rgba(255, 255, 255, 0.12) 70%
  );
  background-size: 250% 100%;
  animation: block__placeholder-shimmer 1.8s ease-in-out infinite;
}

.block__placeholder + .block__placeholder {
  margin-top: 0.35em;
}

.block__placeholder--lg {
  height: clamp(1.5rem, 4.2vw, 2.15rem);
  width: 60%;
}

.block__placeholder--sm {
  height: clamp(1rem, 2.6vw, 1.3rem);
  width: 45%;
}

@keyframes block__placeholder-shimmer {
  0% {
    background-position: 200% 0;
  }
  100% {
    background-position: -200% 0;
  }
}

@media (prefers-reduced-motion: reduce) {
  .block__placeholder {
    animation: none;
    background: rgba(255, 255, 255, 0.2);
  }
}

/* Position within the desktop poster grid (see PosterBoard.vue) and a
   slightly different blob silhouette/tint per block for visual variety. */
.block--data {
  grid-area: data;
  border-radius: 68% 32% 42% 58% / 55% 42% 58% 45%;
}

.block--temperature {
  grid-area: temperature;
  background: var(--color-blob-alt);
  border-radius: 38% 62% 55% 45% / 48% 45% 55% 52%;
}

.block--water {
  grid-area: water;
  border-radius: 48% 52% 62% 38% / 60% 38% 62% 40%;
  /* "Temperatura da água" is the longest label and this is one of the
     narrower pebbles, so on the desktop grid its widest line poked out the
     right curve. Grow this one block into the surrounding grid gap for a
     bigger pebble... */
  margin: -0.55rem;
}

/* ...and cap the label width so it wraps as "Temperatura / da água" (two
   short lines) instead of a single wide "Temperatura da" that overhangs the
   curve. em-based so it tracks the responsive label font size. */
.block--water .block__label-pt {
  max-width: 7.5em;
}

/* On mobile the pebble is a roomy full-width ellipse — no enlargement or
   forced wrap needed there, so undo both. */
@media (max-width: 640px) {
  .block--water {
    margin: 0;
  }

  .block--water .block__label-pt {
    max-width: none;
  }
}

.block--uv {
  grid-area: uv;
  background: var(--color-blob-alt);
  border-radius: 55% 45% 40% 60% / 42% 55% 45% 58%;
}

.block--highTide {
  grid-area: highTide;
  border-radius: 42% 58% 60% 40% / 55% 48% 52% 45%;
}

.block--lowTide {
  grid-area: lowTide;
  background: var(--color-blob-alt);
  border-radius: 60% 40% 45% 55% / 40% 58% 42% 60%;
}
</style>
