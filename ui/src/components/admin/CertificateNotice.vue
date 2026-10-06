<script setup>
import { ShieldAlert } from "lucide-vue-next";
import { computed, onMounted, ref } from "vue";

import api from "@/lib/api";
import { shortDate } from "@/lib/format";
import { useSession } from "@/stores/session";

/**
 * Warns administrators, in the header, about a certificate that expires
 * within thirty days. Let's Encrypt renews its own well before then, so
 * this is mostly about the certificate authority and uploaded certificates.
 */
const session = useSession();
const certificates = ref([]);

const expiring = computed(() =>
  certificates.value
    .filter((certificate) => certificate.expires_soon)
    .sort((a, b) => a.not_after.localeCompare(b.not_after)),
);

onMounted(async () => {
  // Only administrators can act on it, in the settings.
  if (!session.isAdmin) return;
  try {
    const { data } = await api.get("/admin/certificates");
    certificates.value = data;
  } catch {
    // The settings page shows them too; the header can do without.
  }
});
</script>

<template>
  <RouterLink
    v-if="session.isAdmin && expiring.length"
    :to="{ name: 'admin-settings', params: { section: 'general' } }"
    class="flex items-center gap-2 rounded-full border border-warn-line bg-warn-soft py-1.5 pr-3 pl-3 text-[13px] text-warn-ink no-underline hover:bg-warn-line/40"
  >
    <ShieldAlert class="size-4 shrink-0" :stroke-width="2" />
    <span class="hidden md:inline">
      {{ expiring[0].label }} certificate expires {{ shortDate(expiring[0].not_after) }}
    </span>
    <span class="hidden sm:inline md:hidden">Certificate expiring</span>
  </RouterLink>
</template>
