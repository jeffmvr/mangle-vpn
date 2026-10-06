<script setup>
import ToggleSwitch from "./ToggleSwitch.vue";

/**
 * A setting that is on or off, with a sentence saying what it does.
 */
defineProps({
  label: { type: String, required: true },
  description: { type: String, default: "" },
});

const checked = defineModel({ type: Boolean, default: false });
</script>

<template>
  <div
    class="flex items-center justify-between gap-4 rounded-[10px] border border-hair bg-subtle p-3.5"
  >
    <div class="flex flex-col gap-0.5">
      <span class="font-medium">{{ label }}</span>
      <span v-if="description" class="text-[13px] text-ink-3">{{ description }}</span>
    </div>
    <ToggleSwitch v-model="checked" :label="label" />
  </div>
</template>
