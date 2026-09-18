<script setup lang="ts">
import { onMounted, ref } from "vue";
import { useRoute } from "vue-router";
import { showToast } from "vant";
import { useUserStore } from "../stores/user";

interface OrderItem { productId: number; name: string; price: number; quantity: number; emoji: string; color: string; }
interface Order { id: number; orderNo: string; status: string; amount: number; receiver: string; phone: string; region: string; detail: string; items: OrderItem[]; createdAt: string; }

const route = useRoute();
const userStore = useUserStore();
const order = ref<Order | null>(null);
const loading = ref(false);

const statusMap: Record<string, { text: string; color: string }> = {
  pending: { text: "待付款", color: "#ff976a" },
  shipped: { text: "待收货", color: "#1989fa" },
  completed: { text: "待评价", color: "#07c160" },
  aftersale: { text: "售后", color: "#ee0a24" },
  finished: { text: "已完成", color: "#969ba5" },
};

async function load() {
  loading.value = true;
  try {
    const apiBase = import.meta.env.VITE_API_BASE || "http://127.0.0.1:8080";
    const response = await fetch(apiBase + "/api/orders/" + route.params.id, {
      headers: { Authorization: "Bearer " + userStore.token },
    });
    order.value = response.ok ? await response.json() : null;
  } catch {
    order.value = null;
  } finally {
    loading.value = false;
  }
}

async function changeStatus(status: string, text: string) {
  if (!order.value) return;
  const apiBase = import.meta.env.VITE_API_BASE || "http://127.0.0.1:8080";
  const response = await fetch(apiBase + "/api/orders/" + order.value.id + "/status", {
    method: "PUT",
    headers: { "Content-Type": "application/json", Authorization: "Bearer " + userStore.token },
    body: JSON.stringify({ status }),
  });
  if (response.ok) {
    showToast(text + "成功");
    load();
  }
}

onMounted(load);
</script>

<template>
  <div class="sub-page">
    <van-nav-bar title="订单详情" left-arrow @click-left="$router.back()" />
    <div v-if="loading" class="loading"><van-loading color="#ff4d67" /></div>
    <van-empty v-else-if="!order" description="订单不存在" />
    <template v-else>
      <van-cell-group inset class="detail-block">
        <van-cell title="订单号" :value="order.orderNo" />
        <van-cell title="状态">
          <template #value><span :style="{ color: statusMap[order.status]?.color }">{{ statusMap[order.status]?.text || order.status }}</span></template>
        </van-cell>
        <van-cell title="下单时间" :value="new Date(order.createdAt).toLocaleString()" />
      </van-cell-group>
      <van-cell-group v-if="order.receiver" inset class="detail-block">
        <van-cell title="收货信息">
          <template #value>
            <div class="order-receiver">{{ order.receiver }} {{ order.phone }}</div>
            <div class="order-address">{{ order.region }} {{ order.detail }}</div>
          </template>
        </van-cell>
      </van-cell-group>
      <van-cell-group inset class="detail-block">
        <van-cell v-for="item in order.items" :key="item.productId" :title="item.name" :value="'x' + item.quantity + '  ¥' + (item.price * item.quantity).toFixed(2)">
          <template #icon><div class="cart-thumb" :style="{ background: item.color }">{{ item.emoji }}</div></template>
        </van-cell>
        <van-cell title="合计" :value="'¥' + order.amount.toFixed(2)" />
      </van-cell-group>
      <div class="detail-actions-bar">
        <van-button v-if="order.status === 'pending'" round type="danger" @click="changeStatus('shipped', '支付')">去支付</van-button>
        <van-button v-if="order.status === 'shipped'" round type="primary" @click="changeStatus('completed', '确认收货')">确认收货</van-button>
        <van-button v-if="order.status === 'completed'" round type="success" @click="changeStatus('finished', '评价')">评价</van-button>
      </div>
    </template>
  </div>
</template>
