<script setup lang="ts">
import { ref } from "vue";
import { useRouter } from "vue-router";
import { storeToRefs } from "pinia";
import { showConfirmDialog, showToast } from "vant";
import { useShopCart, type ShopProduct } from "../stores/shop";

const router = useRouter();
const shopCart = useShopCart();
const { cartItems, checkedCount, checkedTotal, allChecked } = storeToRefs(shopCart);
const { increase, decrease, toggleChecked, setAllChecked, remove, removeChecked } = shopCart;
const editing = ref(false);

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

function checkout() {
  if (!checkedCount.value) return;
  router.push("/checkout");
}

function toggleEdit() {
  editing.value = !editing.value;
}

function toggleAll() {
  setAllChecked(!allChecked.value);
}

function removeCheckedItems() {
  if (!checkedCount.value) return;
  removeChecked();
  editing.value = false;
  showToast("已删除");
}
</script>

<template>
  <div class="sub-page">
    <van-nav-bar title="购物车">
      <template #right><span class="cart-edit" @click="toggleEdit">{{ editing ? '完成' : '编辑' }}</span></template>
    </van-nav-bar>
    <van-empty v-if="!cartItems.length" image="https://fastly.jsdelivr.net/npm/@vant/assets/custom-empty-image.png" description="购物车还是空的" />
    <template v-else>
      <van-cell-group inset class="cart-list">
        <van-cell v-for="item in cartItems" :key="item.product.id">
          <template #icon>
            <div class="cart-row">
              <van-checkbox :model-value="item.checked" @click="toggleChecked(item.product.id)" />
              <div class="cart-thumb" :style="{ background: item.product.color }">{{ item.product.emoji }}</div>
            </div>
          </template>
          <template #title><strong>{{ item.product.name }}</strong><div class="cart-desc">{{ item.product.description }}</div></template>
          <template #value><div class="cart-price">¥{{ item.product.price.toFixed(2) }}<van-stepper :model-value="item.quantity" min="1" @minus="onMinus(item)" @plus="increase(item.product.id)" /></div></template>
        </van-cell>
      </van-cell-group>
      <div class="cart-submit">
        <van-checkbox :model-value="allChecked" @click="toggleAll">全选</van-checkbox>
        <template v-if="editing">
          <van-button round type="danger" :disabled="!checkedCount" @click="removeCheckedItems">删除选中</van-button>
        </template>
        <template v-else>
          <span>合计 <b>¥{{ checkedTotal.toFixed(2) }}</b></span>
          <van-button round type="danger" :disabled="!checkedCount" @click="checkout">去结算 ({{ checkedCount }})</van-button>
        </template>
      </div>
    </template>
  </div>
</template>
