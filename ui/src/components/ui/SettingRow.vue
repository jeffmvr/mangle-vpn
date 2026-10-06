<script setup>
import { useId } from "vue";

/**
 * One setting in a list of them: its name and a short explanation on the
 * left, its control on the right, so a short form fills a wide page without
 * stretching any field. On a narrow screen the control drops below.
 *
 * The slot receives `id`, `invalid` and `describedby`, as FormField's does.
 */
const props = defineProps({
  label: { type: String, required: true },
  description: { type: String, default: "" },
  error: { type: String, default: "" },
});

const id = useId();
</script>

<template>
  <div
    class="grid gap-3 border-b border-hair py-5 first:pt-0 last:border-b-0 last:pb-0 sm:grid-cols-[minmax(0,2fr)_minmax(0,3fr)] sm:gap-8"
  >
    <div class="flex min-w-0 flex-col gap-0.5 sm:pt-2">
      <label :for="id" class="font-medium">{{ props.label }}</label>
      <p v-if="description" class="hint">{{ description }}</p>
    </div>
    <div class="flex max-w-[480px] min-w-0 flex-col items-start gap-1.5 [&>.input:not(.w-28)]:w-full">
      <slot :id="id" :invalid="error ? 'true' : undefined" :describedby="`${id}-note`" />
      <p v-if="error" :id="`${id}-note`" class="field-error">{{ error }}</p>
    </div>
  </div>
</template>
