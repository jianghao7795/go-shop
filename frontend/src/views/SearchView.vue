<script setup lang="ts">
import { onMounted, ref } from "vue";
import { useRouter } from "vue-router";
import { showToast } from "vant";
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

const HISTORY_KEY = "shop_search_history";
const HOT_KEYWORDS = ["无线耳机", "保温杯", "针织开衫", "精华液", "坚果礼盒", "台灯"];

const router = useRouter();
const query = ref("");
const results = ref<Product[]>([]);
const suggestions = ref<string[]>([]);
const history = ref<string[]>([]);
const loading = ref(false);
const searched = ref(false);

function loadHistory() {
  try {
    history.value = JSON.parse(localStorage.getItem(HISTORY_KEY) || "[]");
  } catch {
    history.value = [];
  }
}

function saveHistory(word: string) {
  const w = word.trim();
  if (!w) return;
  history.value = [w, ...history.value.filter((h) => h !== w)].slice(0, 10);
  localStorage.setItem(HISTORY_KEY, JSON.stringify(history.value));
}

function clearHistory() {
  history.value = [];
  localStorage.removeItem(HISTORY_KEY);
}

async function doSearch(word: string) {
  query.value = word;
  searched.value = true;
  suggestions.value = [];
  loading.value = true;
  try {
    const res = await http.get("/api/products", { params: { search: word } });
    results.value = res.data;
    saveHistory(word);
  } catch {
    showToast("搜索失败");
    results.value = [];
  } finally {
    loading.value = false;
  }
}

let timer: number | undefined;
function onInput(val: string) {
  query.value = val;
  searched.value = false;
  if (timer) window.clearTimeout(timer);
  if (!val.trim()) {
    suggestions.value = [];
    return;
  }
  timer = window.setTimeout(async () => {
    try {
      const res = await http.get("/api/products", { params: { search: val } });
      suggestions.value = res.data.slice(0, 5).map((p: Product) => p.name);
    } catch {
      suggestions.value = [];
    }
  }, 200);
}

onMounted(loadHistory);
</script>

<template>
  <div class="search-page">
    <van-search
      :model-value="query"
      shape="round"
      placeholder="搜索商品、品牌"
      autofocus
      @update:model-value="onInput"
      @search="doSearch"
    >
      <template #action>
        <span class="cancel-text" @click="router.back()">取消</span>
      </template>
    </van-search>

    <main class="search-content">
      <!-- 搜索建议 -->
      <div v-if="!searched && query.trim() && suggestions.length" class="suggest-list">
        <div v-for="s in suggestions" :key="s" class="suggest-item" @click="doSearch(s)">
          {{ s }}
        </div>
      </div>

      <!-- 历史 + 热门 -->
      <template v-else-if="!searched && !query.trim()">
        <div v-if="history.length" class="search-section">
          <div class="section-title">
            <span>搜索历史</span>
            <van-icon name="delete-o" @click="clearHistory" />
          </div>
          <div class="tag-list">
            <span v-for="h in history" :key="h" class="tag" @click="doSearch(h)">{{ h }}</span>
          </div>
        </div>
        <div class="search-section">
          <div class="section-title"><span>热门搜索</span></div>
          <div class="tag-list">
            <span v-for="k in HOT_KEYWORDS" :key="k" class="tag hot" @click="doSearch(k)">{{ k }}</span>
          </div>
        </div>
      </template>

      <!-- 搜索结果 -->
      <template v-else-if="searched">
        <van-loading v-if="loading" class="loading" color="#ff4d67" />
        <van-empty v-else-if="!results.length" description="没有找到相关商品" />
        <div v-else class="product-grid">
          <article
            v-for="p in results"
            :key="p.id"
            class="product-card"
            @click="router.push(`/product/${p.id}`)"
          >
            <div class="product-thumb" :style="{ background: p.color }">{{ p.emoji }}</div>
            <div class="product-info">
              <h3>{{ p.name }}</h3>
              <p class="product-description">{{ p.description }}</p>
              <div class="product-meta">
                <span class="product-price">¥{{ p.price.toFixed(2) }}</span>
                <del>¥{{ p.originalPrice.toFixed(2) }}</del>
              </div>
            </div>
          </article>
        </div>
      </template>
    </main>
  </div>
</template>

<style scoped>
.search-page {
  min-height: 100vh;
  background: #f5f6f8;
}
.cancel-text {
  color: #646a73;
  font-size: 14px;
  padding: 0 4px;
}
.search-content {
  padding: 0 12px 20px;
}
.suggest-list {
  background: #fff;
  border-radius: 10px;
  overflow: hidden;
}
.suggest-item {
  padding: 12px 16px;
  font-size: 14px;
  border-bottom: 1px solid #f0f1f4;
}
.suggest-item:last-child {
  border-bottom: none;
}
.search-section {
  margin-bottom: 16px;
}
.section-title {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 4px;
  font-size: 14px;
  font-weight: 600;
  color: #1f2430;
}
.tag-list {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}
.tag {
  padding: 6px 14px;
  border-radius: 14px;
  background: #fff;
  color: #646a73;
  font-size: 13px;
}
.tag.hot {
  color: #ff4d67;
  background: #fff0f2;
}
</style>
