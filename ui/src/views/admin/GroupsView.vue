<script setup>
import { Plus, Users } from "lucide-vue-next";
import { onMounted, ref, watch } from "vue";
import { useRouter } from "vue-router";

import GroupForm from "@/components/admin/GroupForm.vue";
import AppBadge from "@/components/ui/AppBadge.vue";
import AppButton from "@/components/ui/AppButton.vue";
import AppDialog from "@/components/ui/AppDialog.vue";
import EmptyState from "@/components/ui/EmptyState.vue";
import LoadingRows from "@/components/ui/LoadingRows.vue";
import PageHeader from "@/components/ui/PageHeader.vue";
import SearchInput from "@/components/ui/SearchInput.vue";
import TablePager from "@/components/ui/TablePager.vue";
import { useRowLink } from "@/composables/useRowLink";
import api, { errorMessage, fieldErrors } from "@/lib/api";
import { useSession } from "@/stores/session";
import { useToast } from "@/stores/toast";

const PAGE_SIZE = 15;

const router = useRouter();
const session = useSession();
const toast = useToast();
const openRow = useRowLink();
const groups = ref([]);
const total = ref(0);
const page = ref(1);
const search = ref("");
const loaded = ref(false);

const creating = ref(false);
const draft = ref({});
const errors = ref({});
const busy = ref(false);

async function load() {
  const { data } = await api.get("/admin/groups", {
    params: { search: search.value, page: page.value, size: PAGE_SIZE },
  });
  groups.value = data.results;
  total.value = data.count;
  loaded.value = true;
}

function startCreating() {
  draft.value = { name: "", description: "", max_devices: 1, mfa_enforced: true };
  errors.value = {};
  creating.value = true;
}

async function create() {
  busy.value = true;
  errors.value = {};
  try {
    const { data } = await api.post("/admin/groups", draft.value);
    creating.value = false;
    toast.success(`${data.name} created.`);
    router.push(`/admin/groups/${data.id}`);
  } catch (error) {
    errors.value = fieldErrors(error);
    if (!Object.keys(errors.value).some((key) => key in draft.value)) toast.error(errorMessage(error));
  } finally {
    busy.value = false;
  }
}

watch(search, () => {
  page.value = 1;
  load();
});

watch(page, load);

onMounted(load);
</script>

<template>
  <PageHeader title="Groups" description="Groups set device limits, two-factor, and firewall rules for their members.">
    <template v-if="session.isAdmin" #actions>
      <AppButton variant="primary" @click="startCreating">
        <Plus class="size-4" :stroke-width="2.2" />
        New group
      </AppButton>
    </template>
  </PageHeader>

  <SearchInput v-model="search" label="Search groups" placeholder="Search groups" />

  <div class="card overflow-hidden lg:overflow-visible">
    <LoadingRows v-if="!loaded" />
    <div v-else-if="groups.length" class="overflow-x-auto lg:overflow-visible">
      <table class="w-full min-w-[680px] border-collapse">
        <thead>
          <tr>
            <th class="th">Group</th>
            <th class="th">Members</th>
            <th class="th">Firewall rules</th>
            <th class="th">Devices per user</th>
            <th class="th">Two-factor</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="group in groups" :key="group.id" class="row-link" @click="openRow($event, `/admin/groups/${group.id}`)">
            <td class="td py-3">
              <div class="flex min-w-0 flex-col gap-0.5">
                <span class="flex items-center gap-2">
                  <RouterLink :to="`/admin/groups/${group.id}`" class="link" :class="{ '!text-ink-3': !group.is_enabled }">
                    {{ group.name }}
                  </RouterLink>
                  <AppBadge v-if="!group.is_enabled">Disabled</AppBadge>
                </span>
                <span v-if="group.description" class="truncate text-[13px] text-ink-4">{{ group.description }}</span>
              </div>
            </td>
            <td class="td text-ink-2 tabular-nums">{{ group.member_count }}</td>
            <td class="td tabular-nums" :class="group.rule_count ? 'text-ink-2' : 'text-ink-4'">
              {{ group.rule_count || "None" }}
            </td>
            <td class="td text-ink-2 tabular-nums">{{ group.max_devices }}</td>
            <td class="td text-ink-2">{{ group.mfa_enforced ? "Required" : "Optional" }}</td>
          </tr>
        </tbody>
      </table>
    </div>

    <EmptyState
      v-else-if="loaded"
      :icon="Users"
      :title="search ? 'No matching groups' : 'No groups yet'"
      :message="search ? 'Try a different search.' : 'Every user belongs to a group. Create one to start inviting people.'"
    />

    <TablePager v-if="total" v-model:page="page" :total="total" :page-size="PAGE_SIZE" noun="groups" />
  </div>

  <AppDialog v-model:open="creating" title="New group" :busy="busy">
    <form id="new-group" @submit.prevent="create">
      <GroupForm v-model="draft" :errors="errors" />
    </form>
    <template #footer>
      <AppButton :disabled="busy" @click="creating = false">Cancel</AppButton>
      <AppButton variant="primary" type="submit" form="new-group" :disabled="busy">
        {{ busy ? "Creating…" : "Create group" }}
      </AppButton>
    </template>
  </AppDialog>
</template>
