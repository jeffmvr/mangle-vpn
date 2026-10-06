<script setup>
import { ArrowUpRight, CircleHelp, LaptopMinimal, Plus } from "lucide-vue-next";
import { computed, ref } from "vue";

import DeviceMeta from "@/components/DeviceMeta.vue";
import NewDeviceDialog from "@/components/NewDeviceDialog.vue";
import SetupGuide from "@/components/SetupGuide.vue";
import AppButton from "@/components/ui/AppButton.vue";
import AppDialog from "@/components/ui/AppDialog.vue";
import ConfirmDialog from "@/components/ui/ConfirmDialog.vue";
import PageHeader from "@/components/ui/PageHeader.vue";
import api, { errorMessage } from "@/lib/api";
import { useSession } from "@/stores/session";
import { useToast } from "@/stores/toast";

const session = useSession();
const toast = useToast();

const adding = ref(false);
const guiding = ref(false);
const revoking = ref(null);

const devices = computed(() => session.profile?.devices ?? []);

// What to type into OpenVPN Connect: the address this page was reached on.
const serverAddress = window.location.host;

// What connecting asks for, which depends on the settings.
const signIn = computed(() => {
  if (!session.profile?.vpn_password_required) return "your email and the code from your authenticator app";
  return session.profile?.mfa_required
    ? "your email, your password, then the code from your authenticator app"
    : "your email and your password";
});

const STEP_NUMBER =
  "flex size-6 shrink-0 items-center justify-center rounded-full bg-accent-soft text-xs font-semibold text-accent";
function addFromGuide() {
  guiding.value = false;
  adding.value = true;
}

async function revoke() {
  const device = revoking.value;
  try {
    await api.delete(`/devices/${device.id}`);
    await session.loadProfile();
    toast.success(`${device.name} revoked.`);
  } catch (error) {
    toast.error(errorMessage(error, `${device.name} could not be revoked.`));
    throw error;
  }
}
</script>

<template>
  <div class="flex flex-col gap-7">
    <!-- With no devices yet, adding one is the one thing to do here. -->
    <section v-if="!devices.length" aria-labelledby="first-device-title" class="mx-auto flex w-full max-w-[760px] flex-col items-center pt-6 text-center sm:pt-12">
      <span class="flex size-14 items-center justify-center rounded-2xl bg-accent-soft text-accent">
        <LaptopMinimal class="size-7" :stroke-width="1.8" />
      </span>
      <h1 id="first-device-title" class="mt-5 text-2xl font-semibold tracking-[-0.015em]">Connect your first device</h1>
      <p class="mt-2 max-w-[520px] leading-relaxed text-ink-3">
        Each computer or phone gets its own VPN profile. It takes a couple of minutes.
      </p>
      <AppButton variant="primary" class="mt-7 !h-11 !px-5" :disabled="!session.canAddDevice" @click="adding = true">
        <Plus class="size-4" :stroke-width="2.2" />
        New device
      </AppButton>

      <ol class="mt-12 grid w-full gap-3 text-left sm:grid-cols-3">
        <li class="card flex flex-col gap-2 p-5">
          <span :class="STEP_NUMBER">1</span>
          <span class="mt-1 font-semibold">Get the app</span>
          <span class="text-[13px] leading-relaxed text-ink-3">
            <a href="https://openvpn.net/client/" target="_blank" rel="noopener" class="inline-flex items-center gap-0.5 font-medium text-accent hover:text-accent-strong">
              OpenVPN Connect<ArrowUpRight class="size-3.5" :stroke-width="2" />
            </a>
            is free for macOS, Windows, Linux, iOS and Android.
          </span>
        </li>
        <li class="card flex flex-col gap-2 p-5">
          <span :class="STEP_NUMBER">2</span>
          <span class="mt-1 font-semibold">Add a device</span>
          <span class="text-[13px] leading-relaxed text-ink-3">
            Choose <span class="font-medium text-ink-2">New device</span>, then
            <span class="font-medium text-ink-2">Open in OpenVPN Connect</span>, or download the profile.
          </span>
        </li>
        <li class="card flex flex-col gap-2 p-5">
          <span :class="STEP_NUMBER">3</span>
          <span class="mt-1 font-semibold">Connect and sign in</span>
          <span class="text-[13px] leading-relaxed text-ink-3">Turn the profile on and sign in with {{ signIn }}.</span>
        </li>
      </ol>

      <p class="mt-6 max-w-[560px] text-[13px] leading-relaxed text-ink-4">
        Setting up from the app instead? In OpenVPN Connect, choose
        <span class="font-medium text-ink-3">Import Profile › URL</span>, enter
        <code class="rounded bg-muted px-1.5 py-px font-mono text-[12px] text-ink-3">{{ serverAddress }}</code>
        and sign in.
      </p>
    </section>

    <template v-else>
      <PageHeader
        title="My devices"
        description="Each device gets its own OpenVPN profile. Revoke one if it's lost or retired."
      >
        <template #actions>
          <AppButton @click="guiding = true">
            <CircleHelp class="size-4" :stroke-width="2" />
            How to connect
          </AppButton>
          <AppButton variant="primary" :disabled="!session.canAddDevice" @click="adding = true">
            <Plus class="size-4" :stroke-width="2.2" />
            New device
          </AppButton>
        </template>
      </PageHeader>

      <section aria-label="Devices" class="card overflow-hidden">
        <div class="flex items-center justify-between border-b border-hair px-5 py-3.5">
          <h2 class="font-semibold">Devices</h2>
          <span class="text-[13px] text-ink-4">
            {{ session.deviceCount }} of {{ session.deviceLimit }} allowed
          </span>
        </div>

        <div
          v-for="device in devices"
          :key="device.id"
          class="flex items-center gap-3.5 border-b border-hair px-5 py-4 last:border-0"
        >
          <span
            class="relative flex size-[38px] shrink-0 items-center justify-center rounded-[10px] text-ink-2"
            :class="device.connected ? 'bg-up-soft text-up' : 'bg-muted'"
          >
            <LaptopMinimal class="size-[18px]" :stroke-width="2" />
          </span>
          <div class="flex min-w-0 flex-1 flex-col gap-0.5">
            <span class="truncate font-medium">{{ device.name }}</span>
            <DeviceMeta :device="device" />
          </div>
          <AppButton size="sm" variant="danger" @click="revoking = device">Revoke</AppButton>
        </div>
      </section>
    </template>

    <AppDialog v-model:open="guiding" title="How to connect" width="max-w-[600px]">
      <SetupGuide @add="addFromGuide" />
    </AppDialog>

    <NewDeviceDialog v-model:open="adding" />

    <ConfirmDialog
      :open="revoking !== null"
      :title="`Revoke ${revoking?.name}?`"
      message="It can no longer connect, and any session it has open is dropped."
      confirm-label="Revoke device"
      :action="revoke"
      @update:open="(value) => !value && (revoking = null)"
    />
  </div>
</template>
