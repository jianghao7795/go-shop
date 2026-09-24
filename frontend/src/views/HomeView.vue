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
      ...list.map((c) => ({ label: c.name, value: c.key, icon: c.icon })),
    ];
  } catch {
    /* 分类加载失败，仅保留「推荐」 */
  }
}
const activeCategory = ref("all");
const search = ref("");
const router = useRouter();
const userStore = useUserStore();
const { add: addToCartItem } = useShopCart();
const loading = ref(false);
const loadError = ref("");
const products = ref<Product[]>([]);
const featured = ref<Product[]>([]);

async function loadProducts() {
  loading.value = true;
  loadError.value = "";
  try {
    const params = new URLSearchParams();
    if (search.value.trim()) params.set("search", search.value.trim());
    if (activeCategory.value !== "all")
      params.set("category", activeCategory.value);
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

async function loadFeatured() {
  try {
    const res = await http.get("/api/products");
    const data = res.data as Product[];
    featured.value = [...data].sort((a, b) => b.sales - a.sales).slice(0, 6);
  } catch {
    /* 优选好物加载失败时保持为空 */
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
  loadFeatured();
});
</script>

<template>
  <div class="shop-page">
    <van-nav-bar
      title="优选商城"
      left-text="城市生活"
      :right-text="userStore.isLoggedIn ? userStore.username : '登录'"
      @click-right="onRightClick"
    />
    <van-search v-model="search" shape="round" placeholder="搜索商品、品牌" />
    <main class="shop-content">
      <van-swipe
        class="hero"
        :autoplay="3500"
        indicator-color="white"
        lazy-render
      >
        <van-swipe-item
          v-for="(banner, index) in [
            '今日特惠 · 满199减30',
            '春日焕新 · 新品低至5折',
            '品质生活 · 会员专享价',
          ]"
          :key="banner"
        >
          <div class="hero-card" :class="`hero-${index + 1}`">
            <span class="hero-kicker">优选好物</span
            ><strong>{{ banner }}</strong
            ><small>立即抢购 ></small>
          </div>
        </van-swipe-item>
      </van-swipe>

      <van-grid :column-num="5" :border="false" class="category-grid">
        <van-grid-item
          v-for="category in categories"
          :key="category.value"
          :text="category.label"
          @click="selectCategory(category.value)"
        >
          <template #icon
            ><div
              class="category-icon"
              :class="{ active: activeCategory === category.value }"
            >
              {{ category.icon }}
            </div></template
          >
        </van-grid-item>
      </van-grid>

      <div class="section-heading">
        <h2>优选好物</h2>
        <span>销量精选</span>
      </div>
      <div class="featured-scroll">
        <article
          v-for="p in featured"
          :key="p.id"
          class="featured-card"
          @click="$router.push(`/product/${p.id}`)"
        >
          <div class="featured-thumb" :style="{ background: p.color }">
            {{ p.emoji }}
          </div>
          <h3>{{ p.name }}</h3>
          <div class="featured-meta">
            <span class="featured-price">¥{{ p.price.toFixed(2) }}</span>
            <span class="featured-sales">已售 {{ p.sales }}</span>
          </div>
        </article>
      </div>

      <div class="section-heading">
        <h2>猜你喜欢</h2>
        <span>实时更新</span>
      </div>
      <van-tabs v-model:active="activeCategory" shrink swipeable>
        <van-tab
          v-for="category in categories"
          :key="category.value"
          :title="category.label"
          :name="category.value"
        />
      </van-tabs>
      <div v-if="loading" class="loading"><van-loading color="#ff4d67" /></div>
      <van-empty v-else-if="loadError" :description="loadError" />
      <van-empty v-else-if="!products.length" description="没有找到相关商品" />
      <div v-else class="product-grid">
        <article
          v-for="product in products"
          :key="product.id"
          class="product-card"
          @click="$router.push(`/product/${product.id}`)"
        >
          <div class="product-thumb" :style="{ background: product.color }">
            {{ product.emoji }}
          </div>
          <div class="product-info">
            <h3>{{ product.name }}</h3>
            <p class="product-description">{{ product.description }}</p>
            <div class="product-tags">
              <span>自营</span><span>极速发货</span>
            </div>
            <div class="product-meta">
              <span class="product-price">¥{{ product.price.toFixed(2) }}</span
              ><del>¥{{ product.originalPrice.toFixed(2) }}</del>
            </div>
            <div class="product-actions">
              <span class="sales">已售 {{ product.sales }}</span
              ><van-button
                round
                size="small"
                type="danger"
                @click.stop="addToCart(product)"
                >加入购物车</van-button
              >
            </div>
          </div>
        </article>
      </div>
    </main>
  </div>
</template>

<style scoped>
.featured-scroll {
  display: flex;
  gap: 10px;
  padding: 10px 12px;
  overflow-x: auto;
  -webkit-overflow-scrolling: touch;
}
.featured-scroll::-webkit-scrollbar {
  display: none;
}
.featured-card {
  flex: 0 0 132px;
  overflow: hidden;
  border-radius: 12px;
  background: #fff;
  box-shadow: 0 2px 10px rgba(28, 35, 50, 0.06);
}
.featured-thumb {
  height: 92px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 40px;
  line-height: 1;
}
.featured-card h3 {
  margin: 0;
  padding: 8px 10px 0;
  font-size: 13px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.featured-meta {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  padding: 4px 10px 10px;
}
.featured-price {
  color: #f43f5e;
  font-weight: 700;
  font-size: 14px;
}
.featured-sales {
  color: #969ba5;
  font-size: 11px;
}
</style>
