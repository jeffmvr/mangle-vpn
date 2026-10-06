<script setup>
/**
 * The pages of one thing, such as a group or a user, as tabs under its
 * header. Each tab is a link, `{ id, label, icon, count, to }`, so each
 * page has its own address and a reload or a shared link lands on it.
 */
defineProps({
  items: { type: Array, required: true },
  label: { type: String, required: true },
});
</script>

<template>
  <nav :aria-label="label" class="-mb-1 flex max-w-full self-start overflow-x-auto rounded-xl bg-muted p-1">
    <RouterLink
      v-for="item in items"
      :key="item.id"
      :to="item.to"
      class="flex h-9 shrink-0 items-center gap-2 rounded-lg px-3 font-medium whitespace-nowrap text-ink-3 no-underline transition-colors hover:text-ink sm:px-3.5"
      active-class="!bg-surface !text-ink shadow-[0_1px_2px_rgba(16,24,40,0.08),0_0_0_1px_rgba(16,24,40,0.04)]"
    >
      <component :is="item.icon" v-if="item.icon" class="hidden size-4 shrink-0 sm:block" :stroke-width="2" />
      {{ item.label }}
      <span
        v-if="item.count !== undefined"
        class="min-w-5 rounded-full bg-hair px-1.5 text-center text-xs leading-5 text-ink-3 tabular-nums"
      >
        {{ item.count }}
      </span>
    </RouterLink>
  </nav>
</template>
