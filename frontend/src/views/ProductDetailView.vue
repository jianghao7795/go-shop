<script setup lang="ts">
import { onMounted, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { storeToRefs } from "pinia";
import { Rate, showToast } from "vant";
import { useShopCart, type ShopProduct } from "../stores/shop";
import http from "../lib/http";

const route = useRoute();
const router = useRouter();
const shopCart = useShopCart();
const { cartItems } = storeToRefs(shopCart);
const { add } = shopCart;
const product = ref<ShopProduct | null>(null);
const quantity = ref(1);
const loading = ref(true);
const error = ref("");
const reviews = ref<any[]>([]);
const avg = ref(0);
const reviewCount = ref(0);

async function loadProduct() {
  loading.value = true;
  try {
    const res = await http.get("/api/products/" + route.params.id);
    product.value = res.data as ShopProduct;
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

async function loadReviews() {
  try {
    const res = await http.get("/api/products/" + route.params.id + "/reviews");
    reviews.value = res.data.reviews || [];
    avg.value = res.data.average || 0;
    reviewCount.value = res.data.count || 0;
  } catch {
    reviews.value = [];
  }
}

onMounted(() => {
  loadProduct();
  loadReviews();
});
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
      <section class="detail-reviews">
        <div class="reviews-head">
          <h3>商品评价</h3>
          <span>{{ reviewCount }} 条 · 均分 {{ avg.toFixed(1) }}</span>
        </div>
        <van-empty v-if="!reviews.length" description="暂无评价" />
        <div v-for="r in reviews" :key="r.id" class="review-item">
          <div class="review-meta">
            <span class="review-user">{{ r.userId }}</span>
            <Rate :model-value="r.rating" readonly size="14" color="#ff4d67" />
          </div>
          <p class="review-content">{{ r.content || "此用户没有填写文字评价" }}</p>
        </div>
      </section>

      <van-action-bar>
        <van-action-bar-icon icon="service-o" text="客服" @click="showToast('客服暂未开通')" />
        <van-action-bar-icon icon="cart-o" text="购物车" :badge="cartItems.length || undefined" to="/cart" />
        <van-action-bar-icon icon="star-o" text="收藏" @click="showToast('已收藏')" />
        <van-action-bar-button type="warning" text="加入购物车" @click="addToCart" />
        <van-action-bar-button type="danger" text="立即购买" @click="buyNow" />
      </van-action-bar>
    </template>
  </div>
</template>

<style scoped>
.detail-reviews {
  margin: 12px;
  padding: 12px;
  background: #fff;
  border-radius: 10px;
}
.reviews-head {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  margin-bottom: 8px;
}
.reviews-head h3 {
  margin: 0;
  font-size: 16px;
}
.reviews-head span {
  font-size: 12px;
  color: #969ba5;
}
.review-item {
  padding: 10px 0;
  border-bottom: 1px solid #f0f1f4;
}
.review-item:last-child {
  border-bottom: none;
}
.review-meta {
  display: flex;
  align-items: center;
  justify-content: space-between;
}
.review-user {
  font-size: 13px;
  color: #646a73;
}
.review-content {
  margin: 6px 0 0;
  font-size: 14px;
  color: #1f2430;
}
</style>

