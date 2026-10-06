<script setup>
import { Activity, LaptopMinimal, Menu, ScrollText, Search, SlidersHorizontal, User, Users, X } from "lucide-vue-next";
import { computed, onMounted, onUnmounted, ref, watch } from "vue";
import { useRoute } from "vue-router";

import AccountMenu from "@/components/AccountMenu.vue";
import BrandMark from "@/components/BrandMark.vue";
import RestartNotice from "@/components/RestartNotice.vue";
import CertificateNotice from "@/components/admin/CertificateNotice.vue";
import CommandPalette from "@/components/admin/CommandPalette.vue";
import ServerCard from "@/components/admin/ServerCard.vue";
import { LOG_SOURCES } from "@/lib/logs";
import { SETTINGS_SECTIONS } from "@/lib/settings";
import { useSession } from "@/stores/session";

const POLL_MS = 15000;

const session = useSession();
const route = useRoute();
let poll = null;

// On a narrow screen the navigation is a drawer, closed again by choosing
// a page.
const menuOpen = ref(false);
const searching = ref(false);

// The shortcut as this computer writes it.
const shortcut = /Mac|iPhone|iPad/.test(navigator.platform) ? "⌘K" : "Ctrl K";
watch(() => route.fullPath, () => (menuOpen.value = false));

const links = [
  { to: "/admin/clients", label: "Clients", icon: LaptopMinimal },
  { to: "/admin/users", label: "Users", icon: User },
  { to: "/admin/groups", label: "Groups", icon: Users },
  { to: "/admin/activity", label: "Activity", icon: Activity },
  {
    label: "Logs",
    icon: ScrollText,
    route: "admin-logs",
    adminOnly: true,
    children: LOG_SOURCES.map((item) => ({
      label: item.label,
      to: { name: "admin-logs", params: { source: item.slug } },
    })),
  },
  {
    label: "Settings",
    icon: SlidersHorizontal,
    route: "admin-settings",
    adminOnly: true,
    children: SETTINGS_SECTIONS.map((item) => ({
      label: item.label,
      to: { name: "admin-settings", params: { section: item.slug } },
    })),
  },
];

// The help desk doesn't see the server's logs or settings.
const visibleLinks = computed(() => links.filter((link) => !link.adminOnly || session.isAdmin));

onMounted(() => {
  session.refreshVpn();
  poll = setInterval(session.refreshVpn, POLL_MS);
});

onUnmounted(() => clearInterval(poll));
</script>

