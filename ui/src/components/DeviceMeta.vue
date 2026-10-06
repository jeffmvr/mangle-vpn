<script setup>
import { computed } from "vue";

import RelativeTime from "@/components/ui/RelativeTime.vue";
import StatusDot from "@/components/ui/StatusDot.vue";
import { bytes, shortDate, systemName } from "@/lib/format";

/**
 * The line under a device's name: whether it is connected, what it runs and
 * how much data it has used.
 */
const props = defineProps({
  device: { type: Object, required: true },
});

const facts = computed(() => {
  const list = [systemName(props.device.os)];
  if (props.device.traffic) list.push(`${bytes(props.device.traffic)} used`);
  return list.filter(Boolean);
});
</script>

<template>
  <span class="flex flex-wrap items-center gap-x-1.5 text-[13px] text-ink-4">
    <StatusDot v-if="device.connected" state="up" label="Connected now" />
    <span v-else-if="device.last_login">Last connected <RelativeTime :value="device.last_login" lower /></span>
    <span v-else :title="`Added ${shortDate(device.created_at)}`">Not connected yet</span>
    <template v-for="fact in facts" :key="fact">
      <span aria-hidden="true">·</span>
      <span>{{ fact }}</span>
    </template>
  </span>
</template>
