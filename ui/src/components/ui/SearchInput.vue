<script setup>
import { Search } from "lucide-vue-next";

/** A search box. The value updates as the user types. */
defineProps({
  label: { type: String, required: true },
  placeholder: { type: String, default: "Search" },
});

const model = defineModel({ type: String, default: "" });
</script>

<template>
  <label class="relative flex w-full items-center sm:w-72">
    <span class="sr-only">{{ label }}</span>
    <Search class="pointer-events-none absolute left-3 size-4 text-ink-4" :stroke-width="2" />
    <input v-model="model" type="search" :placeholder="placeholder" class="input pl-9" />
  </label>
</template>
