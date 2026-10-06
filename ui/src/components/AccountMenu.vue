<script setup>
import { ChevronDown, LaptopMinimal, LockKeyhole, LogOut, SlidersHorizontal } from "lucide-vue-next";
import { ref } from "vue";
import { useRoute } from "vue-router";

import UserAvatar from "@/components/ui/UserAvatar.vue";
import { cookie } from "@/lib/api";
import { useSession } from "@/stores/session";

/** The signed in user's menu, top right on every screen. */
const session = useSession();
const route = useRoute();
const open = ref(false);

const ROLES = { admin: "Administrator", helpdesk: "Help desk", member: "Member" };

const item =
  "flex items-center gap-2.5 px-3.5 py-2.5 text-ink-2 no-underline hover:bg-subtle hover:text-ink";
</script>

<template>
  <div class="relative" @keydown.esc="open = false">
    <button
      type="button"
      aria-haspopup="menu"
      :aria-expanded="open"
      class="flex h-9 items-center gap-2 rounded-full border border-line bg-surface pr-2.5 pl-1 text-ink-2 hover:bg-subtle"
      @click="open = !open"
    >
      <UserAvatar :email="session.profile?.email" size="sm" tone="accent" />
      <span class="hidden max-w-[220px] truncate sm:inline">{{ session.profile?.email }}</span>
      <ChevronDown class="size-3.5" :stroke-width="2" />
    </button>

    <!-- Click anywhere else to dismiss. -->
    <div v-if="open" class="fixed inset-0 z-30" @click="open = false" />

    <div
      v-if="open"
      role="menu"
      class="absolute right-0 z-40 mt-2 w-60 overflow-hidden rounded-xl border border-line bg-surface shadow-[0_12px_32px_rgba(16,24,40,0.12)]"
    >
      <div class="border-b border-hair px-3.5 py-3">
        <div class="truncate font-medium">{{ session.profile?.email }}</div>
        <div class="text-[13px] text-ink-4">{{ ROLES[session.profile?.role] ?? "Member" }}</div>
      </div>

      <RouterLink
        v-if="session.canAdminister && !route.path.startsWith('/admin')"
        to="/admin"
        role="menuitem"
        :class="item"
        @click="open = false"
      >
        <SlidersHorizontal class="size-4 text-ink-4" :stroke-width="1.8" />
        Administration
      </RouterLink>
      <RouterLink v-else to="/" role="menuitem" :class="item" @click="open = false">
        <LaptopMinimal class="size-4 text-ink-4" :stroke-width="1.8" />
        My devices
      </RouterLink>
      <a href="/password" role="menuitem" :class="item">
        <LockKeyhole class="size-4 text-ink-4" :stroke-width="1.8" />
        Change password
      </a>
      <!-- Signing out is a POST, so another site cannot sign you out. -->
      <form action="/logout" method="post">
        <input type="hidden" name="csrfmiddlewaretoken" :value="cookie('csrftoken')" />
        <button type="submit" role="menuitem" :class="[item, 'w-full text-left']">
          <LogOut class="size-4 text-ink-4" :stroke-width="1.8" />
          Sign out
        </button>
      </form>

      <div class="border-t border-hair bg-subtle px-3.5 py-2 font-mono text-[11px] text-ink-4">
        Mangle VPN v{{ session.version }}
      </div>
    </div>
  </div>
</template>
