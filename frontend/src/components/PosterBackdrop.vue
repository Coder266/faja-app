<script setup lang="ts">
/**
 * Purely decorative pebble shapes scattered behind the six info blocks,
 * matching the scattered blob texture visible around the panels on the
 * physical poster. Not interactive and carries no content.
 */
</script>

<template>
  <svg
    class="backdrop"
    viewBox="0 0 100 140"
    preserveAspectRatio="none"
    aria-hidden="true"
    focusable="false"
  >
    <path
      class="backdrop__blob"
      d="M60 8 C 70 4, 82 10, 80 20 C 78 30, 64 30, 56 26 C 48 22, 50 12, 60 8 Z"
    />
    <path
      class="backdrop__blob"
      d="M10 45 C 22 40, 34 46, 32 56 C 30 65, 14 66, 6 60 C -2 54, -2 50, 10 45 Z"
    />
    <path
      class="backdrop__blob"
      d="M52 58 C 64 54, 74 62, 70 72 C 66 80, 50 80, 44 72 C 39 65, 42 62, 52 58 Z"
    />
    <path
      class="backdrop__blob"
      d="M8 88 C 20 84, 30 90, 27 100 C 24 108, 8 108, 2 100 C -3 93, -2 92, 8 88 Z"
    />
    <path
      class="backdrop__blob"
      d="M78 100 C 90 96, 100 104, 96 114 C 92 122, 78 122, 72 114 C 67 107, 68 104, 78 100 Z"
    />
    <path
      class="backdrop__blob"
      d="M38 118 C 50 114, 60 120, 57 130 C 54 137, 40 138, 34 131 C 29 124, 30 121, 38 118 Z"
    />
  </svg>
</template>

<style scoped>
.backdrop {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  z-index: 0;
  pointer-events: none;
}

.backdrop__blob {
  fill: var(--color-blob-soft);
}
</style>
