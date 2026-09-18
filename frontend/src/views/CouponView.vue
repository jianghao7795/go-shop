<script setup lang="ts">
import { onMounted, ref } from "vue";
import { showToast } from "vant";
import { useUserStore } from "../stores/user";

interface Coupon { id: number; title: string; amount: number; condition: string; }

const userStore = useUserStore();
const coupons = ref<Coupon[]>([]);
const loading = ref(false);

async function load() {
  loading.value = true;
  try {
    const apiBase = import.meta.env.VITE_API_BASE || "http://127.0.0.1:8080";
    const response = await fetch(apiBase + "/api/coupons", {
      headers: { Authorization: "Bearer " + userStore.token },
    });
    coupons.value = response.ok ? await response.json() : [];
  } catch {
    coupons.value = [];
  } finally {
    loading.value = false;
  }
}

onMounted(load);
</script>

<template>
  <div class="sub-page">
    <van-nav-bar title="优惠券" left-arrow @click-left="$router.back()" />
    <div v-if="loading" class="loading"><van-loading color="#ff4d67" /></div>
    <van-empty v-else-if="!coupons.length" description="暂无优惠券" />
    <div v-else class="coupon-list">
      <div v-for="coupon in coupons" :key="coupon.id" class="coupon-card">
        <div class="coupon-amount"><span>¥</span>{{ coupon.amount }}</div>
        <div class="coupon-info">
          <h3>{{ coupon.title }}</h3>
          <p>{{ coupon.condition }}</p>
        </div>
        <van-button size="small" round type="danger" plain @click="showToast('领取成功')">领取</van-button>
      </div>
    </div>
  </div>
</template>
