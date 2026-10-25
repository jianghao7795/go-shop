<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from "vue";
import { useRoute } from "vue-router";
import http from "../lib/http";
import { errMsg } from "../lib/errmsg";

interface Order { id: number; orderNo: string; status: string; amount: number; createdAt: string; }
const route = useRoute();
const order = ref<Order | null>(null);
const qrImage = ref("");
const loading = ref(true);
const creating = ref(false);
const checking = ref(false);
const loadError = ref("");
const paymentError = ref("");
const statusError = ref("");
const expiresAt = ref(0);
const serverOffset = ref(0);
const now = ref(Date.now());
let pollTimer: number | undefined;
let clockTimer: number | undefined;
let generation = 0;
let active = true;
let controller = new AbortController();

const pending = computed(() => order.value?.status === "pending");
const remainingSeconds = computed(() => Math.max(0, Math.ceil((expiresAt.value - now.value - serverOffset.value) / 1000)));
const expired = computed(() => expiresAt.value > 0 && remainingSeconds.value === 0);
const countdown = computed(() => {
  const seconds = remainingSeconds.value;
  return String(Math.floor(seconds / 60)).padStart(2, "0") + ":" + String(seconds % 60).padStart(2, "0");
});
const resultText = computed(() => {
  if (!order.value) return "";
  if (order.value.status === "cancelled") return "订单已关闭";
  if (order.value.status === "refunded") return "订单已退款";
  if (order.value.status === "aftersale") return "订单正在售后处理中";
  if (["paid", "shipped", "completed", "finished"].includes(order.value.status)) return "支付成功";
  return "请查看订单详情确认当前状态";
});

function setTiming(data: { expiresAt?: string; serverTime?: string }) {
  if (data.expiresAt) {
    const deadline = Date.parse(data.expiresAt);
    if (Number.isFinite(deadline)) expiresAt.value = deadline;
  }
  if (data.serverTime) {
    const serverTime = Date.parse(data.serverTime);
    if (Number.isFinite(serverTime)) serverOffset.value = serverTime - Date.now();
  }
  now.value = Date.now();
}

function stopPolling() {
  if (pollTimer !== undefined) window.clearTimeout(pollTimer);
  pollTimer = undefined;
}
function schedulePoll() {
  stopPolling();
  if (!active || !pending.value) return;
  pollTimer = window.setTimeout(async () => {
    if (document.visibilityState === "visible") await checkStatus();
    schedulePoll();
  }, 2000);
}

async function checkStatus(): Promise<boolean> {
  if (!order.value || checking.value || !active) return false;
  const version = generation;
  checking.value = true;
  try {
    const r = await http.get("/api/orders/" + order.value.id + "/pay/status", { signal: controller.signal, timeout: 15000 });
    if (version !== generation || !active) return false;
    setTiming(r.data);
    const states: Record<string, string> = { pending: "pending", paid: "paid", closed: "cancelled", refunded: "refunded", aftersale: "aftersale" };
    const state = r.data.orderStatus || states[r.data.status];
    if (!state) { statusError.value = "暂时无法确认支付状态，请稍后重试"; return false; }
    order.value.status = state;
    statusError.value = "";
    if (!pending.value) { stopPolling(); qrImage.value = ""; paymentError.value = ""; }
    return true;
  } catch (e) {
    if (version === generation && active) statusError.value = errMsg(e, "网络暂不可用，请重试查询支付结果");
    return false;
  } finally {
    if (version === generation && active) checking.value = false;
  }
}

async function createPayment() {
  if (!order.value || !pending.value || creating.value || expired.value) return;
  const version = generation;
  creating.value = true;
  paymentError.value = "";
  qrImage.value = "";
  stopPolling();
  try {
    const r = await http.post("/api/orders/" + order.value.id + "/pay", undefined, { signal: controller.signal, timeout: 15000 });
    if (version !== generation || !active) return;
    setTiming(r.data);
    if (!r.data.qrImage) { paymentError.value = "暂未获取到收款码，请重试"; return; }
    if (!expired.value) qrImage.value = r.data.qrImage;
    schedulePoll();
  } catch (e) {
    if (version !== generation || !active) return;
    const error = e as any;
    paymentError.value = error?.response?.status === 503 ? "支付服务暂不可用，请稍后重试或联系客服" : errMsg(e, "获取收款码失败，请重试");
  } finally {
    if (version === generation && active) creating.value = false;
  }
}

