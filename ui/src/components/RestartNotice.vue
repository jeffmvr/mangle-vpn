<script setup>
import { TriangleAlert } from "lucide-vue-next";

import { errorMessage } from "@/lib/api";
import { useSession } from "@/stores/session";
import { useToast } from "@/stores/toast";

/**
 * Tells an administrator that saved settings are waiting on an OpenVPN
 * restart, with the restart one click away. Nobody else can act on it, so
 * nobody else sees it.
 */
const session = useSession();
const toast = useToast();

async function restart() {
  try {
    await session.restartVpn();
    toast.success("OpenVPN restarted. The new settings are live.");
  } catch (error) {
    toast.error(errorMessage(error, "OpenVPN could not be restarted."));
  }
}
</script>

<template>
  <div
    v-if="session.isAdmin && session.restartPending"
    class="flex items-center gap-2.5 rounded-full border border-warn-line bg-warn-soft py-1 pr-1 pl-3 text-[13px] text-warn-ink"
  >
    <TriangleAlert class="size-4 shrink-0" :stroke-width="2" />
    <span class="hidden md:inline">Changes need an OpenVPN restart</span>
    <span class="hidden sm:inline md:hidden">Restart needed</span>
    <button
      type="button"
      :disabled="session.vpnBusy"
      class="h-7 rounded-full bg-warn px-3 text-xs font-semibold whitespace-nowrap text-white hover:bg-warn-ink disabled:opacity-60"
      @click="restart"
    >
      {{ session.vpnBusy ? "Restarting…" : "Restart now" }}
    </button>
  </div>
</template>
