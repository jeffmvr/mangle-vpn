<script setup>
import { computed } from "vue";

import AppButton from "./AppButton.vue";

/**
 * The footer under a paged table: where you are, and previous and next.
 */
const props = defineProps({
  total: { type: Number, default: 0 },
  pageSize: { type: Number, default: 15 },
  noun: { type: String, default: "results" },
});

const page = defineModel("page", { type: Number, default: 1 });

const lastPage = computed(() => Math.max(1, Math.ceil(props.total / props.pageSize)));
const first = computed(() => (props.total === 0 ? 0 : (page.value - 1) * props.pageSize + 1));
const last = computed(() => Math.min(props.total, page.value * props.pageSize));
</script>

<template>
  <div
    class="flex items-center justify-between gap-3 border-t border-hair px-5 py-3 text-[13px] text-ink-3"
  >
    <span v-if="total > pageSize">Showing {{ first }}–{{ last }} of {{ total }} {{ noun }}</span>
    <span v-else>{{ total }} {{ noun }}</span>

    <div v-if="lastPage > 1" class="flex gap-2">
      <AppButton size="sm" :disabled="page <= 1" @click="page--">Previous</AppButton>
      <AppButton size="sm" :disabled="page >= lastPage" @click="page++">Next</AppButton>
    </div>
  </div>
</template>