async function retryPayment() {
  if (creating.value || checking.value || !pending.value) return;
  if (await checkStatus()) {
    if (expired.value) schedulePoll(); else await createPayment();
  }
}

async function load() {
  const version = ++generation;
  controller.abort();
  controller = new AbortController();
  stopPolling();
  order.value = null; qrImage.value = ""; loadError.value = ""; paymentError.value = ""; statusError.value = "";
  expiresAt.value = 0; serverOffset.value = 0; creating.value = false; checking.value = false; loading.value = true;
  try {
    const r = await http.get("/api/orders/" + route.params.id, { signal: controller.signal, timeout: 15000 });
    if (version !== generation || !active) return;
    order.value = r.data;
    if (pending.value && await checkStatus()) {
      if (expired.value) schedulePoll(); else await createPayment();
    }
  } catch (e) {
    if (version === generation && active) loadError.value = errMsg(e, "读取订单失败，请稍后重试");
  } finally {
    if (version === generation && active) loading.value = false;
  }
}

watch(expired, value => {
  if (value && pending.value) qrImage.value = "";
});
watch(() => route.params.id, load, { immediate: true });
onMounted(() => { clockTimer = window.setInterval(() => { now.value = Date.now(); }, 1000); });
onUnmounted(() => {
  active = false; generation++; controller.abort(); stopPolling();
  if (clockTimer !== undefined) window.clearInterval(clockTimer);
});
</script>

<template>
  <div class="sub-page">
    <van-nav-bar title="收银台" left-arrow @click-left="$router.back()" />
    <div v-if="loading" class="pay-qr"><van-loading vertical>正在读取支付信息…</van-loading></div>
    <van-empty v-else-if="loadError || !order" :description="loadError || '订单不存在'">
      <van-button type="primary" @click="load">重新加载</van-button>
    </van-empty>
    <template v-else>
      <div class="pay-amount">
        <div class="pay-label">订单金额</div>
        <div class="pay-money">¥{{ order.amount.toFixed(2) }}</div>
        <div class="pay-order">订单号 {{ order.orderNo }}</div>
      </div>
      <van-empty v-if="!pending" :description="resultText">
        <van-button type="primary" :to="'/orders/' + order.id">查看订单详情</van-button>
      </van-empty>
      <template v-else>
        <div v-if="expiresAt" class="pay-deadline" role="status">{{ expired ? '已超过付款时间，正在等待最终支付结果' : '剩余付款时间 ' + countdown }}</div>
        <div v-if="!expired" class="pay-qr">
          <van-loading v-if="creating" vertical>生成收款码中…</van-loading>
          <img v-else-if="qrImage" :src="qrImage" alt="支付宝收款码" />
          <van-empty v-else :description="paymentError || '暂未获取到收款码'" />
        </div>
        <div v-if="qrImage && !expired" class="pay-tip">请使用支付宝扫码付款</div>
        <div v-if="statusError" class="pay-error" role="alert">{{ statusError }}</div>
        <div class="pay-actions">
          <van-button v-if="!qrImage && !expired" block round type="primary" :loading="creating" :disabled="checking" @click="retryPayment">重新获取收款码</van-button>
          <van-button block round :loading="checking" :disabled="creating" @click="checkStatus">{{ expired ? '查询最终支付结果' : '已付款？点此刷新' }}</van-button>
          <van-button block round :to="'/orders/' + order.id">查看订单详情</van-button>
          <van-button v-if="paymentError" block round plain to="/service">联系客服</van-button>
        </div>
      </template>
    </template>
  </div>
</template>

<style scoped>
.pay-amount { margin: 16px; padding: 24px 16px; background: #fff; border-radius: 10px; text-align: center; }
.pay-label { font-size: 13px; color: #969ba5; }
.pay-money { margin: 8px 0; font-size: 32px; font-weight: 700; color: #ff4d67; }
.pay-order { font-size: 12px; color: #969ba5; overflow-wrap: anywhere; }
.pay-qr { display: flex; justify-content: center; align-items: center; min-height: 220px; margin: 16px; background: #fff; border-radius: 10px; }
.pay-qr img { width: 200px; height: 200px; }
.pay-tip, .pay-deadline { text-align: center; font-size: 13px; color: #646a73; padding: 0 16px; }
.pay-error { margin: 12px 16px; color: #ee0a24; text-align: center; }
.pay-actions { display: grid; gap: 12px; margin: 24px 16px; }
</style>