<template>
  <div class="min-h-screen md:grid md:grid-cols-[280px_minmax(0,1fr)]">
    <!-- The column carries the background and border down the whole page;
         the navigation inside it stays in view as the page scrolls. -->
    <aside
      class="fixed inset-y-0 left-0 z-50 w-[280px] max-w-[85vw] border-r border-line bg-surface shadow-[0_16px_48px_rgba(16,24,40,0.18)] transition-transform md:static md:z-auto md:w-auto md:max-w-none md:translate-x-0 md:shadow-none"
      :class="menuOpen ? 'translate-x-0' : '-translate-x-full'"
    >
      <nav aria-label="Administration" class="flex h-dvh flex-col gap-6 overflow-y-auto px-3.5 py-4 md:sticky md:top-0 md:h-screen">
        <div class="flex items-center justify-between px-1.5">
          <BrandMark />
          <button
            type="button"
            aria-label="Close menu"
            class="flex size-9 items-center justify-center rounded-lg text-ink-3 hover:bg-muted md:hidden"
            @click="menuOpen = false"
          >
            <X class="size-5" :stroke-width="2" />
          </button>
        </div>

        <div class="flex flex-col gap-0.5 md:mt-4">
          <div class="px-2.5 pb-1.5 text-[11px] font-semibold tracking-[0.06em] text-ink-4 uppercase" aria-hidden="true">
            Administration
          </div>
          <template v-for="link in visibleLinks" :key="link.label">
            <!-- Marked on the pages beneath it too, such as one group's. -->
            <RouterLink
              v-if="!link.children"
              :to="link.to"
              class="flex h-[38px] items-center gap-2.5 rounded-lg px-2.5 font-medium text-ink-2 no-underline transition-colors hover:bg-muted hover:text-ink"
              :class="{ '!bg-accent-soft !text-accent-strong': route.path.startsWith(link.to) }"
            >
              <component :is="link.icon" class="size-[18px] shrink-0" :stroke-width="2" />
              {{ link.label }}
            </RouterLink>

            <!-- An area with pages of its own leads to its first page, and
                 lists them only while you are in it. It is marked as the
                 current area, but only the page is highlighted. -->
            <template v-else>
              <RouterLink
                :to="link.children[0].to"
                class="flex h-[38px] items-center gap-2.5 rounded-lg px-2.5 font-medium no-underline transition-colors hover:bg-muted hover:text-ink"
                :class="route.name === link.route ? 'text-ink' : 'text-ink-2'"
              >
                <component :is="link.icon" class="size-[18px] shrink-0" :stroke-width="2" />
                {{ link.label }}
              </RouterLink>
              <div v-if="route.name === link.route" class="mb-1 ml-[19px] flex flex-col gap-px border-l border-line pl-2.5">
                <RouterLink
                  v-for="child in link.children"
                  :key="child.label"
                  :to="child.to"
                  class="flex h-[30px] items-center rounded-md px-2.5 text-[13px] text-ink-3 no-underline transition-colors hover:bg-muted hover:text-ink"
                  active-class="!bg-accent-soft font-medium !text-accent-strong"
                >
                  {{ child.label }}
                </RouterLink>
              </div>
            </template>
          </template>
        </div>

        <ServerCard class="mt-auto" />
      </nav>
    </aside>

    <div
      v-if="menuOpen"
      class="fixed inset-0 z-40 bg-ink/30 md:hidden"
      aria-hidden="true"
      @click="menuOpen = false"
    />

    <div class="flex min-w-0 flex-col">
      <header
        class="sticky top-0 z-30 flex h-[60px] items-center gap-2.5 border-b border-line bg-surface/90 px-4 backdrop-blur sm:px-7 md:static md:bg-surface md:backdrop-blur-none"
      >
        <button
          type="button"
          aria-label="Open menu"
          :aria-expanded="menuOpen"
          class="-ml-1.5 flex size-9 items-center justify-center rounded-lg text-ink-2 hover:bg-muted md:hidden"
          @click="menuOpen = true"
        >
          <Menu class="size-5" :stroke-width="2" />
        </button>
        <!-- On a phone the menu shows the brand, leaving room for the notices. -->
        <BrandMark class="max-sm:hidden md:hidden" />

        <!-- Notices about the server sit on the left, in line with the page
             beneath; the account menu keeps the right to itself. -->
        <div class="flex min-w-0 flex-1 items-center gap-2.5 md:-ml-1">
          <CertificateNotice />
          <RestartNotice />
        </div>
        <button
          type="button"
          aria-label="Go to a user, group or setting"
          class="flex h-9 items-center gap-2 rounded-lg border border-line bg-surface px-2.5 text-ink-4 transition-colors hover:border-field hover:text-ink-2"
          @click="searching = true"
        >
          <Search class="size-4" :stroke-width="2" />
          <span class="hidden pr-4 lg:inline">Jump to…</span>
          <kbd class="hidden rounded border border-line bg-subtle px-1.5 font-sans text-[11px] text-ink-4 sm:inline">{{ shortcut }}</kbd>
        </button>
        <AccountMenu />
      </header>

      <main class="flex w-full flex-col gap-6 px-4 pt-8 pb-14 sm:px-7">
        <RouterView />
      </main>
    </div>
  </div>

  <CommandPalette v-model:open="searching" />
</template>
