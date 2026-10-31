<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { useRouter } from "vue-router";
import { storeToRefs } from "pinia";
import { showToast } from "vant";
import { useShopCart, cartKey } from "../stores/shop";
import CartQuantity from "../components/CartQuantity.vue";
import http from "../lib/http";

interface Address { id: number; name: string; phone: string; region: string; detail: string; isDefault: boolean; }
interface Coupon { id: number; couponId: number; status: string; title: string; amount: number; minAmount: number; condition: string; startAt: string; endAt: string; }
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
const { checkedItems, checkedTotal, locked, submitting, syncError, pendingCheckout } = storeToRefs(shopCart);

const addresses = ref<Address[]>([]);
const selectedAddressId = ref<number | null>(null);
const coupons = ref<Coupon[]>([]);
const showCouponPopup = ref(false);
const chosenCouponId = ref<number | null>(null);

const usable = (c: Coupon) => checkedTotal.value >= c.minAmount && Date.now() >= Date.parse(c.startAt) && Date.now() <= Date.parse(c.endAt);
const availableCoupons = computed(() => coupons.value.filter(usable).map(toVantCoupon));
const disabledCoupons = computed(() => coupons.value.filter(c => !usable(c)).map(c => ({ ...toVantCoupon(c), reason: checkedTotal.value < c.minAmount ? "未达使用门槛" : "不在有效期" })));
const chosenCoupon = computed(() => availableCoupons.value.findIndex(c => c.id === chosenCouponId.value));

const selectedAddress = computed(() => {
  const bySelected = addresses.value.find(a => a.id === (pendingCheckout.value?.addressId ?? selectedAddressId.value));
  if (bySelected) return bySelected;
  return addresses.value.find(a => a.isDefault) || addresses.value[0] || null;
});

const selectedCoupon = computed(() => chosenCoupon.value >= 0 ? availableCoupons.value[chosenCoupon.value] || null : null);

const discount = computed(() => selectedCoupon.value ? Math.min(selectedCoupon.value.value / 100, checkedTotal.value) : 0);

const payAmount = computed(() => pendingCheckout.value?.expectedAmount ?? Math.max(0, Math.round(checkedTotal.value * 100) - Math.round(discount.value * 100)) / 100);

async function loadAddresses() {
  try {
    const res = await http.get("/api/addresses");
    addresses.value = res.data;
    selectedAddressId.value = Number(localStorage.getItem(shopCart.addressStorageKey)) || null;
  } catch { showToast("收货地址加载失败，请重试"); }
}

async function loadCoupons() {
  try {
    const res = await http.get("/api/coupons/mine");
    coupons.value = res.data.filter((c: Coupon) => c.status === "unused");
  } catch { showToast("优惠券加载失败，请重试"); }
}

function onCouponChange(index: number) {
  if (locked.value) return;
  chosenCouponId.value = availableCoupons.value[index]?.id ?? null;
  showCouponPopup.value = false;
}

async function submitOrder() {
  if (submitting.value || shopCart.loading) return;
  const addr = selectedAddress.value;
  if (!pendingCheckout.value && !checkedItems.value.length) { showToast("请选择要结算的商品"); return; }
  if (!pendingCheckout.value && !addr) { showToast("请先添加收货地址"); router.push("/address/edit"); return; }
  try {
    if (!pendingCheckout.value) {
      const signature = () => JSON.stringify(checkedItems.value.map(it => [it.product.id, it.skuId, it.quantity, it.product.price]));
      const before = signature();
      if (!await shopCart.refresh()) { showToast(shopCart.syncError || "商品信息刷新失败"); return; }
      if (signature() !== before) { showToast("商品价格、库存或数量已更新，请确认后重新提交"); return; }
    }
    const order = await shopCart.checkout(addr?.id ?? 0, selectedCoupon.value?.id ?? 0, payAmount.value);
    localStorage.removeItem(shopCart.addressStorageKey);
    selectedAddressId.value = null;
    showToast("下单成功");
    router.replace("/orders/" + order.id);
  } catch (err) {
    const e = err as any;
    showToast(pendingCheckout.value ? "订单结果待确认，请点击重试确认订单" : e?.response?.data?.message || e?.message || "下单失败");
    if (!pendingCheckout.value) { void shopCart.refresh(); void loadCoupons(); }
  }
}

onMounted(() => {
  loadAddresses();
  loadCoupons();
  void shopCart.refresh();
});
</script>

<template>
  <div class="sub-page">
    <van-nav-bar title="确认订单" left-arrow @click-left="$router.back()" />
    <van-notice-bar v-if="pendingCheckout" text="上次订单结果待确认，重试将继续确认同一笔订单" />
    <van-notice-bar v-else-if="syncError" :text="syncError" right-icon="replay" @click="shopCart.refresh" />
    <van-empty v-if="!checkedItems.length && !pendingCheckout" description="请选择要结算的商品" />
    <template v-else>
      <van-cell-group inset class="checkout-address">
        <van-cell :is-link="!locked" @click="!locked && router.push('/address?select=1')">
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
        <van-coupon-cell title="优惠券" :coupons="availableCoupons" :chosen-coupon="chosenCoupon" @click="!locked && (showCouponPopup = true)" />
      </van-cell-group>
      <van-cell-group inset class="checkout-items">
        <van-cell v-for="item in checkedItems" :key="cartKey(item.product.id, item.skuId)" :title="item.product.name">
          <template #icon><div class="cart-thumb" :style="{ background: item.product.color }">{{ item.product.emoji }}</div></template>
          <template #label>{{ item.specs || ("单价 ¥" + item.product.price.toFixed(2)) }}{{ item.priceChanged ? ' · 价格已更新' : '' }}</template>
          <template #value>
            <div class="checkout-line">
              <span>¥{{ (item.product.price * item.quantity).toFixed(2) }}</span>
              <CartQuantity :item="item" />
            </div>
          </template>
        </van-cell>
      </van-cell-group>
      <div class="checkout-total">
        <div class="checkout-total-row"><span>商品合计</span><span>¥{{ checkedTotal.toFixed(2) }}</span></div>
        <div v-if="discount > 0" class="checkout-total-row"><span>优惠</span><span class="checkout-discount">-¥{{ discount.toFixed(2) }}</span></div>
        <div class="checkout-total-row"><span>实付</span><b>¥{{ payAmount.toFixed(2) }}</b></div>
      </div>
      <div class="checkout-submit">
        <van-button block round type="danger" :loading="submitting || shopCart.loading" @click="submitOrder">{{ pendingCheckout ? '重试确认订单' : '提交订单' }}</van-button>
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
