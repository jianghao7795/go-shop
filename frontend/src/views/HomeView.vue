<script setup lang="ts">
import { onMounted, ref, watch } from "vue";
import { useRouter } from "vue-router";
import { showToast } from "vant";
import { useShopCart } from "../stores/shop";
import { useUserStore } from "../stores/user";
import http from "../lib/http";

interface Product {
  id: number;
  name: string;
  description: string;
  price: number;
  originalPrice: number;
  sales: number;
  emoji: string;
  color: string;
  category: string;
}

interface Category {
  key: string;
  name: string;
  icon: string;
}

const categories = ref([{ label: "推荐", value: "all", icon: "🔥" }]);

async function loadCategories() {
  try {
    const res = await http.get("/api/categories");
    const list = res.data as Category[];
    categories.value = [
      { label: "推荐", value: "all", icon: "🔥" },
      ...list.map(c => ({ label: c.name, value: c.key, icon: c.icon })),
    ];
  } catch {
    /* 分类加载失败，仅保留「推荐」 */
  }
}
const activeCategory = ref("all");
const search = ref("");
const router = useRouter();
const userStore = useUserStore();
const { count: cartCount, add: addToCartItem } = useShopCart();
const loading = ref(false);
const loadError = ref("");
const products = ref<Product[]>([]);

async function loadProducts() {
  loading.value = true;
  loadError.value = "";
  try {
    const params = new URLSearchParams();
    if (search.value.trim()) params.set("search", search.value.trim());
    if (activeCategory.value !== "all") params.set("category", activeCategory.value);
    const qs = params.toString();
    const res = await http.get("/api/products" + (qs ? "?" + qs : ""));
    const data = res.data as Product[];
    products.value = data;
    if (!data.length) loadError.value = "没有找到相关商品";
  } catch {
    loadError.value = "商品服务暂不可用，请确认 Gin 和 MySQL 已启动";
  } finally {
    loading.value = false;
  }
}

function addToCart(product: Product) {
  addToCartItem(product);
  showToast(`${product.name} 已加入购物车`);
}

function selectCategory(value: string) {
  activeCategory.value = value;
}

function onRightClick() {
  if (userStore.isLoggedIn) router.push("/profile");
  else router.push({ name: "login", query: { redirect: "/" } });
}

let timer: number | undefined;
watch(search, () => {
  if (timer) window.clearTimeout(timer);
  timer = window.setTimeout(loadProducts, 300);
});
watch(activeCategory, loadProducts);

onMounted(() => {
  loadCategories();
  loadProducts();
});
</script>

<template>
  <div class="shop-page">
    <van-nav-bar title="优选商城" left-text="城市生活" :right-text="userStore.isLoggedIn ? userStore.username : '登录'" @click-right="onRightClick" />
    <van-search v-model="search" shape="round" placeholder="搜索商品、品牌" />
    <main class="shop-content">
      <van-swipe class="hero" :autoplay="3500" indicator-color="white" lazy-render>
        <van-swipe-item v-for="(banner, index) in ['今日特惠 · 满199减30', '春日焕新 · 新品低至5折', '品质生活 · 会员专享价']" :key="banner">
          <div class="hero-card" :class="`hero-${index + 1}`"><span class="hero-kicker">优选好物</span><strong>{{ banner }}</strong><small>立即抢购 ></small></div>
        </van-swipe-item>
      </van-swipe>

      <van-grid :column-num="5" :border="false" class="category-grid">
        <van-grid-item v-for="category in categories" :key="category.value" :text="category.label" @click="selectCategory(category.value)">
          <template #icon><div class="category-icon" :class="{ active: activeCategory === category.value }">{{ category.icon }}</div></template>
        </van-grid-item>
      </van-grid>

      <div class="section-heading"><h2>猜你喜欢</h2><span>实时更新</span></div>
      <van-tabs v-model:active="activeCategory" shrink swipeable>
        <van-tab v-for="category in categories" :key="category.value" :title="category.label" :name="category.value" />
      </van-tabs>
      <div v-if="loading" class="loading"><van-loading color="#ff4d67" /></div>
      <van-empty v-else-if="loadError" :description="loadError" />
      <van-empty v-else-if="!products.length" description="没有找到相关商品" />
      <div v-else class="product-grid">
        <article v-for="product in products" :key="product.id" class="product-card" @click="$router.push(`/product/${product.id}`)">
          <div class="product-thumb" :style="{ background: product.color }">{{ product.emoji }}</div>
          <div class="product-info">
            <h3>{{ product.name }}</h3>
            <p class="product-description">{{ product.description }}</p><div class="product-tags"><span>自营</span><span>极速发货</span></div>
            <div class="product-meta"><span class="product-price">¥{{ product.price.toFixed(2) }}</span><del>¥{{ product.originalPrice.toFixed(2) }}</del></div>
            <div class="product-actions"><span class="sales">已售 {{ product.sales }}</span><van-button round size="small" type="danger" @click.stop="addToCart(product)">加入购物车</van-button></div>
          </div>
        </article>
      </div>
    </main>
  </div>
</template>
