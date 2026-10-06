<script setup>
import { Pause, Play, ScrollText } from "lucide-vue-next";
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from "vue";
import { useRoute } from "vue-router";

import AppButton from "@/components/ui/AppButton.vue";
import EmptyState from "@/components/ui/EmptyState.vue";
import PageHeader from "@/components/ui/PageHeader.vue";
import RelativeTime from "@/components/ui/RelativeTime.vue";
import SearchInput from "@/components/ui/SearchInput.vue";
import ToggleSwitch from "@/components/ui/ToggleSwitch.vue";
import api, { errorMessage } from "@/lib/api";
import { dateTime } from "@/lib/format";
import { LOG_SOURCES, parseLogLine } from "@/lib/logs";

const REFRESH_MS = 5000;
const LINES = 500;

const LEVELS = {
  ERROR: "bg-[#7a271a] text-[#fecdca]",
  WARN: "bg-[#7a2e0e] text-[#fedf89]",
  INFO: "bg-white/10 text-ink-5",
  DEBUG: "bg-white/5 text-ink-4",
};

const route = useRoute();

// The log shown comes from the address, chosen in the sidebar.
const current = computed(() => LOG_SOURCES.find((item) => item.slug === route.params.source) ?? LOG_SOURCES[0]);
const source = computed(() => current.value.id);

const log = ref(null);
const error = ref("");
const live = ref(true);
const search = ref("");
const errorsOnly = ref(false);
const panel = ref(null);
let poll = null;

const entries = computed(() => (log.value?.lines ?? []).filter(Boolean).map(parseLogLine));

const filtered = computed(() => {
  const needle = search.value.trim().toLowerCase();
  return entries.value.filter(
    (entry) =>
      (!errorsOnly.value || entry.level === "ERROR") &&
      (!needle || entry.raw.toLowerCase().includes(needle)),
  );
});

const filtering = computed(() => Boolean(search.value.trim()) || errorsOnly.value);

/** The time of day of a line, to the second. */
function clock(time) {
  if (!time || Number.isNaN(time.getTime())) return "";
  return time.toLocaleTimeString(undefined, { hour: "2-digit", minute: "2-digit", second: "2-digit", hour12: false });
}

/** Whether the reader is at the bottom, so new lines should keep them there. */
function atBottom() {
  const el = panel.value;
  return !el || el.scrollHeight - el.scrollTop - el.clientHeight < 40;
}

async function scrollToEnd() {
  await nextTick();
  panel.value?.scrollTo({ top: panel.value.scrollHeight });
}

async function load({ scroll = false } = {}) {
  const follow = scroll || atBottom();
  try {
    const { data } = await api.get(`/admin/logs/${source.value}`, { params: { lines: LINES } });
    log.value = data;
    error.value = "";
  } catch (failure) {
    error.value = errorMessage(failure, "This log could not be read.");
  }
  if (follow) scrollToEnd();
}

function startPolling() {
  clearInterval(poll);
  if (live.value) poll = setInterval(load, REFRESH_MS);
}

watch(source, () => {
  log.value = null;
  load({ scroll: true });
});

watch([search, errorsOnly], scrollToEnd);

watch(live, (value) => {
  startPolling();
  if (value) load();
});

onMounted(() => {
  load({ scroll: true });
  startPolling();
});

onUnmounted(() => clearInterval(poll));
</script>

<template>
  <PageHeader :title="current.title" :description="current.description">
    <template #actions>
      <AppButton @click="live = !live">
        <Pause v-if="live" class="size-4" :stroke-width="2" />
        <Play v-else class="size-4" :stroke-width="2" />
        {{ live ? "Pause" : "Resume" }}
      </AppButton>
    </template>
  </PageHeader>

  <div class="flex flex-wrap items-center gap-x-5 gap-y-3">
    <SearchInput v-model="search" label="Search the log" placeholder="Search the log" />
    <!-- The switch is a label of its own, so the word beside it toggles it
         by hand. -->
    <div class="flex items-center gap-2.5 text-ink-2">
      <ToggleSwitch v-model="errorsOnly" label="Errors only" />
      <span class="cursor-pointer select-none" aria-hidden="true" @click="errorsOnly = !errorsOnly">Errors only</span>
    </div>
    <span class="flex items-center gap-2 text-[13px] text-ink-4 sm:ml-auto">
      <span
        class="inline-block size-[7px] rounded-full"
        :class="live ? 'animate-pulse bg-up-dot' : 'bg-ink-5'"
      />
      <span>
        {{ live ? "Live" : "Paused" }}<template v-if="log?.updated_at">, last written <RelativeTime :value="log.updated_at" lower /></template>
      </span>
    </span>
  </div>

  <div class="card overflow-hidden">
    <p v-if="error" class="border-b border-hair bg-deny-soft px-5 py-3 text-[13px] text-deny">{{ error }}</p>

    <EmptyState
      v-if="log && !log.lines.length"
      :icon="ScrollText"
      title="Nothing logged yet"
      :message="
        source === 'openvpn'
          ? 'OpenVPN writes here once the server has started.'
          : 'Entries appear here as the server writes them.'
      "
    />

    <div
      v-else
      ref="panel"
      tabindex="0"
      aria-label="Log output"
      class="h-[min(640px,70vh)] overflow-auto bg-ink py-3 font-mono text-[12.5px] leading-[1.7] text-line"
    >
      <p v-if="log?.truncated && !filtering" class="px-5 pb-2 text-ink-5">Only the last {{ LINES }} lines are shown.</p>
      <p v-if="log && filtering && !filtered.length" class="px-5 py-2 text-ink-5">
        No lines in the last {{ LINES }} match.
      </p>
      <div
        v-for="(entry, index) in filtered"
        :key="index"
        class="flex gap-3 px-5 hover:bg-white/[0.04]"
        :class="{ 'bg-[#f04438]/[0.08]': entry.level === 'ERROR' }"
      >
        <span class="w-[60px] shrink-0 text-ink-4" :title="entry.time ? dateTime(entry.time) : ''">{{ clock(entry.time) }}</span>
        <span class="w-[46px] shrink-0">
          <span
            v-if="entry.level"
            class="inline-block rounded px-1 text-[10.5px] leading-[18px] font-semibold"
            :class="LEVELS[entry.level] ?? LEVELS.INFO"
          >{{ entry.level }}</span>
        </span>
        <!-- On one line: the text is pre-wrapped, so any space around it shows. -->
        <span class="min-w-0 flex-1 break-all whitespace-pre-wrap"><span class="text-white">{{ entry.message }}</span><template v-for="[key, value] in entry.fields" :key="key">{{ " " }}<span class="text-ink-4">{{ key }}=</span><span class="text-ink-5">{{ value }}</span></template></span>
      </div>
    </div>
  </div>
</template>
