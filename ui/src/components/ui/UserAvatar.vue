<script setup>
import { computed } from "vue";

import { initials } from "@/lib/format";

/** A round avatar showing a person's initials. */
const props = defineProps({
  name: { type: String, default: "" },
  email: { type: String, default: "" },
  size: { type: String, default: "md" },
  tone: { type: String, default: "neutral" },
});

const letters = computed(() => initials(props.name, props.email));

const sizes = {
  sm: "size-7 text-[11px]",
  md: "size-8 text-xs",
  lg: "size-14 text-xl",
};

const tones = {
  neutral: "bg-muted text-ink-2",
  accent: "bg-accent-tint text-accent",
};
</script>

<template>
  <span
    class="inline-flex shrink-0 items-center justify-center rounded-full font-semibold"
    :class="[sizes[size], tones[tone]]"
    aria-hidden="true"
  >
    {{ letters }}
  </span>
</template>
