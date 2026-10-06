<script setup>
import { computed, onMounted, ref, watch } from "vue";
import { useRoute } from "vue-router";

import EmptyState from "@/components/ui/EmptyState.vue";
import LoadingRows from "@/components/ui/LoadingRows.vue";
import PageHeader from "@/components/ui/PageHeader.vue";
import SearchInput from "@/components/ui/SearchInput.vue";
import TablePager from "@/components/ui/TablePager.vue";
import UserAvatar from "@/components/ui/UserAvatar.vue";
import api from "@/lib/api";
import { FILTERS, eventKind as kind, eventSummary as summary } from "@/lib/activity";
import { dayLabel, timeOfDay } from "@/lib/format";

const PAGE_SIZE = 25;

/** The page's events in runs by day, newest first as the server sends them. */
const days = computed(() => {
  const runs = [];
  for (const event of events.value) {
    const label = dayLabel(event.created_at);
    if (runs.at(-1)?.label !== label) runs.push({ label, events: [] });
    runs.at(-1).events.push(event);
  }
  return runs;
});

const events = ref([]);
const total = ref(0);
const page = ref(1);
const route = useRoute();

// A link from elsewhere, such as a user's page, can start with a search.
const search = ref(typeof route.query.search === "string" ? route.query.search : "");
const kindFilter = ref("");
const loaded = ref(false);

async function load() {
  const { data } = await api.get("/admin/events", {
    params: { search: search.value, kind: kindFilter.value || undefined, page: page.value, size: PAGE_SIZE },
  });
  events.value = data.results;
  total.value = data.count;
  loaded.value = true;
}

watch([search, kindFilter], () => {
  page.value = 1;
  load();
});

watch(page, load);

onMounted(load);
</script>

<template>
  <PageHeader title="Activity" description="Who signed in, connected and changed what.">
    <template #actions>
      <label class="max-sm:w-full">
        <span class="sr-only">Kind of activity</span>
        <select v-model="kindFilter" class="input sm:w-48">
          <option v-for="filter in FILTERS" :key="filter.value" :value="filter.value">{{ filter.label }}</option>
        </select>
      </label>
      <SearchInput v-model="search" label="Search activity" placeholder="Search activity" />
    </template>
  </PageHeader>

  <div class="card overflow-hidden lg:overflow-visible">
    <LoadingRows v-if="!loaded" />
    <div v-else-if="events.length" class="overflow-x-auto lg:overflow-visible">
      <table class="w-full min-w-[760px] border-collapse">
        <thead>
          <tr>
            <th class="th w-28">Time</th>
            <th class="th">What</th>
            <th class="th">User</th>
            <th class="th">Detail</th>
          </tr>
        </thead>
        <tbody v-for="(day, index) in days" :key="day.label">
          <tr>
            <th
              colspan="4"
              scope="colgroup"
              class="border-b border-hair bg-subtle px-5 py-2 text-left text-xs font-medium text-ink-3"
              :class="{ 'border-t': index > 0 }"
            >
              {{ day.label }}
            </th>
          </tr>
          <tr v-for="event in day.events" :key="event.id" class="row">
            <td class="td whitespace-nowrap text-ink-3 tabular-nums" :title="new Date(event.created_at).toLocaleString()">
              {{ timeOfDay(event.created_at) }}
            </td>
            <td class="td whitespace-nowrap">
              <span class="flex items-center gap-2.5 font-medium">
                <span class="flex size-7 shrink-0 items-center justify-center rounded-lg" :class="kind(event).tone">
                  <component :is="kind(event).icon" class="size-3.5" :stroke-width="2" />
                </span>
                {{ kind(event).label }}
              </span>
            </td>
            <td class="td">
              <div v-if="event.user?.email" class="flex items-center gap-2.5">
                <UserAvatar :email="event.user.email" size="sm" />
                <RouterLink :to="`/admin/users/${event.user.id}`" class="link font-normal text-ink-2">
                  {{ event.user.email }}
                </RouterLink>
              </div>
              <span v-else class="text-ink-4">System</span>
            </td>
            <td class="td text-ink-3" :title="event.detail">{{ summary(event) }}</td>
          </tr>
        </tbody>
      </table>
    </div>

    <EmptyState
      v-else-if="loaded"
      :icon="ListTree"
      :title="search || kindFilter ? 'Nothing matches' : 'No activity yet'"
      :message="search || kindFilter ? 'Try a different search or filter.' : 'Sign-ins, connections and changes appear here as they happen.'"
    />

    <TablePager v-if="total" v-model:page="page" :total="total" :page-size="PAGE_SIZE" noun="entries" />
  </div>
</template>
