<script setup>
import { computed } from "vue";
import { RouterLink } from "vue-router";

/**
 * The one button in the application.
 *
 * `primary` is the main action on a screen and there is at most one.
 * `danger` is for anything that disconnects, revokes or deletes, and
 * `danger-solid` confirms one of those inside a dialog. Given `to` it
 * renders a router link, given `href` a plain link, so a link that looks
 * like a button is still a link.
 */
const props = defineProps({
  variant: { type: String, default: "secondary" },
  size: { type: String, default: "md" },
  type: { type: String, default: "button" },
  disabled: { type: Boolean, default: false },
  to: { type: [String, Object], default: null },
  href: { type: String, default: null },
});

const variants = {
  primary: "border-transparent bg-accent text-white font-semibold hover:bg-accent-strong",
  secondary: "border-field bg-surface text-ink-2 font-medium hover:bg-subtle",
  danger: "border-deny-line bg-surface text-deny font-medium hover:bg-deny-soft",
  "danger-solid": "border-transparent bg-deny text-white font-semibold hover:bg-[#912018]",
  ghost: "border-transparent bg-transparent text-ink-3 font-medium hover:bg-muted hover:text-ink",
};

const sizes = {
  sm: "h-8 px-3 text-[13px] gap-1.5 rounded-lg",
  md: "h-10 px-4 text-sm gap-2 rounded-lg",
};

const classes = computed(() => [
  "inline-flex shrink-0 items-center justify-center border whitespace-nowrap no-underline transition-colors",
  "disabled:cursor-not-allowed disabled:opacity-50",
  variants[props.variant],
  sizes[props.size],
]);
</script>

<template>
  <RouterLink v-if="to" :to="to" :class="classes"><slot /></RouterLink>
  <a v-else-if="href" :href="href" :class="classes"><slot /></a>
  <button v-else :type="type" :disabled="disabled" :class="classes"><slot /></button>
</template>
