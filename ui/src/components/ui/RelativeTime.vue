<script setup>
import { computed } from "vue";

import { dateTime, relative } from "@/lib/format";

/** A time relative to now, with the exact time on hover. */
const props = defineProps({
  value: { type: String, default: "" },
  never: { type: String, default: "Never" },
  // Lowercase, for use mid-sentence.
  lower: { type: Boolean, default: false },
});

const text = computed(() => {
  const words = relative(props.value, props.never);
  return props.lower ? words.toLowerCase() : words;
});
</script>

<template>
  <time v-if="value" :datetime="value" :title="dateTime(value)">{{ text }}</time>
  <span v-else>{{ text }}</span>
</template>
