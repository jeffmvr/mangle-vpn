<script setup>
import { ChevronLeft } from "lucide-vue-next";

/**
 * A page's title and one-line description, with its actions opposite.
 * Given `back`, a link to the parent list sits above the title.
 */
defineProps({
  title: { type: String, required: true },
  description: { type: String, default: "" },
  back: { type: Object, default: null },
});
</script>

<template>
  <div class="flex flex-col gap-3">
    <RouterLink
      v-if="back"
      :to="back.to"
      class="inline-flex w-fit items-center gap-1 text-[13px] font-medium text-ink-3 no-underline hover:text-accent"
    >
      <ChevronLeft class="size-4" :stroke-width="2" />
      {{ back.label }}
    </RouterLink>

    <div class="flex flex-wrap items-end justify-between gap-4">
      <div class="flex min-w-0 items-center gap-4">
        <slot name="leading" />
        <div class="flex min-w-0 flex-col gap-1">
          <h1 class="truncate text-[26px] leading-tight font-semibold tracking-[-0.02em]">
            {{ title }}
          </h1>
          <p v-if="description || $slots.description" class="text-ink-3">
            <slot name="description">{{ description }}</slot>
          </p>
        </div>
      </div>

      <div v-if="$slots.actions" class="flex flex-wrap items-center gap-2 max-sm:w-full">
        <slot name="actions" />
      </div>
    </div>
  </div>
</template>
