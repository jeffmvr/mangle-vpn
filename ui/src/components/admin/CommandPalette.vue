<script setup>
import {
  Activity,
  CornerDownLeft,
  LaptopMinimal,
  ScrollText,
  Search,
  SlidersHorizontal,
  User,
  Users,
} from "lucide-vue-next";
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from "vue";
import { useRouter } from "vue-router";

import api from "@/lib/api";
import { LOG_SOURCES } from "@/lib/logs";
import { SETTINGS_SECTIONS } from "@/lib/settings";
import { useSession } from "@/stores/session";

/**
 * Jump to any page, user or group by typing: ⌘K, or Ctrl+K, from anywhere
 * in administration.
 */
const open = defineModel("open", { type: Boolean, default: false });

const router = useRouter();
const session = useSession();

const query = ref("");
const active = ref(0);
const users = ref([]);
const groups = ref([]);
const input = ref(null);
const list = ref(null);
let returnFocus = null;
let timer = null;
let latest = 0;

const pages = computed(() => [
  { label: "Clients", hint: "Connected now", icon: LaptopMinimal, to: { name: "admin-clients" } },
  { label: "Users", icon: User, to: { name: "admin-users" } },
  { label: "Groups", icon: Users, to: { name: "admin-groups" } },
  { label: "Activity", icon: Activity, to: { name: "admin-activity" } },
  ...(session.isAdmin
    ? [
        ...SETTINGS_SECTIONS.map((item) => ({
          label: `${item.label} settings`,
          hint: item.description,
          icon: SlidersHorizontal,
          to: { name: "admin-settings", params: { section: item.slug } },
        })),
        ...LOG_SOURCES.map((item) => ({
          label: item.title,
          icon: ScrollText,
          to: { name: "admin-logs", params: { source: item.slug } },
        })),
      ]
    : []),
  { label: "My devices", icon: LaptopMinimal, to: { name: "profile" } },
]);

const matchingPages = computed(() => {
  const words = query.value.trim().toLowerCase().split(/\s+/).filter(Boolean);
  if (!words.length) return pages.value.slice(0, 4);
  return pages.value.filter((page) => {
    // By name only: descriptions are long enough to match nearly anything.
    const name = page.label.toLowerCase().split(/\s+/);
    return words.every((word) => name.some((part) => part.startsWith(word)));
  });
});

const sections = computed(() =>
  [
    { label: "Pages", items: matchingPages.value },
    {
      label: "Users",
      items: users.value.map((user) => ({
        label: user.name || user.email,
        hint: user.name ? user.email : "",
        icon: User,
        to: `/admin/users/${user.id}`,
      })),
    },
    {
      label: "Groups",
      items: groups.value.map((group) => ({
        label: group.name,
        hint: group.description,
        icon: Users,
        to: `/admin/groups/${group.id}`,
      })),
    },
  ].filter((section) => section.items.length),
);

const items = computed(() => sections.value.flatMap((section) => section.items));

/** Looks up users and groups as the query settles. */
async function lookUp(text) {
  const request = ++latest;
  if (text.length < 2) {
    users.value = [];
    groups.value = [];
    return;
  }
  const params = { search: text, size: 5 };
  const [found, foundGroups] = await Promise.all([
    api.get("/admin/users", { params }).catch(() => null),
    api.get("/admin/groups", { params }).catch(() => null),
  ]);
  if (request !== latest) return;
  users.value = found?.data.results ?? [];
  groups.value = foundGroups?.data.results ?? [];
}

watch(query, (text) => {
  active.value = 0;
  clearTimeout(timer);
  timer = setTimeout(() => lookUp(text.trim()), 150);
});

watch(items, () => {
  if (active.value >= items.value.length) active.value = 0;
});

watch(open, async (value) => {
  if (value) {
    returnFocus = document.activeElement;
    query.value = "";
    users.value = [];
    groups.value = [];
    active.value = 0;
    await nextTick();
    input.value?.focus();
  } else {
    returnFocus?.focus?.();
  }
});

function go(item) {
  if (!item) return;
  open.value = false;
  router.push(item.to);
}

