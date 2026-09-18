<script setup lang="ts">
import { onMounted, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { showToast } from "vant";
import { useShopCart, type ShopProduct } from "../stores/shop";

const route = useRoute();
const router = useRouter();
const { add, count: cartCount } = useShopCart();
const product = ref<ShopProduct | null>(null);
const quantity = ref(1);
const loading = ref(true);
const error = ref("");

async function loadProduct() {
  loading.value = true;
  try {
    const apiBase = import.meta.env.VITE_API_BASE || "http://127.0.0.1:8080";
    const response = await fetch(apiBase + "/api/products/" + route.params.id);
    if (!response.ok) throw new Error("商品不存在");
    product.value = await response.json() as ShopProduct;
  } catch {
    error.value = "商品信息加载失败，请稍后重试";
  } finally {
    loading.value = false;
  }
}

function addToCart() {
  if (!product.value) return;
  for (let i = 0; i < quantity.value; i += 1) add(product.value);
  showToast("已加入购物车");
}

function buyNow() {
  if (!product.value) return;
  addToCart();
  router.push("/cart");
}

onMounted(loadProduct);
</script>

<template>
  <div class="detail-page">
    <van-nav-bar :title="product?.name || '商品详情'" left-arrow @click-left="$router.back()" />
    <div v-if="loading" class="detail-loading"><van-loading color="#ee0a24" /></div>
    <van-empty v-else-if="error" :description="error" />
    <template v-else-if="product">
      <div class="detail-image" :style="{ background: product.color }">{{ product.emoji }}</div>
      <section class="detail-main">
        <div class="detail-price"><span>¥</span>{{ product.price.toFixed(2) }} <del>¥{{ product.originalPrice.toFixed(2) }}</del></div>
        <h1>{{ product.name }}</h1>
        <p class="detail-description">{{ product.description }}</p>
        <div class="detail-tags"><span>自营</span><span>极速发货</span><span>7天无理由</span></div>
        <van-cell-group inset class="detail-info">
          <van-cell title="已售" :value="product.sales + ' 件'" />
          <van-cell title="配送" value="预计明日送达" is-link />
          <van-cell title="服务" value="正品保障 · 破损包退" is-link />
        </van-cell-group>
        <van-cell-group inset class="detail-quantity">
          <van-cell title="购买数量"><template #value><van-stepper v-model="quantity" min="1" /></template></van-cell>
        </van-cell-group>
      </section>
      <van-action-bar>
        <van-action-bar-icon icon="service-o" text="客服" @click="showToast('客服暂未开通')" />
        <van-action-bar-icon icon="cart-o" text="购物车" :badge="cartCount || undefined" to="/cart" />
        <van-action-bar-icon icon="star-o" text="收藏" @click="showToast('已收藏')" />
        <van-action-bar-button type="warning" text="加入购物车" @click="addToCart" />
        <van-action-bar-button type="danger" text="立即购买" @click="buyNow" />
      </van-action-bar>
    </template>
  </div>
</template>

