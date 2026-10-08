<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { Rate, showToast } from "vant";
import http from "../../lib/http";

interface ReviewItem {
  id: number;
  userId: string;
  productId: number;
  productName: string;
  rating: number;
  content: string;
  createdAt: string;
}

const list = ref<ReviewItem[]>([]);
const total = ref(0);
const page = ref(1);
const pageSize = 20;
const totalPages = computed(() => Math.ceil(total.value / pageSize));
const loading = ref(false);

async function load() {
  loading.value = true;
  try {
    const res = await http.get("/api/admin/reviews", { params: { page: page.value, pageSize } });
    list.value = res.data.items;
    total.value = res.data.total;
  } catch {
    showToast("加载失败");
  } finally {
    loading.value = false;
  }
}

function changePage(p: number) {
  page.value = p;
  load();
}

function formatTime(s: string) {
  if (!s) return "-";
  const t = s.replace("T", " ").replace(/\.\d+/, "").replace(/([+-]\d{2}:\d{2}|Z)$/, "");
  return t.length >= 16 ? t.slice(0, 16) : t;
}

onMounted(load);
</script>

<template>
  <div class="admin-page">
    <div class="admin-head"><h2>评价管理</h2></div>
    <van-loading v-if="loading" class="loading" color="#ff4d67" />
    <template v-else>
      <table v-if="list.length" class="admin-table">
        <thead>
          <tr><th>ID</th><th>商品</th><th>用户</th><th>评分</th><th>内容</th><th>时间</th></tr>
        </thead>
        <tbody>
          <tr v-for="r in list" :key="r.id">
            <td>{{ r.id }}</td>
            <td>{{ r.productName || "#" + r.productId }}</td>
            <td>{{ r.userId }}</td>
            <td><Rate :model-value="r.rating" readonly allow-half size="12" color="#ff4d67" /></td>
            <td class="review-content">{{ r.content || "-" }}</td>
            <td>{{ formatTime(r.createdAt) }}</td>
          </tr>
        </tbody>
      </table>
      <van-empty v-else description="暂无评价" />
      <div class="pagination" v-if="totalPages > 1">
        <van-button size="small" :disabled="page <= 1" @click="changePage(page - 1)">上一页</van-button>
        <span>{{ page }} / {{ totalPages }}（共 {{ total }} 条）</span>
        <van-button size="small" :disabled="page >= totalPages" @click="changePage(page + 1)">下一页</van-button>
      </div>
    </template>
  </div>
</template>

<style scoped>
.review-content {
  max-width: 320px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
