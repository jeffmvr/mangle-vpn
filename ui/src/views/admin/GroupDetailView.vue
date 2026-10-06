<script setup>
import { Plus, Shield, SlidersHorizontal, UserRound, Users } from "lucide-vue-next";
import { computed, onUnmounted, ref, watch } from "vue";
import { onBeforeRouteLeave, onBeforeRouteUpdate, useRoute, useRouter } from "vue-router";

import FirewallRules from "@/components/admin/FirewallRules.vue";
import GroupForm from "@/components/admin/GroupForm.vue";
import InviteDialog from "@/components/admin/InviteDialog.vue";
import AppButton from "@/components/ui/AppButton.vue";
import ConfirmDialog from "@/components/ui/ConfirmDialog.vue";
import EmptyState from "@/components/ui/EmptyState.vue";
import PageHeader from "@/components/ui/PageHeader.vue";
import PageTabs from "@/components/ui/PageTabs.vue";
import RelativeTime from "@/components/ui/RelativeTime.vue";
import StatusDot from "@/components/ui/StatusDot.vue";
import UserAvatar from "@/components/ui/UserAvatar.vue";
import { useRowLink } from "@/composables/useRowLink";
import api, { errorMessage, fieldErrors } from "@/lib/api";
import { useSession } from "@/stores/session";
import { useToast } from "@/stores/toast";

const route = useRoute();
const router = useRouter();
const session = useSession();
const openRow = useRowLink();

// The firewall tab's table, whose Add rule dialog the tab row opens.
const rules = ref(null);
const toast = useToast();

const group = ref(null);
const form = ref({});
const members = ref([]);
const ruleCount = ref(undefined);
const errors = ref({});
const saving = ref(false);
const inviting = ref(false);
const deleting = ref(false);

// The details as last loaded or saved, to tell whether anything has been
// edited since, as the settings page does.
const saved = ref("");
const dirty = computed(() => saved.value !== "" && JSON.stringify(form.value) !== saved.value);

function discard() {
  form.value = JSON.parse(saved.value);
  errors.value = {};
}

// Leaving the group with unsaved edits asks first. Moving between its own
// pages keeps them, so only leaving the group counts.
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

function warnUnload(event) {
  if (dirty.value) event.preventDefault();
}
window.addEventListener("beforeunload", warnUnload);
onUnmounted(() => window.removeEventListener("beforeunload", warnUnload));

// Which of the group's pages is showing comes from the address, so a reload
// or a shared link lands on it.
const section = computed(() => route.params.section);

const sections = computed(() =>
  [
    { id: "details", label: "Details", icon: SlidersHorizontal },
    { id: "firewall", label: "Firewall rules", icon: Shield, count: ruleCount.value },
    { id: "members", label: "Members", icon: Users, count: members.value.length },
  ].map((item) => ({ ...item, to: { name: "admin-group", params: { id: route.params.id, section: item.id } } })),
);

const summary = computed(() => {
  if (!group.value) return "";
  const devices = `up to ${group.value.max_devices} device${group.value.max_devices === 1 ? "" : "s"} each`;
  const people = `${members.value.length} member${members.value.length === 1 ? "" : "s"}`;
  return [group.value.description, people, devices].filter(Boolean).join(" · ");
});

function fill(data) {
  group.value = data;
  form.value = {
    name: data.name,
    description: data.description,
    max_devices: data.max_devices,
    mfa_enforced: data.mfa_enforced,
    is_enabled: data.is_enabled,
    routes: data.routes,
    nameservers: data.nameservers,
    device_idle_days: data.device_idle_days,
  };
  saved.value = JSON.stringify(form.value);
}

async function loadMembers() {
  const { data } = await api.get(`/admin/groups/${route.params.id}/users`);
  members.value = data;
}

async function load() {
  const { data } = await api.get(`/admin/groups/${route.params.id}`);
  fill(data);
  await loadMembers();
}

async function save() {
  saving.value = true;
  errors.value = {};
  try {
    const { data } = await api.put(`/admin/groups/${group.value.id}`, form.value);
    fill(data);
    toast.success("Changes saved.");
  } catch (error) {
    errors.value = fieldErrors(error);
    if (!Object.keys(errors.value).some((key) => key in form.value)) toast.error(errorMessage(error));
  } finally {
    saving.value = false;
  }
}

async function remove() {
  try {
    await api.delete(`/admin/groups/${group.value.id}`);
    toast.success(`${group.value.name} deleted.`);
    router.push("/admin/groups");
  } catch (error) {
    toast.error(errorMessage(error, "The group could not be deleted."));
    throw error;
  }
}

