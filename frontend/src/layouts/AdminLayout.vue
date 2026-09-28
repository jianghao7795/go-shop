<script setup lang="ts">
import { useUserStore } from "../stores/user";

const userStore = useUserStore();
</script>

<template>
  <div class="admin-layout">
    <aside class="admin-sidebar">
      <div class="admin-brand">🛍️ 管理后台</div>
      <router-link to="/admin">概览</router-link>
      <router-link v-if="userStore.permissions.includes('product:manage')" to="/admin/products">商品</router-link>
      <router-link v-if="userStore.permissions.includes('category:manage')" to="/admin/categories">分类</router-link>
      <router-link v-if="userStore.permissions.includes('order:manage')" to="/admin/orders">订单</router-link>
      <router-link v-if="userStore.permissions.includes('user:manage')" to="/admin/users">用户</router-link>
      <router-link v-if="userStore.permissions.includes('user:manage')" to="/admin/roles">角色</router-link>
      <router-link v-if="userStore.permissions.includes('coupon:manage')" to="/admin/coupons">优惠券</router-link>
      <router-link v-if="userStore.permissions.includes('notification:manage')" to="/admin/notifications">通知</router-link>
      <router-link to="/profile">← 返回商城</router-link>
    </aside>
    <main class="admin-main"><router-view /></main>
  </div>
</template>
