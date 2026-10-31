<script setup lang="ts">
import { computed, onMounted, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { storeToRefs } from "pinia";
import { Rate, showToast } from "vant";
import { useShopCart, type ShopProduct } from "../stores/shop";
import http from "../lib/http";
import { resolveDetailHtml } from "../lib/avatar";

const route = useRoute();
const router = useRouter();
const shopCart = useShopCart();
const { cartItems } = storeToRefs(shopCart);
const { add } = shopCart;
const product = ref<ShopProduct | null>(null);
const quantity = ref(1);
const selectedSkuId = ref(0);
const selectedSku = computed(() => product.value?.skus?.find(s => s.id === selectedSkuId.value));
const currentStock = computed(() => selectedSku.value?.stock ?? (product.value?.skus?.length ? 0 : product.value?.stock ?? 0));
watch(currentStock, stock => { quantity.value = Math.max(1, Math.min(quantity.value, stock)); });

const currentPrice = computed(() => {
  if (!product.value) return 0;
  const sku = product.value.skus?.find((s) => s.id === selectedSkuId.value);
  return sku ? sku.price : product.value.price;
});
const loading = ref(true);
const error = ref("");
const reviews = ref<any[]>([]);
const avg = ref(0);
const reviewCount = ref(0);
const reviewsActive = ref<string[]>([]);

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

function addToCart(): boolean {
  if (!product.value) return false;
  if (product.value.skus?.length && !selectedSku.value) { showToast("请先选择商品规格"); return false; }
  if (quantity.value > currentStock.value || currentStock.value <= 0) { showToast("当前规格库存不足"); return false; }
  const sku = product.value.skus?.find((s) => s.id === selectedSkuId.value);
  const price = sku ? sku.price : product.value.price;
  return add(product.value, selectedSkuId.value, sku?.specs || "", price, quantity.value);
}

function buyNow() {
  if (!product.value) return;
  if (!addToCart()) return;
  shopCart.setAllChecked(false);
  shopCart.toggleChecked(product.value.id + ":" + selectedSkuId.value);
  router.push("/checkout");
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
        <div class="detail-price"><span>¥</span>{{ currentPrice.toFixed(2) }} <del>¥{{ product.originalPrice.toFixed(2) }}</del></div>
        <h1>{{ product.name }}</h1>
        <p class="detail-description">{{ product.description }}</p>
        <div class="detail-tags"><span>自营</span><span>极速发货</span><span>7天无理由</span></div>
        <van-cell-group inset class="detail-info">
          <van-cell title="已售" :value="product.sales + ' 件'" />
          <van-cell title="配送" value="发货后可在订单中查看物流" />
          <van-cell title="服务" value="联系商家咨询售后" is-link to="/service" />
        </van-cell-group>
        <van-cell-group v-if="product.skus && product.skus.length" inset class="detail-sku">
          <van-cell title="规格">
            <template #value>
              <div class="sku-options">
                <button
                  v-for="sku in product.skus"
                  :key="sku.id"
                  class="sku-option"
                  :class="{ active: selectedSkuId === sku.id }"
                  :disabled="sku.stock <= 0"
                  @click="selectedSkuId = sku.id"
                >{{ sku.specs }}{{ sku.stock <= 0 ? '（售罄）' : '' }}</button>
              </div>
            </template>
          </van-cell>
        </van-cell-group>
        <van-cell-group inset class="detail-quantity">
          <van-cell title="库存" :value="product.skus?.length && !selectedSku ? '请选择规格' : currentStock + ' 件'" />
          <van-cell title="购买数量"><template #value><van-stepper v-model="quantity" integer min="1" :max="Math.max(1, currentStock)" :disabled="currentStock <= 0" /></template></van-cell>
        </van-cell-group>
      </section>
      <section class="detail-reviews">
        <van-collapse v-model="reviewsActive">
          <van-collapse-item name="reviews">
            <template #title>
              <div class="reviews-title">
                <span class="reviews-title-name">商品评价</span>
                <span class="reviews-title-meta">{{ reviewCount }} 条 · 均分 {{ avg.toFixed(1) }}</span>
              </div>
            </template>
            <van-empty v-if="!reviews.length" description="暂无评价" />
            <div v-for="r in reviews" :key="r.id" class="review-item">
              <div class="review-meta">
                <span class="review-user">{{ r.userId }}</span>
                <Rate :model-value="r.rating" readonly allow-half size="14" color="#ff4d67" />
              </div>
              <p class="review-content">{{ r.content || "此用户没有填写文字评价" }}</p>
            </div>
          </van-collapse-item>
        </van-collapse>
      </section>
      <section v-if="product.detail" class="detail-rich">
        <h3>商品详情</h3>
        <div class="detail-rich-content" v-html="resolveDetailHtml(product.detail || '')"></div>
      </section>

      <van-action-bar>
        <van-action-bar-icon icon="service-o" text="客服" to="/service" />
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
  background: #fff;
  border-radius: 10px;
  overflow: hidden;
}
.reviews-title {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: space-between;
}
.reviews-title-name {
  font-size: 16px;
  font-weight: 600;
  color: #1f2430;
}
.reviews-title-meta {
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
.detail-rich { margin: 12px; padding: 12px; background: #fff; border-radius: 10px; }
.detail-sku { margin: 12px 0; }
.sku-options { display: flex; flex-wrap: wrap; gap: 8px; justify-content: flex-end; }
.sku-option { padding: 4px 12px; border-radius: 14px; background: #f5f6f8; color: #646a73; font-size: 13px; }
.sku-option.active { color: #ee0a24; background: #ffecef; font-weight: 600; }
.detail-rich h3 { margin: 0 0 8px; font-size: 16px; }
.detail-rich-content { font-size: 14px; color: #1f2430; line-height: 1.7; }
.detail-rich-content :deep(img) { max-width: 100%; height: auto; display: block; }
.detail-rich-content :deep(table) { border-collapse: collapse; width: 100%; }
.detail-rich-content :deep(td), .detail-rich-content :deep(th) { border: 1px solid #ebedf0; padding: 6px; }
</style>

