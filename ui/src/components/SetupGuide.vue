<script setup>
import { Plus } from "lucide-vue-next";
import { computed } from "vue";

import AppButton from "@/components/ui/AppButton.vue";
import { useSession } from "@/stores/session";

/** How to connect a device, step by step, starting with adding it. */
defineEmits(["add"]);

const session = useSession();

const STEP =
  "flex size-[26px] shrink-0 items-center justify-center rounded-full bg-accent-soft text-[13px] font-semibold text-accent";

// What to type into OpenVPN Connect: the address this page was reached on.
const serverAddress = window.location.host;

// The last step depends on what connecting asks for.
const signInStep = computed(() => {
  if (!session.profile?.vpn_password_required) {
    return "Use your email and the current code from your authenticator app.";
  }
  return session.profile?.mfa_required
    ? "Use your email and account password, then the current code from your authenticator app."
    : "Use your email and account password.";
});
</script>

<template>
  <div class="flex flex-col gap-5">
    <ol class="flex flex-col gap-5">
      <li class="flex items-start gap-3.5">
        <span :class="STEP">1</span>
        <div class="flex flex-col items-start gap-2.5 pt-0.5">
          <div class="flex flex-col gap-0.5">
            <span class="font-semibold">Add a device</span>
            <span class="leading-relaxed text-ink-3">Name it after the computer or phone, like “Work laptop”.</span>
          </div>
          <AppButton variant="primary" :disabled="!session.canAddDevice" @click="$emit('add')">
            <Plus class="size-4" :stroke-width="2.2" />
            New device
          </AppButton>
        </div>
      </li>
      <li class="flex items-start gap-3.5">
        <span :class="STEP">2</span>
        <div class="flex flex-col gap-0.5 pt-0.5">
          <span class="font-semibold">Import its profile</span>
          <span class="leading-relaxed text-ink-3">
            Choose <span class="font-medium text-ink-2">Open in OpenVPN Connect</span>, or download the
            <code class="rounded bg-muted px-1.5 py-px font-mono text-[13px]">.ovpn</code> file and import it
            yourself. Don't have the app?
            <a href="https://openvpn.net/client/" target="_blank" rel="noopener" class="font-medium text-accent hover:text-accent-strong">Get OpenVPN Connect</a>,
            free for macOS, Windows, Linux, iOS and Android.
          </span>
        </div>
      </li>
      <li class="flex items-start gap-3.5">
        <span :class="STEP">3</span>
        <div class="flex flex-col gap-0.5 pt-0.5">
          <span class="font-semibold">Connect and sign in</span>
          <span class="leading-relaxed text-ink-3">Turn the profile on in OpenVPN Connect. {{ signInStep }}</span>
        </div>
      </li>
    </ol>

    <div class="rounded-[10px] border border-hair bg-subtle px-4 py-3.5 text-[13px] leading-relaxed text-ink-3">
      <span class="font-semibold text-ink-2">Setting up from the app instead?</span>
      In OpenVPN Connect, choose <span class="font-medium text-ink-2">Import Profile</span> ›
      <span class="font-medium text-ink-2">URL</span>, enter
      <code class="rounded bg-muted px-1.5 py-px font-mono text-[12px] text-ink-2">{{ serverAddress }}</code>
      and sign in with your email and password. It adds a device for you.
    </div>
  </div>
</template>

