<script setup lang="ts">
import { useShopCart } from "../stores/shop";
const { count } = useShopCart();
</script>

<template>
  <van-tabbar route>
    <van-tabbar-item icon="home-o" to="/">首页</van-tabbar-item>
    <van-tabbar-item icon="apps-o" to="/category" badge="热">分类</van-tabbar-item>
    <van-tabbar-item icon="shopping-cart-o" to="/cart" :badge="count || undefined">购物车</van-tabbar-item>
    <van-tabbar-item icon="user-o" to="/profile" dot>我的</van-tabbar-item>
  </van-tabbar>
</template>

