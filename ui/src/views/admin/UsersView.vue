<script setup>
import { Plus, UserRound } from "lucide-vue-next";
import { onMounted, ref, watch } from "vue";

import InviteDialog from "@/components/admin/InviteDialog.vue";
import AppBadge from "@/components/ui/AppBadge.vue";
import AppButton from "@/components/ui/AppButton.vue";
import EmptyState from "@/components/ui/EmptyState.vue";
import LoadingRows from "@/components/ui/LoadingRows.vue";
import PageHeader from "@/components/ui/PageHeader.vue";
import RelativeTime from "@/components/ui/RelativeTime.vue";
import SearchInput from "@/components/ui/SearchInput.vue";
import StatusDot from "@/components/ui/StatusDot.vue";
import TablePager from "@/components/ui/TablePager.vue";
import UserAvatar from "@/components/ui/UserAvatar.vue";
import { useRowLink } from "@/composables/useRowLink";
import api from "@/lib/api";
import { timeOfDay } from "@/lib/format";
import { useSession } from "@/stores/session";

const PAGE_SIZE = 15;

const session = useSession();
const openRow = useRowLink();
const ROLE_BADGES = { admin: "Admin", helpdesk: "Help desk" };

const users = ref([]);
const total = ref(0);
const page = ref(1);
const search = ref("");
const loaded = ref(false);
const inviting = ref(false);

async function load() {
  const { data } = await api.get("/admin/users", {
    params: { search: search.value, page: page.value, size: PAGE_SIZE },
  });
  users.value = data.results;
  total.value = data.count;
  loaded.value = true;
}

/** What a user's two-factor setting comes to, in words. */
function twoFactor(user) {
  if (user.mfa_enabled) return "Enrolled";
  if (user.mfa_enforced === false) return "Not required";
  return "Not enrolled";
}

// A new search goes back to the first page, or the results can look empty.
// Changing the page loads it, so load directly only when the page is
// already the first.
watch(search, () => {
  if (page.value === 1) load();
  else page.value = 1;
});

watch(page, load);

onMounted(load);
</script>

<template>
  <PageHeader title="Users" description="Everyone who can sign in and create VPN devices.">
    <template v-if="session.isAdmin" #actions>
      <AppButton variant="primary" @click="inviting = true">
        <Plus class="size-4" :stroke-width="2.2" />
        Invite users
      </AppButton>
    </template>
  </PageHeader>

  <SearchInput v-model="search" label="Search users" placeholder="Search by name or email" />

  <div class="card overflow-hidden lg:overflow-visible">
    <LoadingRows v-if="!loaded" />
    <div v-else-if="users.length" class="overflow-x-auto lg:overflow-visible">
      <table class="w-full border-collapse">
        <thead>
          <tr>
            <th class="th">User</th>
            <th class="th hidden sm:table-cell">Group</th>
            <th class="th hidden lg:table-cell">Two-factor</th>
            <th class="th hidden md:table-cell">Status</th>
            <th class="th hidden lg:table-cell">Last sign-in</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="user in users" :key="user.id" class="row-link" @click="openRow($event, `/admin/users/${user.id}`)">
            <td class="td py-3">
              <div class="flex items-center gap-3">
                <UserAvatar :name="user.name" :email="user.email" />
                <div class="flex min-w-0 flex-col gap-0.5">
                  <span class="flex items-center gap-2">
                    <RouterLink :to="`/admin/users/${user.id}`" class="link truncate">
                      {{ user.name || user.email }}
                    </RouterLink>
                    <AppBadge v-if="ROLE_BADGES[user.role]" tone="accent">{{ ROLE_BADGES[user.role] }}</AppBadge>
                  </span>
                  <span v-if="user.name" class="truncate text-[13px] text-ink-4">{{ user.email }}</span>
                  <!-- On a narrow screen the hidden columns' key facts sit here. -->
                  <span class="text-[13px] text-ink-4 sm:hidden">{{ user.group?.name }}</span>
                  <StatusDot v-if="user.locked_until" state="deny" label="Locked" class="md:hidden" />
                  <StatusDot v-else-if="!user.is_enabled" state="off" label="Disabled" class="md:hidden" />
                </div>
              </div>
            </td>
            <td class="td hidden text-ink-2 sm:table-cell">{{ user.group?.name }}</td>
            <td class="td hidden lg:table-cell">
              <StatusDot v-if="twoFactor(user) === 'Not enrolled'" state="pending" label="Not enrolled" muted />
              <span v-else class="text-ink-2">{{ twoFactor(user) }}</span>
            </td>
            <td class="td hidden md:table-cell">
              <StatusDot
                v-if="user.locked_until"
                state="deny"
                :label="`Locked until ${timeOfDay(user.locked_until)}`"
              />
              <StatusDot
                v-else
                :state="user.is_enabled ? 'up' : 'off'"
                :label="user.is_enabled ? 'Active' : 'Disabled'"
                muted
              />
            </td>
            <td class="td hidden text-ink-3 lg:table-cell"><RelativeTime :value="user.last_login" /></td>
          </tr>
        </tbody>
      </table>
    </div>

    <EmptyState
      v-else-if="loaded"
      :icon="UserRound"
      :title="search ? 'No matching users' : 'No users yet'"
      :message="search ? 'Try a different search.' : 'Invite people to let them create VPN devices.'"
    />

    <TablePager v-if="total" v-model:page="page" :total="total" :page-size="PAGE_SIZE" noun="users" />
  </div>

  <InviteDialog v-model:open="inviting" @invited="load" />
</template>
