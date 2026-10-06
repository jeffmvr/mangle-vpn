<script setup>
import AppHeader from "@/components/AppHeader.vue";
</script>

<template>
  <div class="flex min-h-screen flex-col">
    <AppHeader />
    <main class="mx-auto w-full max-w-[1120px] px-4 pt-10 pb-16 sm:px-6">
      <RouterView />
    </main>
  </div>
</template>
