<script setup>
import { ref } from "vue";

import AppButton from "./AppButton.vue";
import AppDialog from "./AppDialog.vue";

/**
 * Asks before doing something that cannot be undone.
 *
 * `action` is the async work to run on confirm; the dialog stays open and
 * busy until it settles, and closes only if it succeeds.
 */
const props = defineProps({
  title: { type: String, required: true },
  message: { type: String, default: "" },
  confirmLabel: { type: String, default: "Confirm" },
  danger: { type: Boolean, default: true },
  action: { type: Function, required: true },
});

const open = defineModel("open", { type: Boolean, default: false });
const busy = ref(false);

async function confirm() {
  busy.value = true;
  try {
    await props.action();
    open.value = false;
  } finally {
    busy.value = false;
  }
}
</script>

<template>
  <AppDialog v-model:open="open" :title="title" :description="message" :busy="busy" width="max-w-[440px]">
    <slot />
    <template #footer>
      <AppButton :disabled="busy" @click="open = false">Cancel</AppButton>
      <AppButton :variant="danger ? 'danger-solid' : 'primary'" :disabled="busy" @click="confirm">
        {{ busy ? "Working…" : confirmLabel }}
      </AppButton>
    </template>
  </AppDialog>
</template>
