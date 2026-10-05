<script setup lang="ts">
import { onMounted, onUnmounted, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { showToast } from "vant";
import http from "../lib/http";

interface Order { id: number; orderNo: string; status: string; amount: number; }

const route = useRoute();
const router = useRouter();
const order = ref<Order | null>(null);
const qrImage = ref("");
const loading = ref(true);
const errorMsg = ref("");
let timer: number | undefined;

async function load() {
  try {
    const res = await http.get("/api/orders/" + route.params.id);
    order.value = res.data;
    if (order.value.status !== "pending") {
      // 已支付或后续状态（paid/shipped/completed/finished/aftersale）→ 视为已完成支付
      showToast("订单已支付");
      router.replace("/orders");
      return;
    }
    await createPayment();
  } catch {
    order.value = null;
    loading.value = false;
  }
}

async function createPayment() {
  try {
    const res = await http.post("/api/orders/" + route.params.id + "/pay");
    qrImage.value = res.data.qrImage;
    loading.value = false;
    startPolling();
  } catch (err) {
    const e = err as any;
    if (e?.response?.status === 503) errorMsg.value = e?.response?.data?.message || "支付服务暂未配置";
    else errorMsg.value = e?.response?.data?.message || "支付下单失败";
    loading.value = false;
  }
}

function startPolling() {
  stopPolling();
  timer = window.setInterval(poll, 2000);
}

async function poll() {
  if (!order.value) return;
  try {
    const res = await http.get("/api/orders/" + order.value.id + "/pay/status");
    const status = res.data.status;
    if (status === "paid") {
      stopPolling();
      showToast("支付成功");
      router.replace("/orders");
    } else if (status === "closed") {
      stopPolling();
      errorMsg.value = "支付已关闭，请重新下单";
    }
  } catch (err) {
    const e = err as any;
    // 接口返回错误（非网络抖动）：提示并停止轮询；无响应的网络抖动则忽略继续
    if (e?.response) {
      stopPolling();
      errorMsg.value = e.response.data?.message || "支付状态查询失败，请稍后重试";
    }
  }
}

function stopPolling() {
  if (timer !== undefined) {
    clearInterval(timer);
    timer = undefined;
  }
}

onMounted(load);
onUnmounted(stopPolling);
</script>

<template>
  <div class="sub-page">
    <van-nav-bar title="收银台" left-arrow @click-left="$router.back()" />
    <van-empty v-if="!order" description="订单不存在" />
    <van-empty v-else-if="errorMsg" :description="errorMsg" />
    <template v-else>
      <div class="pay-amount">
        <div class="pay-label">应付金额</div>
        <div class="pay-money">¥{{ order.amount.toFixed(2) }}</div>
        <div class="pay-order">订单号 {{ order.orderNo }}</div>
      </div>
      <div class="pay-qr">
        <van-loading v-if="loading" size="40px" vertical>生成收款码中…</van-loading>
        <img v-else-if="qrImage" :src="qrImage" alt="支付宝收款码" />
      </div>
      <div class="pay-tip">请使用支付宝扫码付款</div>
      <div class="pay-refresh">
        <van-button block round type="primary" @click="poll">已付款？点此刷新</van-button>
      </div>
    </template>
  </div>
</template>

<style scoped>
.pay-amount {
  margin: 16px;
  padding: 24px 16px;
  background: #fff;
  border-radius: 10px;
  text-align: center;
}
.pay-label { font-size: 13px; color: #969ba5; }
.pay-money { margin: 8px 0; font-size: 32px; font-weight: 700; color: #ff4d67; }
.pay-order { font-size: 12px; color: #969ba5; }
.pay-qr {
  display: flex;
  justify-content: center;
  align-items: center;
  min-height: 220px;
  margin: 16px;
  background: #fff;
  border-radius: 10px;
}
.pay-qr img { width: 200px; height: 200px; }
.pay-tip { text-align: center; font-size: 13px; color: #969ba5; }
.pay-refresh { margin: 24px 16px; }
</style>
