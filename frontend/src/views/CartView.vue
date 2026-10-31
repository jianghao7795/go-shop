<script setup lang="ts">
import { onMounted, ref } from "vue";
import { useRouter } from "vue-router";
import { storeToRefs } from "pinia";
import { showConfirmDialog, showToast } from "vant";
import { useShopCart, cartKey, type CartLine } from "../stores/shop";
import CartQuantity from "../components/CartQuantity.vue";

const router = useRouter();
const shopCart = useShopCart();
const { cartItems, checkedCount, checkedTotal, allChecked, allSelected, selectedCount, locked, loading, syncing, syncError, pendingCheckout } = storeToRefs(shopCart);
const { toggleChecked, setAllChecked, remove, removeChecked, refresh } = shopCart;
const editing = ref(false);

async function removeItem(item: CartLine) {
  const key = cartKey(item.product.id, item.skuId);
  try {
    await showConfirmDialog({ title: "删除商品", message: `确定删除「${item.product.name}」吗？` });
    remove([key]);
  } catch { /* 用户取消 */ }
}

async function checkout() {
  if (pendingCheckout.value) { router.push("/checkout"); return; }
  if (locked.value || !checkedCount.value || !await refresh()) return;
  if (!checkedCount.value) { showToast("所选商品已失效，请重新选择"); return; }
  router.push("/checkout");
}

function toggleEdit() {
  if (locked.value) return;
  editing.value = !editing.value;
}

function toggleAll() {
  setAllChecked(!(editing.value ? allSelected.value : allChecked.value), editing.value);
}

async function removeCheckedItems() {
  if (!selectedCount.value || locked.value) return;
  try {
    await showConfirmDialog({ title: "删除商品", message: "确定删除选中的商品吗？" });
    removeChecked(); editing.value = false; showToast("已删除");
  } catch { /* 用户取消 */ }
}
onMounted(() => { void refresh(); });
</script>

<template>
  <div class="sub-page">
    <van-nav-bar title="购物车">
      <template #right><span class="cart-edit" @click="toggleEdit">{{ editing ? '完成' : '编辑' }}</span></template>
    </van-nav-bar>
    <van-notice-bar v-if="pendingCheckout" text="上次订单结果待确认，点击继续确认" @click="router.push('/checkout')" />
    <van-notice-bar v-else-if="syncError" :text="syncError" right-icon="replay" @click="refresh" />
    <van-notice-bar v-else-if="loading || syncing" :text="loading ? '正在更新商品信息…' : '正在同步购物车…'" />
    <van-empty v-if="!cartItems.length" image="https://fastly.jsdelivr.net/npm/@vant/assets/custom-empty-image.png" description="购物车还是空的" />
    <template v-else>
      <van-cell-group inset class="cart-list">
        <van-cell v-for="item in cartItems" :key="cartKey(item.product.id, item.skuId)">
          <template #icon>
            <div class="cart-row">
              <van-checkbox :model-value="item.checked" :disabled="locked || (!editing && !item.available)" @click="toggleChecked(cartKey(item.product.id, item.skuId), editing)" />
              <div class="cart-thumb" :style="{ background: item.product.color }">{{ item.product.emoji }}</div>
            </div>
          </template>
          <template #title>
            <strong>{{ item.product.name }}</strong><div class="cart-desc">{{ item.specs || item.product.description }}</div>
            <div v-if="item.reason" class="cart-status">{{ item.reason }}</div>
            <div v-else class="cart-desc">库存 {{ item.stock }} 件</div>
            <div v-if="item.priceChanged" class="cart-status">价格已更新</div>
          </template>
          <template #value>
            <div class="cart-price">¥{{ item.product.price.toFixed(2) }}<CartQuantity :item="item" /></div>
            <van-button size="mini" plain :disabled="locked" @click="removeItem(item)">删除</van-button>
          </template>
        </van-cell>
      </van-cell-group>
      <div class="cart-submit">
        <van-checkbox :model-value="editing ? allSelected : allChecked" :disabled="locked" @click="toggleAll">全选</van-checkbox>
        <template v-if="editing">
          <van-button round type="danger" :disabled="locked || !selectedCount" @click="removeCheckedItems">删除选中</van-button>
        </template>
        <template v-else>
          <span>合计 <b>¥{{ checkedTotal.toFixed(2) }}</b></span>
          <van-button round type="danger" :disabled="locked || !checkedCount" @click="checkout">去结算 ({{ checkedCount }})</van-button>
        </template>
      </div>
    </template>
  </div>
</template>

<style scoped>
.cart-status { margin-top: 5px; color: #ee0a24; font-size: 12px; }
.cart-price + .van-button { margin-top: 8px; }
</style>