watch(() => route.params.id, load, { immediate: true });
</script>

<template>
  <template v-if="group">
    <PageHeader :title="group.name" :description="summary" :back="{ to: '/admin/groups', label: 'Groups' }">
      <template v-if="session.isAdmin" #actions>
        <AppButton variant="danger" @click="deleting = true">Delete group</AppButton>
      </template>
    </PageHeader>

    <div
      v-if="!group.is_enabled"
      class="flex items-center gap-2.5 rounded-[10px] border border-line bg-subtle px-4 py-3 text-ink-3"
    >
      <StatusDot state="off" />
      This group is disabled. Its members can't sign in or connect.
    </div>

    <PageTabs :items="sections" label="Group" />

    <div>
      <!-- The tab's own actions sit just above its content; the header
           keeps those for the whole group. -->
      <div
        v-if="session.isAdmin && (section === 'firewall' || section === 'members')"
        class="mb-3 flex justify-end"
      >
        <AppButton v-if="section === 'firewall'" size="sm" variant="primary" @click="rules?.startAdding()">
          <Plus class="size-3.5" :stroke-width="2.4" />
          Add rule
        </AppButton>
        <AppButton v-else size="sm" variant="primary" @click="inviting = true">
          <Plus class="size-3.5" :stroke-width="2.4" />
          Add users
        </AppButton>
      </div>

      <div class="min-w-0">
        <form v-if="section === 'details'" class="card" @submit.prevent="save">
          <!-- Read-only for the help desk. -->
          <fieldset :disabled="!session.isAdmin" class="min-w-0 p-6">
            <GroupForm v-model="form" :errors="errors" full />
          </fieldset>

          <!-- Stays in view while there is something to save. -->
          <div
            v-if="session.isAdmin"
            class="sticky bottom-0 flex items-center justify-between gap-3 rounded-b-xl border-t border-hair px-5 py-3.5 transition-colors"
            :class="dirty ? 'bg-accent-soft' : 'bg-surface'"
          >
            <span v-if="dirty" class="text-[13px] font-medium text-accent-strong">You have unsaved changes.</span>
            <span v-else />
            <div class="flex items-center gap-2">
              <AppButton v-if="dirty" :disabled="saving" @click="discard">Discard</AppButton>
              <AppButton variant="primary" type="submit" :disabled="saving || !dirty">
                {{ saving ? "Saving…" : "Save changes" }}
              </AppButton>
            </div>
          </div>
        </form>

        <FirewallRules v-show="section === 'firewall'" ref="rules" :group-id="group.id" @count="(count) => (ruleCount = count)" />

        <div v-if="section === 'members'" class="card overflow-hidden lg:overflow-visible">
          <div v-if="members.length" class="overflow-x-auto lg:overflow-visible">
            <table class="w-full min-w-[560px] border-collapse">
              <thead>
                <tr>
                  <th class="th">User</th>
                  <th class="th">Last sign-in</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="member in members" :key="member.id" class="row-link" @click="openRow($event, `/admin/users/${member.id}`)">
                  <td class="td py-3">
                    <div class="flex items-center gap-3">
                      <UserAvatar :name="member.name" :email="member.email" />
                      <div class="flex min-w-0 flex-col gap-0.5">
                        <RouterLink :to="`/admin/users/${member.id}`" class="link truncate">
                          {{ member.name || member.email }}
                        </RouterLink>
                        <span v-if="member.name" class="truncate text-[13px] text-ink-4">{{ member.email }}</span>
                      </div>
                    </div>
                  </td>
                  <td class="td text-ink-3"><RelativeTime :value="member.last_login" /></td>
                </tr>
              </tbody>
            </table>
          </div>
          <EmptyState v-else :icon="UserRound" title="No members" message="Add users to give them VPN access under this group's rules.">
            <AppButton v-if="session.isAdmin" variant="primary" @click="inviting = true">Add users</AppButton>
          </EmptyState>
        </div>
      </div>
    </div>
  </template>

  <InviteDialog v-model:open="inviting" :group-id="group?.id ?? ''" @invited="loadMembers" />

  <ConfirmDialog
    v-model:open="leaving"
    title="Discard unsaved changes?"
    :message="`Your changes to ${group?.name} haven't been saved.`"
    confirm-label="Discard changes"
    :action="() => settle(true)"
  />

  <ConfirmDialog
    v-model:open="deleting"
    :title="`Delete ${group?.name}?`"
    :message="`Its ${members.length} member${members.length === 1 ? '' : 's'} are deleted too, along with all of their devices. This can't be undone.`"
    confirm-label="Delete group and members"
    :action="remove"
  />
</template>
