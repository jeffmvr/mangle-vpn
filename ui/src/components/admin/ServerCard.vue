<script setup>
import { ref } from "vue";

import AppButton from "@/components/ui/AppButton.vue";
import ConfirmDialog from "@/components/ui/ConfirmDialog.vue";
import StatusDot from "@/components/ui/StatusDot.vue";
import { errorMessage } from "@/lib/api";
import { useSession } from "@/stores/session";
import { useToast } from "@/stores/toast";

/** The OpenVPN server's state, with start, stop and restart. */
const session = useSession();
const toast = useToast();
const confirmingStop = ref(false);

async function run(action, success, failure) {
  try {
    await action();
    toast.success(success);
  } catch (error) {
    toast.error(errorMessage(error, failure));
    throw error;
  }
}

const restart = () => run(session.restartVpn, "OpenVPN restarted.", "OpenVPN could not be restarted.");
const start = () => run(session.toggleVpn, "OpenVPN started.", "OpenVPN could not be started.");
const stop = () => run(session.toggleVpn, "OpenVPN stopped.", "OpenVPN could not be stopped.");
</script>

<template>
  <div class="flex flex-col gap-3 rounded-xl border border-line p-3.5">
    <div class="flex items-center justify-between">
      <span class="text-[13px] font-semibold">OpenVPN server</span>
      <StatusDot
        :state="session.vpnRunning ? 'up' : 'off'"
        :label="session.vpnRunning ? 'Running' : 'Stopped'"
        class="!text-xs"
      />
    </div>

    <!-- The help desk sees the state but doesn't control the server. -->
    <template v-if="!session.isAdmin" />
    <div v-else-if="session.vpnRunning" class="grid grid-cols-2 gap-1.5">
      <AppButton size="sm" :disabled="session.vpnBusy" @click="restart().catch(() => {})">Restart</AppButton>
      <AppButton size="sm" variant="danger" :disabled="session.vpnBusy" @click="confirmingStop = true">
        Stop
      </AppButton>
    </div>
    <AppButton v-else size="sm" variant="primary" :disabled="session.vpnBusy" @click="start().catch(() => {})">
      {{ session.vpnBusy ? "Starting…" : "Start server" }}
    </AppButton>

    <ConfirmDialog
      v-model:open="confirmingStop"
      title="Stop the OpenVPN server?"
      message="Every connected client is disconnected, and nobody can connect until it is started again."
      confirm-label="Stop server"
      :action="stop"
    />
  </div>
</template>
