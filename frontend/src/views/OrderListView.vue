<script setup lang="ts">
import { computed, onMounted, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { showConfirmDialog, showToast } from "vant";
import http from "../lib/http";

interface OrderItem { productId: number; name: string; price: number; quantity: number; emoji: string; color: string; }
interface Order { id: number; orderNo: string; status: string; amount: number; items: OrderItem[]; createdAt: string; }

const route = useRoute();
const router = useRouter();
const active = ref(0);
const orders = ref<Order[]>([]);
const loading = ref(false);
const page = ref(1);
const total = ref(0);
const finished = computed(() => orders.value.length >= total.value);

const tabs = [
  { name: "", title: "全部" },
  { name: "pending", title: "待付款" },
  { name: "paid", title: "待发货" },
  { name: "shipped", title: "待收货" },
  { name: "completed", title: "待评价" },
  { name: "aftersale", title: "售后" },
  { name: "cancelled", title: "已取消" },
];

const statusMap: Record<string, { text: string; color: string }> = {
  pending: { text: "待付款", color: "#ff976a" },
  paid: { text: "待发货", color: "#7c4dff" },
  shipped: { text: "待收货", color: "#1989fa" },
  completed: { text: "待评价", color: "#07c160" },
  aftersale: { text: "售后", color: "#ee0a24" },
  finished: { text: "已完成", color: "#969ba5" },
  cancelled: { text: "已取消", color: "#969ba5" },
};

async function loadOrders() {
  loading.value = true;
  try {
    const status = tabs[active.value].name;
    const res = await http.get("/api/orders", { params: { ...(status ? { status } : {}), page: 1, pageSize: 20 } });
    orders.value = res.data.items || [];
    total.value = res.data.total;
    page.value = 1;
  } catch {
    orders.value = [];
  } finally {
    loading.value = false;
  }
}

async function loadMore() {
  if (finished.value || loading.value) return;
  try {
    const status = tabs[active.value].name;
    const res = await http.get("/api/orders", { params: { ...(status ? { status } : {}), page: page.value + 1, pageSize: 20 } });
    orders.value.push(...(res.data.items || []));
    total.value = res.data.total;
    page.value += 1;
  } catch { /* 忽略 */ }
}

async function changeStatus(order: Order, status: string, text: string) {
  try {
    await http.put("/api/orders/" + order.id + "/status", { status });
    showToast(text + "成功");
    loadOrders();
  } catch {
    showToast("操作失败，请稍后重试");
  }
}

async function cancelOrder(order: Order) {
  try {
    await showConfirmDialog({ title: "取消订单", message: "确定取消该订单吗？" });
  } catch { return; }
  await changeStatus(order, "cancelled", "取消订单");
}

async function applyAftersale(order: Order) {
  try {
    await showConfirmDialog({ title: "申请售后", message: "确定申请售后吗？" });
  } catch { return; }
  await changeStatus(order, "aftersale", "申请售后");
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
          <van-button v-if="order.status === 'pending'" size="small" round type="danger" @click="router.push('/pay/' + order.id)">去支付</van-button>
          <van-button v-if="order.status === 'pending'" size="small" round plain type="default" @click="cancelOrder(order)">取消订单</van-button>
          <van-button v-if="order.status === 'shipped'" size="small" round type="primary" @click="changeStatus(order, 'completed', '确认收货')">确认收货</van-button>
          <van-button v-if="order.status === 'paid' || order.status === 'shipped'" size="small" round plain type="warning" @click="applyAftersale(order)">申请售后</van-button>
          <van-button v-if="order.status === 'completed'" size="small" round type="success" @click="router.push('/review/' + order.id)">评价</van-button>
        </div>
      </van-cell-group>
      <div v-if="!finished" class="load-more">
        <van-button size="small" plain @click="loadMore">加载更多</van-button>
      </div>
    </template>
  </div>
</template>
