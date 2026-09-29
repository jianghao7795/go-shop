<script setup lang="ts">
import { onMounted, reactive, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { Rate, showToast } from "vant";
import http from "../lib/http";

interface OrderItem { productId: number; name: string; emoji: string; color: string; }
interface Order { id: number; orderNo: string; status: string; items: OrderItem[]; }

const route = useRoute();
const router = useRouter();
const order = ref<Order | null>(null);
const loading = ref(false);
const submitting = ref(false);
const reviews = reactive<Record<number, { rating: number; content: string }>>({});

async function load() {
  loading.value = true;
  try {
    const res = await http.get("/api/orders/" + route.params.id);
    order.value = res.data;
    for (const it of order.value.items) {
      reviews[it.productId] = { rating: 5, content: "" };
    }
  } catch {
    order.value = null;
  } finally {
    loading.value = false;
  }
}

function setRating(productId: number, rating: number) {
  reviews[productId].rating = rating;
}

async function submit() {
  if (!order.value) return;
  submitting.value = true;
  try {
    const body = {
      reviews: order.value.items.map((it) => ({
        productId: it.productId,
        rating: reviews[it.productId].rating,
        content: reviews[it.productId].content,
      })),
    };
    await http.post("/api/orders/" + order.value.id + "/reviews", body);
    showToast("评价成功");
    router.replace("/orders");
  } catch (err) {
    const e = err as any;
    if (e?.response) showToast(e.response.data?.message || "评价失败");
    else showToast("评价服务暂不可用");
  } finally {
    submitting.value = false;
  }
}

onMounted(load);
</script>

<template>
  <div class="sub-page">
    <van-nav-bar title="评价订单" left-arrow @click-left="$router.back()" />
    <van-loading v-if="loading" class="loading" color="#ff4d67" />
    <van-empty v-else-if="!order" description="订单不存在" />
    <template v-else>
      <van-cell-group v-for="it in order.items" :key="it.productId" inset class="review-card">
        <van-cell :title="it.name">
          <template #icon><div class="cart-thumb" :style="{ background: it.color }">{{ it.emoji }}</div></template>
        </van-cell>
        <div class="review-form">
          <div class="review-rate-row">
            <span>商品评分</span>
            <Rate :model-value="reviews[it.productId].rating" color="#ff4d67" @update:model-value="(v: number) => setRating(it.productId, v)" />
          </div>
          <van-field v-model="reviews[it.productId].content" type="textarea" rows="2" autosize placeholder="说说你的使用感受吧" />
        </div>
      </van-cell-group>
      <div class="review-submit">
        <van-button block round type="danger" :loading="submitting" @click="submit">提交评价</van-button>
      </div>
    </template>
  </div>
</template>

<style scoped>
.review-card {
  margin-bottom: 12px;
}
.review-form {
  padding: 0 16px 16px;
}
.review-rate-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 0;
  font-size: 14px;
}
.review-submit {
  margin: 20px 16px;
}
</style>