async function move(step) {
  const count = items.value.length;
  if (!count) return;
  active.value = (active.value + step + count) % count;
  await nextTick();
  list.value?.querySelector(`[data-index="${active.value}"]`)?.scrollIntoView({ block: "nearest" });
}

function onKeydown(event) {
  if (event.key === "ArrowDown") {
    event.preventDefault();
    move(1);
  } else if (event.key === "ArrowUp") {
    event.preventDefault();
    move(-1);
  } else if (event.key === "Enter") {
    event.preventDefault();
    go(items.value[active.value]);
  } else if (event.key === "Escape") {
    open.value = false;
  }
}

function onGlobalKeydown(event) {
  if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === "k") {
    event.preventDefault();
    open.value = !open.value;
  }
}

/** The position of an item across every section, for the keyboard. */
function indexOf(item) {
  return items.value.indexOf(item);
}

onMounted(() => window.addEventListener("keydown", onGlobalKeydown));
onUnmounted(() => {
  window.removeEventListener("keydown", onGlobalKeydown);
  clearTimeout(timer);
});
</script>

<template>
  <Teleport to="body">
    <Transition
      enter-active-class="transition duration-100 ease-out"
      enter-from-class="opacity-0"
      leave-active-class="transition duration-75 ease-in"
      leave-to-class="opacity-0"
    >
      <div
        v-if="open"
        class="fixed inset-0 z-50 flex justify-center bg-ink/40 px-4 pt-[12vh]"
        @mousedown.self="open = false"
      >
        <div
          role="dialog"
          aria-modal="true"
          aria-label="Go to"
          class="flex h-fit max-h-[min(520px,70vh)] w-full max-w-[580px] flex-col overflow-hidden rounded-2xl bg-surface shadow-[0_24px_48px_rgba(16,24,40,0.24)]"
          @keydown="onKeydown"
        >
          <label class="flex items-center gap-3 border-b border-hair px-4">
            <Search class="size-[18px] shrink-0 text-ink-4" :stroke-width="2" />
            <span class="sr-only">Go to</span>
            <input
              ref="input"
              v-model="query"
              type="text"
              role="combobox"
              aria-expanded="true"
              aria-controls="palette-results"
              :aria-activedescendant="items.length ? `palette-${active}` : undefined"
              placeholder="Jump to a user, group or setting"
              autocomplete="off"
              spellcheck="false"
              class="h-14 w-full bg-transparent text-[15px] outline-none placeholder:text-ink-5"
            />
            <kbd class="rounded border border-line px-1.5 font-sans text-[11px] text-ink-4">Esc</kbd>
          </label>

          <div id="palette-results" ref="list" role="listbox" class="overflow-y-auto p-2">
            <p v-if="!items.length" class="px-3 py-8 text-center text-ink-4">Nothing matches “{{ query }}”.</p>
            <div v-for="section in sections" :key="section.label" role="group" :aria-label="section.label" class="pb-1">
              <div class="px-3 pt-2 pb-1 text-xs font-medium text-ink-4">{{ section.label }}</div>
              <div
                v-for="item in section.items"
                :id="`palette-${indexOf(item)}`"
                :key="item.label + (item.hint ?? '')"
                :data-index="indexOf(item)"
                role="option"
                :aria-selected="indexOf(item) === active"
                class="flex cursor-pointer items-center gap-3 rounded-lg px-3 py-2"
                :class="indexOf(item) === active ? 'bg-accent-soft' : ''"
                @mousemove="active = indexOf(item)"
                @click="go(item)"
              >
                <component
                  :is="item.icon"
                  class="size-4 shrink-0"
                  :class="indexOf(item) === active ? 'text-accent' : 'text-ink-4'"
                  :stroke-width="2"
                />
                <span class="shrink-0 font-medium" :class="indexOf(item) === active ? 'text-accent-strong' : 'text-ink'">
                  {{ item.label }}
                </span>
                <span v-if="item.hint" class="min-w-0 truncate text-[13px] text-ink-4">{{ item.hint }}</span>
                <CornerDownLeft
                  v-if="indexOf(item) === active"
                  class="ml-auto size-3.5 shrink-0 text-accent"
                  :stroke-width="2"
                />
              </div>
            </div>
          </div>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>
