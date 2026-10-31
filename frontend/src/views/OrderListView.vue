<script setup lang="ts">
import { computed, onUnmounted, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { showConfirmDialog, showToast } from "vant";
import http from "../lib/http";
import AftersaleForm from "../components/AftersaleForm.vue";
import { canApplyAftersale, orderStatuses as statusMap, type Order } from "../lib/orders";
import { errMsg } from "../lib/errmsg";

const route = useRoute();
const router = useRouter();
const active = ref(0);
const orders = ref<Order[]>([]);
const loading = ref(false);
const loadingMore = ref(false);
const loadError = ref("");
let requestVersion = 0;
const aftersaleOrder = ref<Order | null>(null);
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
  { name: "refunded", title: "已退款" },
  { name: "finished", title: "已完成" },
  { name: "cancelled", title: "已取消" },
];

async function loadOrders() {
  const version = ++requestVersion;
  loading.value = true;
  loadingMore.value = false;
  loadError.value = "";
  try {
    const status = tabs[active.value].name;
    const res = await http.get("/api/orders", { params: { ...(status ? { status } : {}), page: 1, pageSize: 20 } });
    if (version !== requestVersion) return;
    orders.value = res.data.items || [];
    total.value = res.data.total;
    page.value = 1;
  } catch {
    if (version !== requestVersion) return;
    orders.value = [];
    total.value = 0;
    loadError.value = "订单加载失败，请稍后重试";
  } finally {
    if (version === requestVersion) loading.value = false;
  }
}

async function loadMore() {
  if (finished.value || loading.value || loadingMore.value) return;
  const version = requestVersion;
  const nextPage = page.value + 1;
  loadingMore.value = true;
  try {
    const status = tabs[active.value].name;
    const res = await http.get("/api/orders", { params: { ...(status ? { status } : {}), page: nextPage, pageSize: 20 } });
    if (version !== requestVersion) return;
    orders.value.push(...(res.data.items || []));
    total.value = res.data.total;
    page.value = nextPage;
  } catch {
    if (version === requestVersion) showToast("加载失败，请重试");
  } finally {
    if (version === requestVersion) loadingMore.value = false;
  }
}

async function changeStatus(order: Order, status: string, text: string) {
  try {
    await http.put("/api/orders/" + order.id + "/status", { status });
    showToast(text + "成功");
    loadOrders();
  } catch (e) {
    showToast(errMsg(e, "操作失败，请稍后重试"));
  }
}

async function cancelOrder(order: Order) {
  try {
    await showConfirmDialog({ title: "取消订单", message: "确定取消该订单吗？" });
  } catch { return; }
  await changeStatus(order, "cancelled", "取消订单");
}

watch(() => route.query.status, () => {
  const status = String(route.query.status || "");
  const idx = tabs.findIndex(t => t.name === status);
  active.value = Math.max(0, idx);
}, { immediate: true });
watch(active, loadOrders, { immediate: true });
onUnmounted(() => { requestVersion++; });
</script>

<template>
  <div class="sub-page">
    <van-nav-bar title="我的订单" left-arrow @click-left="$router.back()" />
    <van-tabs v-model:active="active" sticky>
      <van-tab v-for="tab in tabs" :key="tab.title" :title="tab.title" />
    </van-tabs>
    <div v-if="loading" class="loading"><van-loading color="#ff4d67" /></div>
    <van-empty v-else-if="loadError" :description="loadError"><van-button type="primary" @click="loadOrders">重新加载</van-button></van-empty>
    <van-empty v-else-if="!orders.length" description="暂无相关订单" />
    <template v-else>
      <van-cell-group v-for="order in orders" :key="order.id" inset class="order-card">
        <van-cell :title="'订单号 ' + order.orderNo" is-link @click="router.push({ name: 'order-detail', params: { id: order.id } })">
          <template #value><span class="order-status" :style="{ color: statusMap[order.status]?.color }">{{ statusMap[order.status]?.text || order.status }}</span></template>
        </van-cell>
        <van-cell v-for="item in order.items" :key="item.productId + ':' + item.skuId" :title="item.name" :label="item.specs" :value="'x' + item.quantity">
          <template #icon><div class="cart-thumb" :style="{ background: item.color }">{{ item.emoji }}</div></template>
        </van-cell>
        <van-cell title="合计" :value="'¥' + order.amount.toFixed(2)" />
        <div class="order-actions">
          <van-button v-if="order.status === 'pending'" size="small" round type="danger" @click="router.push('/pay/' + order.id)">去支付</van-button>
          <van-button v-if="order.status === 'pending'" size="small" round plain type="default" @click="cancelOrder(order)">取消订单</van-button>
          <van-button v-if="order.status === 'shipped'" size="small" round type="primary" @click="changeStatus(order, 'completed', '确认收货')">确认收货</van-button>
          <van-button v-if="canApplyAftersale(order)" size="small" round plain type="warning" @click="aftersaleOrder = order">申请售后</van-button>
          <van-button v-if="order.status === 'completed'" size="small" round type="success" @click="router.push('/review/' + order.id)">评价</van-button>
        </div>
      </van-cell-group>
      <div v-if="!finished" class="load-more">
        <van-button size="small" plain :loading="loadingMore" @click="loadMore">加载更多</van-button>
      </div>
    </template>
    <AftersaleForm :order="aftersaleOrder" @close="aftersaleOrder = null" @submitted="loadOrders" />
  </div>
</template>
