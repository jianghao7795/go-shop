<script setup lang="ts">
import { useUserStore } from "../stores/user";
import { adminMenu } from "../lib/admin-menu";

const userStore = useUserStore();
</script>

<template>
  <div class="admin-layout">
    <aside class="admin-sidebar">
      <div class="admin-brand">🛍️ 管理后台</div>
      <template v-for="item in adminMenu" :key="item.path">
        <router-link
          v-if="!item.perm || userStore.hasPerm(item.perm)"
          :to="item.path"
          >{{ item.title }}</router-link
        >
      </template>
      <router-link to="/profile">← 返回商城</router-link>
    </aside>
    <main class="admin-main"><router-view /></main>
  </div>
</template>
