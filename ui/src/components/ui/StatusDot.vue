<script setup>
/**
 * A dot with an optional word, carrying one piece of state.
 */
defineProps({
  state: { type: String, default: "up" },
  label: { type: String, default: "" },
  // The word in plain text, leaving the dot to carry the state.
  muted: { type: Boolean, default: false },
});

const dots = {
  up: "bg-up-dot",
  off: "bg-ink-5",
  pending: "bg-warn",
  deny: "bg-deny",
};

const words = {
  up: "text-up",
  off: "text-ink-3",
  pending: "text-warn",
  deny: "text-deny",
};
</script>

<template>
  <span class="inline-flex items-center gap-1.5 text-[13px] font-medium" :class="muted ? 'font-normal text-ink-3' : words[state]">
    <span class="inline-block size-[7px] shrink-0 rounded-full" :class="dots[state]" />
    <template v-if="label">{{ label }}</template>
  </span>
</template>
