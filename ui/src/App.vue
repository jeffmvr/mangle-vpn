<script setup>
import { onMounted } from "vue";

import ToastStack from "@/components/ui/ToastStack.vue";
import { useSession } from "@/stores/session";

const session = useSession();

onMounted(() => session.start());
</script>

<template>
  <!-- Nothing renders until the profile is known, because every view
       needs the user's group or their administrator flag to decide
       what to show. -->
  <RouterView v-if="session.ready" />
  <ToastStack />
</template>
