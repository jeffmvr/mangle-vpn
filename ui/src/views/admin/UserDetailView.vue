<script setup>
import { Activity, Check, Copy, LaptopMinimal, UserPen } from "lucide-vue-next";
import { computed, ref, watch } from "vue";
import {
  onBeforeRouteLeave,
  onBeforeRouteUpdate,
  useRoute,
  useRouter,
} from "vue-router";

import DeviceMeta from "@/components/DeviceMeta.vue";
import AppBadge from "@/components/ui/AppBadge.vue";
import AppButton from "@/components/ui/AppButton.vue";
import AppDialog from "@/components/ui/AppDialog.vue";
import ConfirmDialog from "@/components/ui/ConfirmDialog.vue";
import EmptyState from "@/components/ui/EmptyState.vue";
import FormField from "@/components/ui/FormField.vue";
import PageHeader from "@/components/ui/PageHeader.vue";
import PageTabs from "@/components/ui/PageTabs.vue";
import RelativeTime from "@/components/ui/RelativeTime.vue";
import StatusDot from "@/components/ui/StatusDot.vue";
import SettingRow from "@/components/ui/SettingRow.vue";
import ToggleSwitch from "@/components/ui/ToggleSwitch.vue";
import UserAvatar from "@/components/ui/UserAvatar.vue";
import api, { errorMessage, fieldErrors } from "@/lib/api";
import { eventKind, eventSummary } from "@/lib/activity";
import { bytes, duration, timeOfDay } from "@/lib/format";
import { useSession } from "@/stores/session";
import { useToast } from "@/stores/toast";

const route = useRoute();
const router = useRouter();
const session = useSession();
const toast = useToast();

const user = ref(null);
const groups = ref([]);
const devices = ref([]);
const sessions = ref([]);
const events = ref([]);

// Which of the user's pages is showing comes from the address.
const section = computed(() => route.params.section);

const sections = computed(() =>
  [
    { id: "details", label: "Details", icon: UserPen },
    {
      id: "devices",
      label: "Devices",
      icon: LaptopMinimal,
      count: devices.value.length,
    },
    { id: "activity", label: "Activity", icon: Activity },
  ].map((item) => ({
    ...item,
    to: {
      name: "admin-user",
      params: { id: route.params.id, section: item.id },
    },
  })),
);
const form = ref({});
const errors = ref({});
const saving = ref(false);

const confirming = ref(""); // "password" | "mfa" | "delete" | ""
const revoking = ref(null);
// The link to choose a new password, after a reset, and whether it was
// e-mailed too.
const resetLink = ref("");
const resetEmailed = ref(false);
const copied = ref(false);

// The device whose fixed address is being changed, and the address typed.
const addressing = ref(null);
const address = ref("");
const addressError = ref("");
const addressBusy = ref(false);

const isSelf = computed(() => user.value?.id === session.profile?.id);

// Administrators change accounts. The help desk may only help members back
// in: unlock, reset, and revoke a lost device.
const canEdit = computed(() => session.isAdmin);
const canHelp = computed(
  () => session.isAdmin || user.value?.role === "member",
);

const ROLE_BADGES = { admin: "Admin", helpdesk: "Help desk" };

/** The select's value for the three-way two-factor setting. */
const mfaChoice = computed({
  get: () =>
    form.value.mfa_enforced === null
      ? "inherit"
      : String(form.value.mfa_enforced),
  set: (value) => {
    form.value.mfa_enforced = value === "inherit" ? null : value === "true";
  },
});

function fill(data) {
  user.value = data;
  form.value = {
    name: data.name,
    email: data.email,
    group_id: data.group_id,
    role: data.role,
    is_enabled: data.is_enabled,
    mfa_enforced: data.mfa_enforced,
  };
  saved.value = JSON.stringify(form.value);
}

// The details as last loaded or saved, to tell whether anything has been
// edited since, as the settings page does.
const saved = ref("");
const dirty = computed(
  () => saved.value !== "" && JSON.stringify(form.value) !== saved.value,
);

function discard() {
  form.value = JSON.parse(saved.value);
  errors.value = {};
}

// Leaving the user with unsaved edits asks first. Moving between their
// own pages keeps them.
const leaving = ref(false);
let decide = null;

