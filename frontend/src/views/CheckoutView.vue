<script setup lang="ts">
import { computed, onActivated, onMounted, ref } from "vue";
import { useRouter } from "vue-router";
import { storeToRefs } from "pinia";
import { showConfirmDialog, showToast } from "vant";
import { useShopCart, type ShopProduct } from "../stores/shop";
import http from "../lib/http";

interface Address { id: number; name: string; phone: string; region: string; detail: string; isDefault: boolean; }
interface Coupon { id: number; title: string; amount: number; minAmount: number; condition: string; startAt: string; endAt: string; }
interface VantCoupon { id: number; value: number; denominations?: number; name: string; condition: string; description: string; startAt: number; endAt: number; unitDesc: string; reason?: string; }

function toVantCoupon(c: Coupon): VantCoupon {
  return {
    id: c.id,
    value: c.amount * 100,
    denominations: c.amount * 100,
    name: c.title,
    condition: c.condition,
    description: "",
    startAt: Math.floor(new Date(c.startAt).getTime() / 1000),
    endAt: Math.floor(new Date(c.endAt).getTime() / 1000),
    unitDesc: "元",
  };
}

const router = useRouter();
const shopCart = useShopCart();
const { cartItems, total } = storeToRefs(shopCart);
const { clear, increase, decrease, remove } = shopCart;

const addresses = ref<Address[]>([]);
const selectedAddressId = ref<number | null>(null);
const coupons = ref<Coupon[]>([]);
const showCouponPopup = ref(false);
const chosenCoupon = ref(-1);

const availableCoupons = computed(() => coupons.value.filter(c => total.value >= c.minAmount).map(toVantCoupon));
const disabledCoupons = computed(() => coupons.value.filter(c => total.value < c.minAmount).map(c => ({ ...toVantCoupon(c), reason: "未达使用门槛" })));

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

const selectedAddress = computed(() => {
  const bySelected = addresses.value.find(a => a.id === selectedAddressId.value);
  if (bySelected) return bySelected;
  return addresses.value.find(a => a.isDefault) || addresses.value[0] || null;
});

const selectedCoupon = computed(() => chosenCoupon.value >= 0 ? availableCoupons.value[chosenCoupon.value] || null : null);

const discount = computed(() => selectedCoupon.value ? Math.min(selectedCoupon.value.value / 100, total.value) : 0);

const payAmount = computed(() => Math.max(0, total.value - discount.value));

async function loadAddresses() {
  try {
    const res = await http.get("/api/addresses");
    addresses.value = res.data;
    selectedAddressId.value = Number(localStorage.getItem("shop_checkout_address_id")) || null;
  } catch { /* 忽略 */ }
}

async function loadCoupons() {
  try {
    const res = await http.get("/api/coupons");
    coupons.value = res.data;
  } catch { /* 忽略 */ }
}

function onCouponChange(index: number) {
  chosenCoupon.value = index;
  showCouponPopup.value = false;
}

async function submitOrder() {
  const addr = selectedAddress.value;
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
    await http.post("/api/orders", { addressId: addr.id, couponId: selectedCoupon.value ? selectedCoupon.value.id : 0, items });
    clear();
    localStorage.removeItem("shop_checkout_address_id");
    selectedAddressId.value = null;
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

onMounted(() => {
  loadAddresses();
  loadCoupons();
});
onActivated(() => {
  loadAddresses();
  loadCoupons();
});
</script>

<template>
  <div class="sub-page">
    <van-nav-bar title="确认订单" left-arrow @click-left="$router.back()" />
    <van-empty v-if="!cartItems.length" description="购物车为空" />
    <template v-else>
      <van-cell-group inset class="checkout-address">
        <van-cell is-link @click="router.push('/address?select=1')">
          <template #title>
            <div v-if="selectedAddress">
              <div class="address-title"><strong>{{ selectedAddress.name }}</strong><span>{{ selectedAddress.phone }}</span></div>
              <div class="address-detail">{{ selectedAddress.region }} {{ selectedAddress.detail }}</div>
            </div>
            <div v-else class="checkout-no-addr">请选择收货地址</div>
          </template>
        </van-cell>
      </van-cell-group>
      <van-cell-group inset class="checkout-coupon">
        <van-coupon-cell title="优惠券" :coupons="availableCoupons" :chosen-coupon="chosenCoupon" @click="showCouponPopup = true" />
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
      <div class="checkout-total">
        <div class="checkout-total-row"><span>商品合计</span><span>¥{{ total.toFixed(2) }}</span></div>
        <div v-if="discount > 0" class="checkout-total-row"><span>优惠</span><span class="checkout-discount">-¥{{ discount.toFixed(2) }}</span></div>
        <div class="checkout-total-row"><span>实付</span><b>¥{{ payAmount.toFixed(2) }}</b></div>
      </div>
      <div class="checkout-submit">
        <van-button block round type="danger" :loading="submitting" @click="submitOrder">提交订单</van-button>
      </div>
    </template>
    <van-popup v-model:show="showCouponPopup" position="bottom" round>
      <div class="coupon-list-popup">
        <van-coupon-list
          :coupons="availableCoupons"
          :disabled-coupons="disabledCoupons"
          :chosen-coupon="chosenCoupon"
          :show-exchange-bar="false"
          close-button-text="不使用优惠券"
          @change="onCouponChange"
        />
      </div>
    </van-popup>
  </div>
</template>
