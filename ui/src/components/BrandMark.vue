<script setup>
import { Shield } from "lucide-vue-next";

import { useSession } from "@/stores/session";

/** The logo square and the organisation's name. */
const session = useSession();
</script>

<template>
  <RouterLink to="/" class="flex shrink-0 items-center gap-2.5 text-ink no-underline">
    <img v-if="session.logoUrl" :src="session.logoUrl" alt="" class="h-[30px] max-w-[120px] object-contain" />
    <span v-else class="flex size-[30px] items-center justify-center rounded-lg bg-ink text-white">
      <Shield class="size-4" :stroke-width="2" />
    </span>
    <span class="text-[15px] font-semibold tracking-[-0.01em] whitespace-nowrap">{{ session.organization }} VPN</span>
  </RouterLink>
</template>
