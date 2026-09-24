<script setup lang="ts">
import { onMounted, ref } from "vue";
import http from "../../lib/http";

const counts = ref<{ products: number; orders: number; users: number }>({
  products: 0,
  orders: 0,
  users: 0,
});

async function load() {
  try {
    const [products, orders, users] = await Promise.all([
      http.get("/api/admin/products"),
      http.get("/api/admin/orders"),
      http.get("/api/admin/users"),
    ]);
    counts.value.products = products.data.total;
    counts.value.orders = orders.data.total;
    counts.value.users = users.data.total;
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
      <div class="stat-tile">
        <div class="stat-value">{{ counts.products }}</div>
        <div class="stat-label">商品数</div>
      </div>
      <div class="stat-tile">
        <div class="stat-value">{{ counts.orders }}</div>
        <div class="stat-label">订单数</div>
      </div>
      <div class="stat-tile">
        <div class="stat-value">{{ counts.users }}</div>
        <div class="stat-label">用户数</div>
      </div>
    </div>
  </div>
</template>

