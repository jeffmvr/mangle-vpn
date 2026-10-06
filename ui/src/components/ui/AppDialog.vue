<script setup>
import { X } from "lucide-vue-next";
import { nextTick, onUnmounted, ref, useId, watch } from "vue";

/**
 * A modal dialog.
 *
 * Escape and the backdrop close it unless it is busy, focus moves into it
 * when it opens and returns to where it was when it closes, and the page
 * behind it does not scroll.
 */
const props = defineProps({
  title: { type: String, required: true },
  description: { type: String, default: "" },
  width: { type: String, default: "max-w-[520px]" },
  busy: { type: Boolean, default: false },
});

const open = defineModel("open", { type: Boolean, default: false });

const titleId = useId();
const panel = ref(null);
let returnFocus = null;

function close() {
  if (!props.busy) open.value = false;
}

function onKeydown(event) {
  if (event.key === "Escape") close();
}

watch(open, async (value) => {
  if (value) {
    returnFocus = document.activeElement;
    document.body.style.overflow = "hidden";
    await nextTick();
    const target =
      panel.value?.querySelector("[autofocus]") ??
      panel.value?.querySelector("input, select, textarea, button:not([data-close])");
    target?.focus();
  } else {
    document.body.style.overflow = "";
    returnFocus?.focus?.();
  }
});

onUnmounted(() => {
  document.body.style.overflow = "";
});
</script>

<template>
  <Teleport to="body">
    <Transition
      enter-active-class="transition duration-150 ease-out"
      enter-from-class="opacity-0"
      leave-active-class="transition duration-100 ease-in"
      leave-to-class="opacity-0"
    >
      <div
        v-if="open"
        class="fixed inset-0 z-50 flex items-center justify-center overflow-y-auto bg-ink/55 px-4 py-12"
        @mousedown.self="close"
        @keydown="onKeydown"
      >
        <div
          ref="panel"
          role="dialog"
          aria-modal="true"
          :aria-labelledby="titleId"
          class="flex w-full flex-col rounded-2xl bg-surface shadow-[0_24px_48px_rgba(16,24,40,0.24)]"
          :class="width"
        >
          <div class="flex items-start justify-between gap-4 px-6 pt-6">
            <div class="flex flex-col gap-1.5">
              <h2 :id="titleId" class="text-[19px] font-semibold tracking-[-0.01em]">{{ title }}</h2>
              <p v-if="description" class="leading-normal text-ink-3">{{ description }}</p>
            </div>
            <button
              type="button"
              data-close
              aria-label="Close"
              class="-mt-1 -mr-2 flex size-9 shrink-0 items-center justify-center rounded-lg text-ink-4 hover:bg-muted hover:text-ink"
              @click="close"
            >
              <X class="size-[18px]" :stroke-width="2" />
            </button>
          </div>

          <div class="flex flex-col gap-5 p-6">
            <slot />
          </div>

          <div
            v-if="$slots.footer"
            class="flex flex-wrap justify-end gap-2.5 border-t border-hair px-6 py-4"
          >
            <slot name="footer" />
          </div>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>