function confirmLeave(to) {
  if (!dirty.value || to.params.id === route.params.id) return true;
  leaving.value = true;
  return new Promise((resolve) => (decide = resolve));
}

function settle(leave) {
  decide?.(leave);
  decide = null;
}

watch(leaving, (open) => !open && settle(false));
onBeforeRouteLeave(confirmLeave);
onBeforeRouteUpdate(confirmLeave);

async function load() {
  const id = route.params.id;
  const [userResponse, groupsResponse, devicesResponse, sessionsResponse] = await Promise.all([
    api.get(`/admin/users/${id}`),
    api.get("/admin/groups/all"),
    api.get(`/admin/users/${id}/devices`),
    api.get(`/admin/users/${id}/sessions`, { params: { size: 10 } }),
  ]);
  fill(userResponse.data);
  groups.value = groupsResponse.data;
  devices.value = devicesResponse.data;
  sessions.value = sessionsResponse.data.results;
  await loadEvents();
}

/**
 * Loads the user's recent activity. It is found by address rather than by
 * account, so that changes administrators made to them, which name them,
 * come with what they did themselves.
 */
async function loadEvents() {
  const { data } = await api.get("/admin/events", { params: { search: user.value.email, size: 15 } });
  events.value = data.results;
}

async function save() {
  saving.value = true;
  errors.value = {};
  try {
    const { data } = await api.put(`/admin/users/${user.value.id}`, form.value);
    fill(data);
    toast.success("Changes saved.");
  } catch (error) {
    errors.value = fieldErrors(error);
    if (!Object.keys(errors.value).some((key) => key in form.value))
      toast.error(errorMessage(error));
  } finally {
    saving.value = false;
  }
}

async function resetPassword() {
  try {
    const { data } = await api.delete(`/admin/users/${user.value.id}/password`);
    resetEmailed.value = data.emailed;
    resetLink.value = data.link;
  } catch (error) {
    toast.error(errorMessage(error, "The password could not be reset."));
    throw error;
  }
}

async function unlock() {
  try {
    await api.delete(`/admin/users/${user.value.id}/lockout`);
    await load();
    toast.success(`${user.value.email} can sign in again.`);
  } catch (error) {
    toast.error(errorMessage(error, "The account could not be unlocked."));
  }
}

async function resetMfa() {
  try {
    await api.put(`/admin/users/${user.value.id}/mfa`);
    await load();
    toast.success(
      "Two-factor reset. They'll enrol again on their next sign-in.",
    );
  } catch (error) {
    toast.error(errorMessage(error, "Two-factor could not be reset."));
    throw error;
  }
}

async function remove() {
  try {
    await api.delete(`/admin/users/${user.value.id}`);
    toast.success(`${user.value.email} deleted.`);
    router.push("/admin/users");
  } catch (error) {
    toast.error(errorMessage(error, "The user could not be deleted."));
    throw error;
  }
}

async function revoke() {
  const device = revoking.value;
  try {
    await api.delete(`/admin/devices/${device.id}`);
    devices.value = devices.value.filter((item) => item.id !== device.id);
    toast.success(`${device.name} revoked.`);
  } catch (error) {
    toast.error(errorMessage(error, `${device.name} could not be revoked.`));
    throw error;
  }
}

function startAddressing(device) {
  addressing.value = device;
  address.value = device.static_ip;
  addressError.value = "";
}

/** Saves a fixed address: one typed, "auto" for the next free one, or "" for none. */
async function saveAddress(value) {
  addressBusy.value = true;
  addressError.value = "";
  try {
    const { data } = await api.put(`/admin/devices/${addressing.value.id}`, {
      static_ip: value,
    });
    devices.value = devices.value.map((item) =>
      item.id === data.id ? data : item,
    );
    toast.success(
      data.static_ip
        ? `${data.name} will connect at ${data.static_ip} from its next connection.`
        : `${data.name} will get an address from the pool from its next connection.`,
    );
    addressing.value = null;
  } catch (error) {
    addressError.value = fieldErrors(error).static_ip ?? errorMessage(error);
  } finally {
    addressBusy.value = false;
  }
}

async function copyPassword() {
  await navigator.clipboard.writeText(resetLink.value);
  copied.value = true;
  setTimeout(() => (copied.value = false), 1500);
}

watch(() => route.params.id, load, { immediate: true });
</script>

