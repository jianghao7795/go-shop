<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { useRouter } from "vue-router";
import { storeToRefs } from "pinia";
import { showConfirmDialog, showToast } from "vant";
import { useShopCart, type ShopProduct } from "../stores/shop";
import http from "../lib/http";

interface Address { id: number; name: string; phone: string; region: string; detail: string; isDefault: boolean; }

const router = useRouter();
const shopCart = useShopCart();
const { cartItems, total } = storeToRefs(shopCart);
const { clear, increase, decrease, remove } = shopCart;

const addresses = ref<Address[]>([]);

async function onMinus(item: { product: ShopProduct; quantity: number }) {
  if (item.quantity <= 1) {
    try {
      await showConfirmDialog({ title: "删除商品", message: `确定删除「${item.product.name}」吗？` });
      remove([item.product.id]);
    } catch { /* 用户取消 */ }
    return;
  }
  decrease(item.product.id);
}
const submitting = ref(false);

const defaultAddress = computed(() => addresses.value.find(a => a.isDefault) || addresses.value[0] || null);

async function loadAddresses() {
  try {
    const res = await http.get("/api/addresses");
    addresses.value = res.data;
  } catch { /* 忽略 */ }
}

async function submitOrder() {
  const addr = defaultAddress.value;
  if (!cartItems.value.length) { showToast("购物车为空"); return; }
  if (!addr) { showToast("请先添加收货地址"); router.push("/address/edit"); return; }
  submitting.value = true;
  try {
    const items = cartItems.value.map(it => ({
      productId: it.product.id,
      name: it.product.name,
      price: it.product.price,
      quantity: it.quantity,
      emoji: it.product.emoji,
      color: it.product.color,
    }));
    await http.post("/api/orders", { addressId: addr.id, items });
    clear();
    showToast("下单成功");
    router.replace("/orders");
  } catch (err) {
    const e = err as any;
    if (e?.response) showToast(e.response.data?.message || "下单失败");
    else showToast("下单服务暂不可用");
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
        <van-cell v-for="item in cartItems" :key="item.product.id" :title="item.product.name">
          <template #icon><div class="cart-thumb" :style="{ background: item.product.color }">{{ item.product.emoji }}</div></template>
          <template #label>单价 ¥{{ item.product.price.toFixed(2) }}</template>
          <template #value>
            <div class="checkout-line">
              <span>¥{{ (item.product.price * item.quantity).toFixed(2) }}</span>
              <van-stepper :model-value="item.quantity" min="1" @minus="onMinus(item)" @plus="increase(item.product.id)" />
            </div>
          </template>
        </van-cell>
      </van-cell-group>
      <div class="checkout-total"><span>合计</span><b>¥{{ total.toFixed(2) }}</b></div>
      <div class="checkout-submit">
        <van-button block round type="danger" :loading="submitting" @click="submitOrder">提交订单</van-button>
      </div>
    </template>
  </div>
</template>
