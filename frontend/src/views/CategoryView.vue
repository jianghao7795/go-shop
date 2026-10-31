<script setup lang="ts">
import { onMounted, onUnmounted, ref, watch } from "vue";
import { useRouter } from "vue-router";
import { showToast } from "vant";
import { useShopCart } from "../stores/shop";
import http from "../lib/http";

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

const categories = ref<Category[]>([]);
const router = useRouter();
const active = ref(0);
const products = ref<Product[]>([]);
const loading = ref(false);
const categoriesLoading = ref(true);
const categoryError = ref("");
const productError = ref("");
let productRequest = 0;
const { add: addToCartItem } = useShopCart();

async function loadCategories() {
  categoriesLoading.value = true;
  categoryError.value = "";
  try {
    const res = await http.get("/api/categories");
    categories.value = res.data || [];
    await loadProducts();
  } catch {
    categoryError.value = "分类加载失败，请稍后重试";
  } finally {
    categoriesLoading.value = false;
  }
}

async function loadProducts() {
  const request = ++productRequest;
  const cat = categories.value[active.value];
  if (!cat) { products.value = []; loading.value = false; return; }
  loading.value = true;
  productError.value = "";
  try {
    const res = await http.get("/api/products", { params: { category: cat.key } });
    if (request !== productRequest) return;
    products.value = res.data || [];
  } catch {
    if (request !== productRequest) return;
    products.value = [];
    productError.value = "商品加载失败，请稍后重试";
  } finally {
    if (request === productRequest) loading.value = false;
  }
}

async function addToCart(product: Product) {
  try {
    const { data } = await http.get("/api/products/" + product.id);
    if (data.skus?.length) { router.push("/product/" + product.id); showToast("请选择商品规格"); return; }
    if (data.stock <= 0) { showToast("商品已售罄"); return; }
    addToCartItem(data);
  } catch { showToast("读取商品失败，请稍后重试"); }
}

watch(active, loadProducts);
onMounted(loadCategories);
onUnmounted(() => { productRequest++; });
</script>

<template>
  <div class="sub-page">
    <van-nav-bar title="商品分类" />
    <div v-if="categoriesLoading" class="loading"><van-loading color="#ff4d67" /></div>
    <van-empty v-else-if="categoryError" :description="categoryError"><van-button type="primary" @click="loadCategories">重新加载</van-button></van-empty>
    <van-empty v-else-if="!categories.length" description="暂无商品分类" />
    <div v-else class="category-layout">
      <van-sidebar v-model="active">
        <van-sidebar-item v-for="item in categories" :key="item.key" :title="item.name" />
      </van-sidebar>
      <section class="category-detail">
        <h2>{{ categories[active].name }}</h2>
        <p>{{ categories[active].note }}</p>
        <div class="category-banner">{{ categories[active].icon }} <strong>精选好物</strong></div>
        <div v-if="loading" class="loading"><van-loading color="#ff4d67" /></div>
        <van-empty v-else-if="productError" :description="productError"><van-button type="primary" @click="loadProducts">重新加载</van-button></van-empty>
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