<template>
  <template v-if="user">
    <PageHeader
      :title="user.name || user.email"
      :back="{ to: '/admin/users', label: 'Users' }"
    >
      <template #leading>
        <UserAvatar
          :name="user.name"
          :email="user.email"
          size="lg"
          tone="accent"
        />
      </template>
      <template #description>
        <span class="flex flex-wrap items-center gap-2.5">
          <span v-if="user.name">{{ user.email }}</span>
          <StatusDot
            v-if="user.locked_until"
            state="deny"
            :label="`Locked until ${timeOfDay(user.locked_until)}`"
          />
          <StatusDot
            v-else
            :state="user.is_enabled ? 'up' : 'off'"
            :label="user.is_enabled ? 'Active' : 'Disabled'"
          />
          <AppBadge v-if="ROLE_BADGES[user.role]" tone="accent">{{
            ROLE_BADGES[user.role]
          }}</AppBadge>
        </span>
      </template>
      <template #actions>
        <template v-if="canHelp">
          <AppButton v-if="user.locked_until" variant="primary" @click="unlock"
            >Unlock</AppButton
          >
          <AppButton @click="confirming = 'password'">Reset password</AppButton>
          <AppButton @click="confirming = 'mfa'">Reset 2FA</AppButton>
        </template>
        <AppButton
          v-if="canEdit && !isSelf"
          variant="danger"
          @click="confirming = 'delete'"
          >Delete user</AppButton
        >
      </template>
    </PageHeader>

    <PageTabs :items="sections" label="User" />

    <div>
      <form v-if="section === 'details'" class="card" @submit.prevent="save">
        <!-- Read-only for the help desk. -->
        <fieldset :disabled="!canEdit" class="min-w-0 p-6">
          <SettingRow v-slot="field" label="Full name" :error="errors.name">
            <input
              :id="field.id"
              v-model="form.name"
              type="text"
              class="input"
              :aria-invalid="field.invalid"
              :aria-describedby="field.describedby"
            />
          </SettingRow>

          <SettingRow
            v-slot="field"
            label="Email"
            description="What they sign in with."
            :error="errors.email"
          >
            <input
              :id="field.id"
              v-model="form.email"
              type="email"
              class="input"
              :aria-invalid="field.invalid"
              :aria-describedby="field.describedby"
            />
          </SettingRow>

          <SettingRow
            v-slot="field"
            label="Group"
            description="Sets their device limit and what they can reach."
            :error="errors.group_id"
          >
            <select
              :id="field.id"
              v-model="form.group_id"
              class="input"
              :aria-invalid="field.invalid"
              :aria-describedby="field.describedby"
            >
              <option v-for="group in groups" :key="group.id" :value="group.id">
                {{ group.name }}
              </option>
            </select>
          </SettingRow>

          <SettingRow
            v-slot="field"
            label="Role"
            :description="
              isSelf && canEdit
                ? 'You can\'t change your own role.'
                : 'Help desk can unlock and reset members, but not change settings.'
            "
            :error="errors.role"
          >
            <select
              :id="field.id"
              v-model="form.role"
              class="input"
              :disabled="isSelf"
              :aria-describedby="field.describedby"
            >
              <option value="member">Member</option>
              <option value="helpdesk">Help desk</option>
              <option value="admin">Administrator</option>
            </select>
          </SettingRow>

          <SettingRow
            v-slot="field"
            label="Two-factor"
            :description="
              user.mfa_enabled
                ? 'Enrolled with an authenticator app.'
                : 'Not enrolled yet.'
            "
          >
            <select
              :id="field.id"
              v-model="mfaChoice"
              class="input"
              :aria-describedby="field.describedby"
            >
              <option value="inherit">Inherited from group</option>
              <option value="true">Required</option>
              <option value="false">Not required</option>
            </select>
          </SettingRow>

          <SettingRow
            v-if="!isSelf"
            label="Account enabled"
            description="Disabled users can't sign in or start new VPN sessions."
          >
            <ToggleSwitch
              v-model="form.is_enabled"
              label="Account enabled"
              class="sm:mt-2"
            />
          </SettingRow>
        </fieldset>

        <div
          v-if="canEdit"
          class="sticky bottom-0 flex items-center justify-between gap-3 rounded-b-xl border-t border-hair px-6 py-3.5 transition-colors"
          :class="dirty ? 'bg-accent-soft' : 'bg-surface'"
        >
          <span v-if="dirty" class="text-[13px] font-medium text-accent-strong"
            >You have unsaved changes.</span
          >
          <span v-else />
          <div class="flex items-center gap-2">
            <AppButton v-if="dirty" :disabled="saving" @click="discard"
              >Discard</AppButton
            >
            <AppButton
              variant="primary"
              type="submit"
              :disabled="saving || !dirty"
            >
              {{ saving ? "Saving…" : "Save changes" }}
            </AppButton>
          </div>
        </div>
      </form>

      <section
        v-else-if="section === 'devices'"
        aria-label="Devices"
        class="card overflow-hidden"
      >
        <div
          v-for="device in devices"
          :key="device.id"
          class="flex items-center gap-3.5 border-b border-hair px-5 py-4 last:border-0"
        >
          <div class="flex min-w-0 flex-1 flex-col gap-0.5">
            <span class="truncate font-medium">{{ device.name }}</span>
            <DeviceMeta :device="device" />
            <span v-if="device.static_ip" class="text-[13px] text-ink-3">
              Fixed address
              <span class="font-mono">{{ device.static_ip }}</span>
            </span>
          </div>
          <AppButton v-if="canEdit" size="sm" @click="startAddressing(device)"
            >Address</AppButton
          >
          <AppButton
            v-if="canHelp"
            size="sm"
            variant="danger"
            @click="revoking = device"
            >Revoke</AppButton
          >
        </div>

        <EmptyState
          v-if="!devices.length"
          :icon="LaptopMinimal"
          title="No devices"
          message="They haven't created any devices yet."
        />
      </section>

      <div v-else class="flex flex-col gap-6">
        <section aria-labelledby="sessions-title" class="card overflow-hidden">
          <div class="card-head">
            <h2 id="sessions-title" class="text-[15px] font-semibold">
              Recent connections
            </h2>
          </div>
          <div v-if="sessions.length" class="overflow-x-auto">
            <table class="w-full border-collapse">
              <thead>
                <tr>
                  <th class="th">Device</th>
                  <th class="th">Ended</th>
                  <th class="th hidden sm:table-cell">Lasted</th>
                  <th class="th hidden md:table-cell">From</th>
                  <th class="th hidden lg:table-cell">Data down / up</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="session in sessions" :key="session.id" class="row">
                  <td class="td font-medium">{{ session.device_name }}</td>
                  <td class="td text-ink-3">
                    <RelativeTime :value="session.ended_at" />
                  </td>
                  <td class="td hidden text-ink-3 tabular-nums sm:table-cell">
                    {{ duration(session.duration) }}
                  </td>
                  <td
                    class="td hidden font-mono text-[13px] text-ink-3 md:table-cell"
                  >
                    {{ session.remote_ip }}
                  </td>
                  <td
                    class="td hidden whitespace-nowrap text-ink-3 tabular-nums lg:table-cell"
                  >
                    {{ bytes(session.bytes_sent) }} /
                    {{ bytes(session.bytes_received) }}
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
          <p v-else class="px-5 py-6 text-ink-4">
            No finished connections yet.
          </p>
        </section>

        <section aria-labelledby="events-title" class="card overflow-hidden">
          <div class="card-head">
            <h2 id="events-title" class="text-[15px] font-semibold">
              Recent activity
            </h2>
            <RouterLink
              :to="{ name: 'admin-activity', query: { search: user.email } }"
              class="text-[13px] font-medium text-accent no-underline hover:text-accent-strong"
            >
              See all
            </RouterLink>
          </div>
          <div
            v-for="event in events"
            :key="event.id"
            class="flex items-center gap-3 border-b border-hair px-5 py-3 last:border-0"
          >
            <span
              class="flex size-7 shrink-0 items-center justify-center rounded-lg"
              :class="eventKind(event).tone"
            >
              <component
                :is="eventKind(event).icon"
                class="size-3.5"
                :stroke-width="2"
              />
            </span>
            <div
              class="flex min-w-0 flex-1 flex-col gap-0.5 sm:flex-row sm:items-baseline sm:gap-3"
            >
              <span class="font-medium whitespace-nowrap">{{
                eventKind(event).label
              }}</span>
              <span
                class="truncate text-[13px] text-ink-3"
                :title="event.detail"
                >{{ eventSummary(event) }}</span
              >
            </div>
            <RelativeTime
              :value="event.created_at"
              class="shrink-0 text-[13px] text-ink-4"
            />
          </div>
          <p v-if="!events.length" class="px-5 py-6 text-ink-4">Nothing yet.</p>
        </section>
      </div>
    </div>
  </template>

  <ConfirmDialog
    v-model:open="leaving"
    title="Discard unsaved changes?"
    :message="`Your changes to ${user?.email} haven't been saved.`"
    confirm-label="Discard changes"
    :action="() => settle(true)"
  />

  <ConfirmDialog
    :open="confirming === 'password'"
    title="Reset password?"
    message="Their current password stops working at once, and they're signed out. They get a link to choose a new one."
    confirm-label="Reset password"
    :action="resetPassword"
    @update:open="(value) => !value && (confirming = '')"
  />

  <ConfirmDialog
    :open="confirming === 'mfa'"
    title="Reset two-factor?"
    message="Their authenticator stops working, and they'll set up a new one on their next sign-in."
    confirm-label="Reset 2FA"
    :action="resetMfa"
    @update:open="(value) => !value && (confirming = '')"
  />

  <ConfirmDialog
    :open="confirming === 'delete'"
    :title="`Delete ${user?.email}?`"
    message="Their account and every device are removed, and any open VPN session is dropped. This can't be undone."
    confirm-label="Delete user"
    :action="remove"
    @update:open="(value) => !value && (confirming = '')"
  />

  <ConfirmDialog
    :open="revoking !== null"
    :title="`Revoke ${revoking?.name}?`"
    message="It can no longer connect, and any session it has open is dropped."
    confirm-label="Revoke device"
    :action="revoke"
    @update:open="(value) => !value && (revoking = null)"
  />

  <AppDialog
    :open="addressing !== null"
    :title="`Address for ${addressing?.name}`"
    description="Give the device the same VPN address every time it connects, for firewall rules or access lists that name it. It takes effect from its next connection."
    :busy="addressBusy"
    width="max-w-[480px]"
    @update:open="(value) => !value && (addressing = null)"
  >
    <form
      id="device-address"
      class="flex flex-col gap-3"
      @submit.prevent="saveAddress(address.trim())"
    >
      <FormField
        v-slot="field"
        label="Fixed address"
        hint="Leave blank to use an address from the pool."
        :error="addressError"
      >
        <input
          :id="field.id"
          v-model="address"
          type="text"
          placeholder="Next free address"
          class="input font-mono text-[13px]"
          :aria-invalid="field.invalid"
          :aria-describedby="field.describedby"
        />
      </FormField>
    </form>
    <template #footer>
      <AppButton
        :disabled="addressBusy"
        class="mr-auto"
        @click="saveAddress('auto')"
        >Use the next free address</AppButton
      >
      <AppButton :disabled="addressBusy" @click="addressing = null"
        >Cancel</AppButton
      >
      <AppButton
        variant="primary"
        type="submit"
        form="device-address"
        :disabled="addressBusy"
        >Save</AppButton
      >
    </template>
  </AppDialog>

  <AppDialog
    :open="resetLink !== ''"
    title="Password reset"
    :description="
      resetEmailed
        ? 'We\'ve emailed them a link to choose a new password. You can also pass it on yourself. It works once, for 3 days.'
        : 'This server can\'t send email, so pass this link on to them yourself. It works once, for 3 days.'
    "
    width="max-w-[480px]"
    @update:open="(value) => !value && (resetLink = '')"
  >
    <div
      class="flex items-center gap-2 rounded-[10px] border border-line bg-subtle py-2 pr-2 pl-4"
    >
      <code class="flex-1 truncate font-mono text-[13px]">{{ resetLink }}</code>
      <AppButton size="sm" @click="copyPassword">
        <Check v-if="copied" class="size-4 text-up" :stroke-width="2" />
        <Copy v-else class="size-4" :stroke-width="2" />
        {{ copied ? "Copied" : "Copy" }}
      </AppButton>
    </div>
    <template #footer>
      <AppButton variant="primary" @click="resetLink = ''">Done</AppButton>
    </template>
  </AppDialog>
</template>
