<script setup lang="ts">
import { onMounted, ref, watch } from "vue";
import { showToast } from "vant";
import { useShopCart } from "../stores/shop";

interface Category {
  key: string;
  name: string;
  icon: string;
  note: string;
}

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

const apiBase = import.meta.env.VITE_API_BASE || "http://127.0.0.1:8080";
const categories = ref<Category[]>([]);
const active = ref(0);
const products = ref<Product[]>([]);
const loading = ref(false);
const { add: addToCartItem } = useShopCart();

async function loadCategories() {
  try {
    const response = await fetch(`${apiBase}/api/categories`);
    if (response.ok) {
      categories.value = await response.json();
      await loadProducts();
    }
  } catch {
    /* 分类加载失败，保持空列表 */
  }
}

async function loadProducts() {
  const cat = categories.value[active.value];
  if (!cat) return;
  loading.value = true;
  try {
    const response = await fetch(`${apiBase}/api/products?category=${encodeURIComponent(cat.key)}`);
    products.value = response.ok ? await response.json() : [];
  } catch {
    products.value = [];
  } finally {
    loading.value = false;
  }
}

function addToCart(product: Product) {
  addToCartItem(product);
  showToast(`${product.name} 已加入购物车`);
}

watch(active, loadProducts);
onMounted(loadCategories);
</script>

<template>
  <div class="sub-page">
    <van-nav-bar title="商品分类" />
    <div v-if="!categories.length" class="loading"><van-loading color="#ff4d67" /></div>
    <div v-else class="category-layout">
      <van-sidebar v-model="active">
        <van-sidebar-item v-for="item in categories" :key="item.key" :title="item.name" />
      </van-sidebar>
      <section class="category-detail">
        <h2>{{ categories[active].name }}</h2>
        <p>{{ categories[active].note }}</p>
        <div class="category-banner">{{ categories[active].icon }} <strong>精选好物</strong></div>
        <div v-if="loading" class="loading"><van-loading color="#ff4d67" /></div>
        <van-empty v-else-if="!products.length" description="该分类暂无商品" />
        <div v-else class="product-grid">
          <article v-for="product in products" :key="product.id" class="product-card" @click="$router.push(`/product/${product.id}`)">
            <div class="product-thumb" :style="{ background: product.color }">{{ product.emoji }}</div>
            <div class="product-info">
              <h3>{{ product.name }}</h3>
              <p class="product-description">{{ product.description }}</p>
              <div class="product-meta"><span class="product-price">¥{{ product.price.toFixed(2) }}</span><del>¥{{ product.originalPrice.toFixed(2) }}</del></div>
              <div class="product-actions"><span class="sales">已售 {{ product.sales }}</span><van-button round size="small" type="danger" @click.stop="addToCart(product)">加入购物车</van-button></div>
            </div>
          </article>
        </div>
      </section>
    </div>
  </div>
</template>
