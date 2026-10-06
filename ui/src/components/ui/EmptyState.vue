<script setup>
/**
 * What a list says when it has nothing in it.
 *
 * An empty screen is an invitation, so this says what to do next rather
 * than only reporting the absence.
 */
defineProps({
  title: { type: String, required: true },
  message: { type: String, default: "" },
  icon: { type: [Object, Function], default: null },
});
</script>

<template>
  <div class="flex flex-col items-center gap-2 px-6 py-14 text-center">
    <span
      v-if="icon"
      class="mb-1 flex size-11 items-center justify-center rounded-xl bg-muted text-ink-3"
    >
      <component :is="icon" class="size-5" :stroke-width="1.8" />
    </span>
    <p class="font-medium text-ink">{{ title }}</p>
    <p v-if="message" class="max-w-[44ch] text-ink-4">{{ message }}</p>
    <div v-if="$slots.default" class="mt-3 flex justify-center gap-2"><slot /></div>
  </div>
</template>
