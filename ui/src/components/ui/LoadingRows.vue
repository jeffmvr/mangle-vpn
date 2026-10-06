<script setup>
/**
 * Placeholder rows shown while a list loads for the first time, so the card
 * keeps roughly its final shape instead of appearing empty and then jumping.
 */
defineProps({
  rows: { type: Number, default: 5 },
});

// Varied widths read as text rather than as a striped block.
const widths = ["w-2/5", "w-1/3", "w-1/2", "w-1/4", "w-2/5", "w-1/3"];
</script>

<template>
  <div role="status" aria-label="Loading" class="divide-y divide-hair">
    <div v-for="row in rows" :key="row" class="flex items-center gap-3 px-5 py-4">
      <span class="size-8 shrink-0 animate-pulse rounded-full bg-muted" />
      <span class="h-3 animate-pulse rounded-full bg-muted" :class="widths[row % widths.length]" />
      <span class="ml-auto h-3 w-16 animate-pulse rounded-full bg-muted" />
    </div>
  </div>
</template>
