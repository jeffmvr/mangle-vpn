<script setup>
import { Download, ExternalLink, LockKeyhole } from "lucide-vue-next";
import { ref, watch } from "vue";

import AppButton from "@/components/ui/AppButton.vue";
import AppDialog from "@/components/ui/AppDialog.vue";
import FormField from "@/components/ui/FormField.vue";
import api, { errorMessage, fieldErrors } from "@/lib/api";
import { useSession } from "@/stores/session";
import { useToast } from "@/stores/toast";

/**
 * Creates a device and hands its OpenVPN profile over, either straight to
 * OpenVPN Connect through an import link or as a file to download.
 *
 * The server hands each profile out once, shortly after the device is
 * created, so this happens here rather than being left for later. The device
 * is created on the first attempt and reused if the user then switches
 * method, for instance when Connect turns out not to be installed.
 */
const open = defineModel("open", { type: Boolean, default: false });

const session = useSession();
const toast = useToast();

const systems = [
  { id: "macos", label: "macOS" },
  { id: "windows", label: "Windows" },
  { id: "linux", label: "Linux" },
];

const name = ref("");
const os = ref("macos");
const errors = ref({});
const busy = ref("");
const device = ref(null);
const opening = ref(false);

watch(open, (value) => {
  if (value) {
    name.value = "";
    errors.value = {};
    device.value = null;
    opening.value = false;
  }
});

/** Creates the device on the first call and returns it on every call. */
async function ensureDevice() {
  if (device.value) return device.value;

  const { data } = await api.post("/devices", { name: name.value, os: os.value });
  device.value = data;
  session.loadProfile();
  return data;
}

/** Reports a failure against the form's fields, or as a toast. */
async function report(error) {
  // A refused download comes back as a Blob, since a file was asked for.
  const body = error?.response?.data;
  if (body instanceof Blob) {
    try {
      error.response.data = JSON.parse(await body.text());
    } catch {
      // Not JSON; the generic message below will do.
    }
  }

  errors.value = fieldErrors(error);
  if (!errors.value.name && !errors.value.os) toast.error(errorMessage(error));
}

/** Sends the profile straight to OpenVPN Connect. */
async function openInConnect() {
  busy.value = "connect";
  errors.value = {};
  try {
    const created = await ensureDevice();
    const { data } = await api.post(`/devices/${created.id}/import-link`);
    window.location.href = data.url;
    opening.value = true;
  } catch (error) {
    await report(error);
  } finally {
    busy.value = "";
  }
}

/** Downloads the profile as a file. */
async function download() {
  busy.value = "download";
  errors.value = {};
  try {
    const created = await ensureDevice();
    const { data } = await api.get(`/devices/${created.id}`, {
      params: { os: os.value },
      responseType: "blob",
    });

    const link = document.createElement("a");
    link.href = URL.createObjectURL(data);
    link.download = `${session.organization} - ${created.name}.ovpn`;
    link.click();
    URL.revokeObjectURL(link.href);

    open.value = false;
    toast.success(`${created.name} added. Its profile has downloaded.`);
  } catch (error) {
    await report(error);
  } finally {
    busy.value = "";
  }
}

function finish() {
  open.value = false;
  toast.success(`${device.value.name} added.`);
}
</script>

<template>
  <AppDialog
    v-model:open="open"
    :title="opening ? 'Finish in OpenVPN Connect' : 'New device'"
    :description="
      opening
        ? `Confirm the import of ${device?.name} in OpenVPN Connect. The link works once, within five minutes of adding the device.`
        : 'We\'ll create an OpenVPN profile for this device and hand it to OpenVPN Connect.'
    "
    :busy="busy !== ''"
  >
    <template v-if="!opening">
      <form id="new-device" class="flex flex-col gap-5" @submit.prevent="openInConnect">
        <FormField v-slot="field" label="Device name" :error="errors.name">
          <input
            :id="field.id"
            v-model="name"
            type="text"
            maxlength="32"
            placeholder="Work laptop"
            autofocus
            :disabled="device !== null"
            class="input h-11"
            :aria-invalid="field.invalid"
            :aria-describedby="field.describedby"
          />
        </FormField>

        <fieldset class="flex flex-col gap-2" :disabled="device !== null">
          <legend class="label mb-2">Operating system</legend>
          <div class="grid grid-cols-3 gap-2">
            <label
              v-for="system in systems"
              :key="system.id"
              class="flex h-11 items-center justify-center gap-2 rounded-lg border transition-colors"
              :class="
                os === system.id
                  ? 'border-2 border-accent bg-accent-soft font-semibold text-accent-strong'
                  : 'border-field font-medium text-ink-2 hover:bg-subtle'
              "
            >
              <input v-model="os" type="radio" name="os" :value="system.id" class="accent-accent" />
              {{ system.label }}
            </label>
          </div>
          <p v-if="errors.os" class="field-error">{{ errors.os }}</p>
        </fieldset>
      </form>
    </template>

    <p v-else class="leading-relaxed text-ink-2">
      Nothing happened? Install
      <a href="https://openvpn.net/client/" target="_blank" rel="noopener" class="font-medium text-accent hover:text-accent-strong">OpenVPN Connect</a>
      and try again, or download the profile and import the file yourself.
    </p>

    <div class="flex gap-3 rounded-[10px] border border-warn-line bg-warn-soft px-4 py-3.5">
      <LockKeyhole class="mt-0.5 size-[18px] shrink-0 text-warn" :stroke-width="2" />
      <div class="flex flex-col gap-2 text-[13px] text-warn-ink">
        <div class="font-semibold">When OpenVPN Connect asks you to sign in</div>
        <dl class="grid grid-cols-[84px_minmax(0,1fr)] gap-x-3 gap-y-1">
          <dt class="font-medium">Username</dt>
          <dd class="truncate font-mono">{{ session.profile?.email }}</dd>
          <template v-if="session.profile?.vpn_password_required">
            <dt class="font-medium">Password</dt>
            <dd>your account password</dd>
            <template v-if="session.profile?.mfa_required">
              <dt class="font-medium">Code</dt>
              <dd>the current code from your authenticator app</dd>
            </template>
          </template>
          <template v-else>
            <dt class="font-medium">Password</dt>
            <dd>the current code from your authenticator app</dd>
          </template>
        </dl>
        <p>The profile can be fetched once. If you lose it, revoke the device and add it again.</p>
      </div>
    </div>

    <template #footer>
      <template v-if="!opening">
        <AppButton :disabled="busy !== ''" @click="open = false">Cancel</AppButton>
        <AppButton :disabled="busy !== ''" @click="download">
          <Download class="size-4" :stroke-width="2.2" />
          {{ busy === "download" ? "Downloading…" : "Download .ovpn" }}
        </AppButton>
        <AppButton variant="primary" type="submit" form="new-device" :disabled="busy !== ''">
          <ExternalLink class="size-4" :stroke-width="2.2" />
          {{ busy === "connect" ? "Opening…" : "Open in OpenVPN Connect" }}
        </AppButton>
      </template>
      <template v-else>
        <AppButton :disabled="busy !== ''" @click="openInConnect">Try again</AppButton>
        <AppButton :disabled="busy !== ''" @click="download">
          <Download class="size-4" :stroke-width="2.2" />
          {{ busy === "download" ? "Downloading…" : "Download instead" }}
        </AppButton>
        <AppButton variant="primary" @click="finish">Done</AppButton>
      </template>
    </template>
  </AppDialog>
</template>
