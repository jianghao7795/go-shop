<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { useRouter } from "vue-router";
import { showToast } from "vant";
import { useShopCart } from "../stores/shop";
import { useUserStore } from "../stores/user";

interface Address { id: number; name: string; phone: string; region: string; detail: string; isDefault: boolean; }

const router = useRouter();
const { cartItems, total, clear } = useShopCart();
const userStore = useUserStore();

const addresses = ref<Address[]>([]);
const submitting = ref(false);

const defaultAddress = computed(() => addresses.value.find(a => a.isDefault) || addresses.value[0] || null);

async function loadAddresses() {
  const apiBase = import.meta.env.VITE_API_BASE || "http://127.0.0.1:8080";
  const response = await fetch(apiBase + "/api/addresses", {
    headers: { Authorization: "Bearer " + userStore.token },
  });
  if (response.ok) addresses.value = await response.json();
}

async function submitOrder() {
  const addr = defaultAddress.value;
  if (!cartItems.value.length) { showToast("购物车为空"); return; }
  if (!addr) { showToast("请先添加收货地址"); router.push("/address/edit"); return; }
  submitting.value = true;
  try {
    const apiBase = import.meta.env.VITE_API_BASE || "http://127.0.0.1:8080";
    const items = cartItems.value.map(it => ({
      productId: it.product.id,
      name: it.product.name,
      price: it.product.price,
      quantity: it.quantity,
      emoji: it.product.emoji,
      color: it.product.color,
    }));
    const response = await fetch(apiBase + "/api/orders", {
      method: "POST",
      headers: { "Content-Type": "application/json", Authorization: "Bearer " + userStore.token },
      body: JSON.stringify({ addressId: addr.id, items }),
    });
    const data = await response.json();
    if (!response.ok) { showToast(data.message || "下单失败"); return; }
    clear();
    showToast("下单成功");
    router.replace("/orders");
  } catch {
    showToast("下单服务暂不可用");
  } finally {
    submitting.value = false;
  }
}

onMounted(loadAddresses);
</script>

<template>
  <div class="sub-page">
    <van-nav-bar title="确认订单" left-arrow @click-left="$router.back()" />
    <van-empty v-if="!cartItems.length" description="购物车为空" />
    <template v-else>
      <van-cell-group inset class="checkout-address">
        <van-cell is-link @click="router.push('/address')">
          <template #title>
            <div v-if="defaultAddress">
              <div class="address-title"><strong>{{ defaultAddress.name }}</strong><span>{{ defaultAddress.phone }}</span></div>
              <div class="address-detail">{{ defaultAddress.region }} {{ defaultAddress.detail }}</div>
            </div>
            <div v-else class="checkout-no-addr">请选择收货地址</div>
          </template>
        </van-cell>
      </van-cell-group>
      <van-cell-group inset class="checkout-items">
        <van-cell v-for="item in cartItems" :key="item.product.id" :title="item.product.name" :value="'x' + item.quantity + '  ¥' + (item.product.price * item.quantity).toFixed(2)">
          <template #icon><div class="cart-thumb" :style="{ background: item.product.color }">{{ item.product.emoji }}</div></template>
        </van-cell>
      </van-cell-group>
      <div class="checkout-total"><span>合计</span><b>¥{{ total.toFixed(2) }}</b></div>
      <div class="checkout-submit">
        <van-button block round type="danger" :loading="submitting" @click="submitOrder">提交订单</van-button>
      </div>
    </template>
  </div>
</template>
