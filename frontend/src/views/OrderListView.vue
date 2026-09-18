<script setup lang="ts">
import { onMounted, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { showToast } from "vant";
import { useUserStore } from "../stores/user";

interface OrderItem { productId: number; name: string; price: number; quantity: number; emoji: string; color: string; }
interface Order { id: number; orderNo: string; status: string; amount: number; items: OrderItem[]; createdAt: string; }

const userStore = useUserStore();
const route = useRoute();
const router = useRouter();
const active = ref(0);
const orders = ref<Order[]>([]);
const loading = ref(false);

const tabs = [
  { name: "", title: "全部" },
  { name: "pending", title: "待付款" },
  { name: "shipped", title: "待收货" },
  { name: "completed", title: "待评价" },
  { name: "aftersale", title: "售后" },
];

const statusMap: Record<string, { text: string; color: string }> = {
  pending: { text: "待付款", color: "#ff976a" },
  shipped: { text: "待收货", color: "#1989fa" },
  completed: { text: "待评价", color: "#07c160" },
  aftersale: { text: "售后", color: "#ee0a24" },
  finished: { text: "已完成", color: "#969ba5" },
};

async function loadOrders() {
  loading.value = true;
  try {
    const apiBase = import.meta.env.VITE_API_BASE || "http://127.0.0.1:8080";
    const status = tabs[active.value].name;
    const url = apiBase + "/api/orders" + (status ? "?status=" + status : "");
    const response = await fetch(url, { headers: { Authorization: "Bearer " + userStore.token } });
    orders.value = response.ok ? await response.json() : [];
  } catch {
    orders.value = [];
  } finally {
    loading.value = false;
  }
}

async function changeStatus(order: Order, status: string, text: string) {
  try {
    const apiBase = import.meta.env.VITE_API_BASE || "http://127.0.0.1:8080";
    const response = await fetch(apiBase + "/api/orders/" + order.id + "/status", {
      method: "PUT",
      headers: { "Content-Type": "application/json", Authorization: "Bearer " + userStore.token },
      body: JSON.stringify({ status }),
    });
    if (response.ok) {
      showToast(text + "成功");
      loadOrders();
    } else {
      showToast("操作失败");
    }
  } catch {
    showToast("操作失败，请稍后重试");
  }
}

watch(active, loadOrders);
onMounted(() => {
  const status = String(route.query.status || "");
  const idx = tabs.findIndex(t => t.name === status);
  if (idx >= 0) active.value = idx;
  loadOrders();
});
</script>

<template>
  <div class="sub-page">
    <van-nav-bar title="我的订单" left-arrow @click-left="$router.back()" />
    <van-tabs v-model:active="active" sticky>
      <van-tab v-for="tab in tabs" :key="tab.title" :title="tab.title" />
    </van-tabs>
    <div v-if="loading" class="loading"><van-loading color="#ff4d67" /></div>
    <van-empty v-else-if="!orders.length" description="暂无相关订单" />
    <template v-else>
      <van-cell-group v-for="order in orders" :key="order.id" inset class="order-card">
        <van-cell :title="'订单号 ' + order.orderNo" is-link @click="router.push({ name: 'order-detail', params: { id: order.id } })">
          <template #value><span class="order-status" :style="{ color: statusMap[order.status]?.color }">{{ statusMap[order.status]?.text || order.status }}</span></template>
        </van-cell>
        <van-cell v-for="item in order.items" :key="item.productId" :title="item.name" :value="'x' + item.quantity">
          <template #icon><div class="cart-thumb" :style="{ background: item.color }">{{ item.emoji }}</div></template>
        </van-cell>
        <van-cell title="合计" :value="'¥' + order.amount.toFixed(2)" />
        <div class="order-actions">
          <van-button v-if="order.status === 'pending'" size="small" round type="danger" @click="changeStatus(order, 'shipped', '支付')">去支付</van-button>
          <van-button v-if="order.status === 'shipped'" size="small" round type="primary" @click="changeStatus(order, 'completed', '确认收货')">确认收货</van-button>
          <van-button v-if="order.status === 'completed'" size="small" round type="success" @click="changeStatus(order, 'finished', '评价')">评价</van-button>
        </div>
      </van-cell-group>
    </template>
  </div>
</template>
