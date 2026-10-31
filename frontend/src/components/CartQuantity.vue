<script setup lang="ts">
import { cartKey, MAX_CART_QUANTITY, useShopCart, type CartLine } from "../stores/shop";
const props = defineProps<{ item: CartLine; disabled?: boolean }>();
const cart = useShopCart();
function change(value: number | string) {
  return cart.setQuantity(cartKey(props.item.product.id, props.item.skuId), value);
}
</script>

<template>
  <van-stepper :model-value="item.quantity" integer :min="1" :max="MAX_CART_QUANTITY"
    :disable-plus="item.quantity >= item.stock"
    :disabled="disabled || cart.locked || item.stock <= 0 || (!!item.reason && item.reason !== '库存不足，请调整数量')"
    :before-change="change" />
</template>
