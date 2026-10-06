<script setup>
import { CircleAlert, CircleCheck, X } from "lucide-vue-next";

import { useToast } from "@/stores/toast";

const toast = useToast();
</script>

<template>
  <div
    aria-live="polite"
    class="pointer-events-none fixed right-4 bottom-4 z-[60] flex w-[calc(100%-2rem)] max-w-sm flex-col gap-2"
  >
    <TransitionGroup
      enter-active-class="transition duration-200 ease-out"
      enter-from-class="translate-y-2 opacity-0"
      leave-active-class="transition duration-150 ease-in"
      leave-to-class="opacity-0"
    >
      <div
        v-for="item in toast.toasts"
        :key="item.id"
        class="pointer-events-auto flex items-start gap-3 rounded-xl border border-line bg-surface px-4 py-3 shadow-[0_8px_24px_rgba(16,24,40,0.12)]"
      >
        <CircleCheck v-if="item.tone === 'success'" class="mt-px size-[18px] shrink-0 text-up" :stroke-width="2" />
        <CircleAlert v-else class="mt-px size-[18px] shrink-0 text-deny" :stroke-width="2" />
        <p class="flex-1 text-ink">{{ item.message }}</p>
        <button
          type="button"
          aria-label="Dismiss"
          class="-mr-1 flex size-6 items-center justify-center rounded-md text-ink-4 hover:bg-muted hover:text-ink"
          @click="toast.dismiss(item.id)"
        >
          <X class="size-4" :stroke-width="2" />
        </button>
      </div>
    </TransitionGroup>
  </div>
</template>
