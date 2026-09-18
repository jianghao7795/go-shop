<script setup lang="ts">
import { onMounted, watch } from "vue";
import { useRoute } from "vue-router";
import ShopTabbar from "./components/ShopTabbar.vue";
import { useUserStore } from "./stores/user";
import { useNotificationStore } from "./stores/notification";

const route = useRoute();
const userStore = useUserStore();
const notificationStore = useNotificationStore();

onMounted(() => {
  userStore.validate();
  if (userStore.isLoggedIn) notificationStore.connect();
});

watch(() => userStore.isLoggedIn, (loggedIn) => {
  if (loggedIn) notificationStore.connect();
  else notificationStore.disconnect();
});
</script>

<template>
  <RouterView />
  <ShopTabbar v-if="!route.meta.hideTabbar" />
</template>

