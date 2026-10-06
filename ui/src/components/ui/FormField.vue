<script setup>
import { useId } from "vue";

/**
 * A label, its control, and the hint or error beneath it.
 *
 * The slot receives `id` and `invalid`, to put on the control so the label
 * and the error are tied to it.
 */
const props = defineProps({
  label: { type: String, required: true },
  hint: { type: String, default: "" },
  error: { type: String, default: "" },
});

const id = useId();
</script>

<template>
  <div class="flex flex-col gap-1.5">
    <label :for="id" class="label">{{ props.label }}</label>
    <slot :id="id" :invalid="error ? 'true' : undefined" :describedby="`${id}-note`" />
    <p v-if="error" :id="`${id}-note`" class="field-error">{{ error }}</p>
    <p v-else-if="hint" :id="`${id}-note`" class="hint">{{ hint }}</p>
  </div>
</template>
