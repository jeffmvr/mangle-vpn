<script setup>
/** A small label: a role, an action, a count. */
defineProps({
  tone: { type: String, default: "neutral" },
});

const tones = {
  neutral: "bg-muted text-ink-3",
  accent: "bg-accent-soft text-accent-strong",
  up: "bg-up-soft text-up",
  drop: "bg-drop-soft text-drop",
  warn: "bg-warn-soft text-warn-ink",
};
</script>

<template>
  <span
    class="inline-flex h-[22px] items-center gap-1 rounded-md px-2 text-xs font-semibold whitespace-nowrap"
    :class="tones[tone]"
  >
    <slot />
  </span>
</template>
