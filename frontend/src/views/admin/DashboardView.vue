<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import http from "../../lib/http";
import { useUserStore } from "../../stores/user";

const userStore = useUserStore();
const canProducts = computed(() => userStore.permissions.includes("product:manage"));
const canOrders = computed(() => userStore.permissions.includes("order:manage"));
const canUsers = computed(() => userStore.permissions.includes("user:manage"));

const counts = ref<{ products: number; orders: number; users: number }>({
  products: 0,
  orders: 0,
  users: 0,
});

async function load() {
  const tasks: Promise<void>[] = [];
  if (canProducts.value) {
    tasks.push(http.get("/api/admin/products").then((r) => (counts.value.products = r.data.total)));
  }
  if (canOrders.value) {
    tasks.push(http.get("/api/admin/orders").then((r) => (counts.value.orders = r.data.total)));
  }
  if (canUsers.value) {
    tasks.push(http.get("/api/admin/users").then((r) => (counts.value.users = r.data.total)));
  }
  try {
    await Promise.all(tasks);
  } catch {
    /* 统计加载失败时保留 0 */
  }
}

onMounted(load);
</script>

<template>
  <div class="admin-page">
    <h2>概览</h2>
    <div class="stat-grid">
      <div v-if="canProducts" class="stat-tile">
        <div class="stat-value">{{ counts.products }}</div>
        <div class="stat-label">商品数</div>
      </div>
      <div v-if="canOrders" class="stat-tile">
        <div class="stat-value">{{ counts.orders }}</div>
        <div class="stat-label">订单数</div>
      </div>
      <div v-if="canUsers" class="stat-tile">
        <div class="stat-value">{{ counts.users }}</div>
        <div class="stat-label">用户数</div>
      </div>
    </div>
  </div>
</template>
