<script setup>
/**
 * An on/off switch. It is a real checkbox underneath, so it is reachable
 * by keyboard and announced as one.
 */
defineProps({
  label: { type: String, required: true },
  disabled: { type: Boolean, default: false },
});

const checked = defineModel({ type: Boolean, default: false });
</script>

<template>
  <label class="relative inline-flex shrink-0 cursor-pointer items-center has-[:disabled]:cursor-not-allowed has-[:disabled]:opacity-50">
    <input
      v-model="checked"
      type="checkbox"
      role="switch"
      class="peer sr-only"
      :disabled="disabled"
      :aria-label="label"
    />
    <span
      class="h-6 w-11 rounded-full bg-field transition-colors peer-checked:bg-accent peer-focus-visible:outline-2 peer-focus-visible:outline-offset-2 peer-focus-visible:outline-accent"
    />
    <span
      class="absolute left-0.5 size-5 rounded-full bg-white shadow-[0_1px_3px_rgba(16,24,40,0.2)] transition-transform peer-checked:translate-x-5"
    />
  </label>
</template>
