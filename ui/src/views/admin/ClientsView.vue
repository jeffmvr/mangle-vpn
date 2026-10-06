<script setup>
import { Power, Unplug } from "lucide-vue-next";
import { onMounted, onUnmounted, ref, watch } from "vue";

import AppButton from "@/components/ui/AppButton.vue";
import ConfirmDialog from "@/components/ui/ConfirmDialog.vue";
import EmptyState from "@/components/ui/EmptyState.vue";
import LoadingRows from "@/components/ui/LoadingRows.vue";
import PageHeader from "@/components/ui/PageHeader.vue";
import SearchInput from "@/components/ui/SearchInput.vue";
import StatusDot from "@/components/ui/StatusDot.vue";
import TablePager from "@/components/ui/TablePager.vue";
import UserAvatar from "@/components/ui/UserAvatar.vue";
import api, { errorMessage } from "@/lib/api";
import { bytes, duration } from "@/lib/format";
import { useSession } from "@/stores/session";
import { useToast } from "@/stores/toast";

const PAGE_SIZE = 15;
const REFRESH_MS = 5000;

const session = useSession();
const toast = useToast();

const clients = ref([]);
const total = ref(0);
const page = ref(1);
const search = ref("");
const loaded = ref(false);
const disconnecting = ref(null);
let poll = null;

/** Loads the current page of connected clients. */
async function load() {
  const { data } = await api.get("/admin/clients", {
    params: { search: search.value, page: page.value, size: PAGE_SIZE },
  });
  clients.value = data.results;
  total.value = data.count;
  loaded.value = true;
}

/** Drops a client's connection. */
async function disconnect() {
  const client = disconnecting.value;
  try {
    await api.delete(`/admin/clients/${client.id}`);
    toast.success(`${client.device.name} disconnected.`);
    await load();
  } catch (error) {
    toast.error(errorMessage(error, "The client could not be disconnected."));
    throw error;
  }
}

/** Starts OpenVPN from the empty list. */
async function start() {
  try {
    await session.toggleVpn();
    toast.success("OpenVPN started.");
  } catch (error) {
    toast.error(errorMessage(error, "OpenVPN could not be started."));
  }
}

// Searching returns to the first page, or the results can look empty.
watch(search, () => {
  page.value = 1;
  load();
});

watch(page, load);

onMounted(() => {
  load();
  poll = setInterval(load, REFRESH_MS);
});

onUnmounted(() => clearInterval(poll));
</script>

<template>
  <PageHeader title="Connected clients" description="Devices with an active VPN session right now.">
    <template #actions>
      <SearchInput v-model="search" label="Search clients" placeholder="Search by user, device or IP" />
    </template>
  </PageHeader>

  <div class="card overflow-hidden lg:overflow-visible">
    <LoadingRows v-if="!loaded" />
    <div v-else-if="clients.length" class="overflow-x-auto lg:overflow-visible">
      <table class="w-full border-collapse">
        <thead>
          <tr>
            <th class="th">User</th>
            <th class="th hidden sm:table-cell">Device</th>
            <th class="th hidden md:table-cell">VPN address</th>
            <th class="th hidden lg:table-cell">Connected from</th>
            <th class="th hidden sm:table-cell">Connected for</th>
            <th class="th hidden xl:table-cell">Data down / up</th>
            <th class="th"><span class="sr-only">Actions</span></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="client in clients" :key="client.id" class="row">
            <td class="td">
              <div class="flex items-center gap-2.5">
                <UserAvatar :email="client.device.user.email" />
                <div class="flex min-w-0 flex-col gap-0.5">
                  <RouterLink :to="`/admin/users/${client.device.user.id}`" class="link truncate">
                    {{ client.device.user.email }}
                  </RouterLink>
                  <span class="text-[13px] text-ink-4 sm:hidden">
                    {{ client.device.name }}, {{ duration(client.duration) }}
                  </span>
                </div>
              </div>
            </td>
            <td class="td hidden text-ink-2 sm:table-cell">{{ client.device.name }}</td>
            <td class="td hidden font-mono text-[13px] text-ink-2 md:table-cell">{{ client.virtual_ip }}</td>
            <td class="td hidden font-mono text-[13px] text-ink-3 lg:table-cell">{{ client.remote_ip || "—" }}</td>
            <td class="td hidden sm:table-cell">
              <StatusDot state="up" class="!font-normal !text-ink-2 !text-sm" :label="duration(client.duration)" />
            </td>
            <!-- From the device's side: what it downloaded is what the server sent. -->
            <td class="td hidden whitespace-nowrap text-ink-3 tabular-nums xl:table-cell">
              {{ bytes(client.bytes_sent) }} / {{ bytes(client.bytes_received) }}
            </td>
            <td class="td text-right">
              <AppButton size="sm" variant="danger" @click="disconnecting = client">Disconnect</AppButton>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <EmptyState
      v-else-if="loaded && !search && session.vpnKnown && !session.vpnRunning"
      :icon="Power"
      title="OpenVPN is stopped"
      message="Nobody can connect until the server is started."
    >
      <AppButton variant="primary" :disabled="session.vpnBusy" @click="start">
        {{ session.vpnBusy ? "Starting…" : "Start server" }}
      </AppButton>
    </EmptyState>

    <EmptyState
      v-else-if="loaded"
      :icon="Unplug"
      :title="search ? 'No matching clients' : 'Nobody is connected'"
      :message="search ? 'Try a different search.' : 'Connected devices appear here within a few seconds.'"
    />

    <TablePager v-if="total" v-model:page="page" :total="total" :page-size="PAGE_SIZE" noun="connected" />
  </div>

  <ConfirmDialog
    :open="disconnecting !== null"
    :title="`Disconnect ${disconnecting?.device.name}?`"
    message="The session ends now. The device can reconnect straight away unless it is also revoked."
    confirm-label="Disconnect"
    :action="disconnect"
    @update:open="(value) => !value && (disconnecting = null)"
  />
</template>
