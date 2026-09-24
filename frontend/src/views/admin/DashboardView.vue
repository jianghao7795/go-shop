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
    counts.value.products = products.data.length;
    counts.value.orders = orders.data.length;
    counts.value.users = users.data.length;
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

<style scoped>
.stat-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 12px;
}
.stat-tile {
  background: #fff;
  border-radius: 8px;
  padding: 20px 12px;
  text-align: center;
}
.stat-value {
  font-size: 28px;
  font-weight: 700;
  color: #ff4d67;
}
.stat-label {
  margin-top: 6px;
  font-size: 13px;
  color: #969ba5;
}
</style>
