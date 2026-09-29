<script setup lang="ts">
import { onMounted, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { showToast } from "vant";
import http from "../lib/http";

interface Order { id: number; orderNo: string; status: string; amount: number; }

const route = useRoute();
const router = useRouter();
const order = ref<Order | null>(null);
const method = ref("wechat");
const paying = ref(false);

const METHODS = [
  { key: "wechat", label: "微信支付", icon: "💚" },
  { key: "alipay", label: "支付宝", icon: "🔵" },
  { key: "card", label: "银行卡", icon: "💳" },
];

async function load() {
  try {
    const res = await http.get("/api/orders/" + route.params.id);
    order.value = res.data;
  } catch {
    order.value = null;
  }
}

async function pay() {
  if (!order.value) return;
  paying.value = true;
  try {
    await http.post("/api/orders/" + order.value.id + "/pay");
    showToast("支付成功");
    router.replace("/orders");
  } catch (err) {
    const e = err as any;
    if (e?.response) showToast(e.response.data?.message || "支付失败");
    else showToast("支付服务暂不可用");
  } finally {
    paying.value = false;
  }
}

onMounted(load);
</script>

<template>
  <div class="sub-page">
    <van-nav-bar title="收银台" left-arrow @click-left="$router.back()" />
    <van-empty v-if="!order" description="订单不存在" />
    <template v-else>
      <div class="pay-amount">
        <div class="pay-label">应付金额</div>
        <div class="pay-money">¥{{ order.amount.toFixed(2) }}</div>
        <div class="pay-order">订单号 {{ order.orderNo }}</div>
      </div>
      <van-cell-group inset class="pay-methods">
        <van-cell v-for="m in METHODS" :key="m.key" :title="m.label" clickable @click="method = m.key">
          <template #icon><span class="pay-icon">{{ m.icon }}</span></template>
          <template #value><van-icon v-if="method === m.key" name="success" color="#07c160" /></template>
        </van-cell>
      </van-cell-group>
      <div class="pay-submit">
        <van-button block round type="danger" :loading="paying" @click="pay">确认支付</van-button>
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
.pay-label {
  font-size: 13px;
  color: #969ba5;
}
.pay-money {
  margin: 8px 0;
  font-size: 32px;
  font-weight: 700;
  color: #ff4d67;
}
.pay-order {
  font-size: 12px;
  color: #969ba5;
}
.pay-methods {
  margin: 16px;
}
.pay-icon {
  font-size: 20px;
  margin-right: 8px;
}
.pay-submit {
  margin: 24px 16px;
}
</style>
