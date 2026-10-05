<script setup lang="ts">
import { storeToRefs } from "pinia";
import { useShopCart } from "../stores/shop";
import { useNotificationStore } from "../stores/notification";
const shopCart = useShopCart();
const { cartItems } = storeToRefs(shopCart);
const notificationStore = useNotificationStore();
</script>

<template>
  <van-tabbar route>
    <van-tabbar-item icon="home-o" to="/">首页</van-tabbar-item>
    <van-tabbar-item icon="apps-o" to="/category" badge="热">分类</van-tabbar-item>
    <van-tabbar-item icon="shopping-cart-o" to="/cart" :badge="cartItems.length || undefined">购物车</van-tabbar-item>
    <van-tabbar-item icon="user-o" to="/profile" :dot="notificationStore.unread > 0">我的</van-tabbar-item>
  </van-tabbar>
</template>

