<script setup lang="ts">
import { onMounted } from "vue";
import { useRoute } from "vue-router";
import ShopTabbar from "./components/ShopTabbar.vue";
import { useUserStore } from "./stores/user";

const route = useRoute();
const userStore = useUserStore();

onMounted(() => { userStore.validate(); });
</script>

<template>
  <RouterView />
  <ShopTabbar v-if="!route.meta.hideTabbar" />
</template>

