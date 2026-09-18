<script setup lang="ts">
import { ref } from "vue";
import { useRouter } from "vue-router";
import { showToast } from "vant";
import { useShopCart } from "../stores/shop";

const router = useRouter();
const { cartItems, count, total, decrease, remove } = useShopCart();
const editing = ref(false);
const selected = ref<number[]>([]);

function checkout() {
  if (!count.value) return;
  router.push("/checkout");
}

function toggleEdit() {
  editing.value = !editing.value;
  selected.value = [];
}

function toggleSelect(id: number) {
  const idx = selected.value.indexOf(id);
  if (idx >= 0) selected.value.splice(idx, 1);
  else selected.value.push(id);
}

function toggleAll() {
  if (selected.value.length === cartItems.value.length) selected.value = [];
  else selected.value = cartItems.value.map(it => it.product.id);
}

function removeSelected() {
  if (!selected.value.length) return;
  remove(selected.value);
  selected.value = [];
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
              <van-checkbox v-if="editing" :model-value="selected.includes(item.product.id)" @click="toggleSelect(item.product.id)" />
              <div class="cart-thumb" :style="{ background: item.product.color }">{{ item.product.emoji }}</div>
            </div>
          </template>
          <template #title><strong>{{ item.product.name }}</strong><div class="cart-desc">{{ item.product.description }}</div></template>
          <template #value><div class="cart-price">¥{{ item.product.price.toFixed(2) }}<van-stepper :model-value="item.quantity" min="0" @minus="decrease(item.product.id)" /></div></template>
        </van-cell>
      </van-cell-group>
      <div class="cart-submit">
        <template v-if="editing">
          <van-checkbox :model-value="cartItems.length > 0 && selected.length === cartItems.length" @click="toggleAll">全选</van-checkbox>
          <van-button round type="danger" :disabled="!selected.length" @click="removeSelected">删除选中</van-button>
        </template>
        <template v-else>
          <span>合计 <b>¥{{ total.toFixed(2) }}</b></span>
          <van-button round type="danger" @click="checkout">去结算 ({{ count }})</van-button>
        </template>
      </div>
    </template>
  </div>
</template>
