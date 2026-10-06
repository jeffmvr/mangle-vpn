<script setup>
import AccountMenu from "@/components/AccountMenu.vue";
import BrandMark from "@/components/BrandMark.vue";

/**
 * The bar across the top of the screens every user sees. Staff reach the
 * administration pages from the account menu.
 */
</script>

<template>
  <header class="border-b border-line bg-surface">
    <div class="mx-auto flex max-w-[1120px] flex-wrap items-center justify-between gap-3 px-4 py-3 sm:px-6">
      <BrandMark />
      <AccountMenu />
    </div>
  </header>
</template>
