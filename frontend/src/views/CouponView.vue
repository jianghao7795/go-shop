<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { showToast } from "vant";
import http from "../lib/http";
import { errMsg } from "../lib/errmsg";

interface Coupon {
  id: number;
  title: string;
  amount: number;
  minAmount: number;
  condition: string;
}
interface MyCoupon {
  id: number;
  couponId: number;
  status: string;
  title: string;
  amount: number;
  condition: string;
}

const coupons = ref<Coupon[]>([]);
const mine = ref<MyCoupon[]>([]);
const loading = ref(false);

const claimedIds = computed(() => new Set(mine.value.map((m) => m.couponId)));

async function load() {
  loading.value = true;
  try {
    const [c, m] = await Promise.all([
      http.get("/api/coupons"),
      http.get("/api/coupons/mine"),
    ]);
    coupons.value = c.data;
    mine.value = m.data;
  } catch {
    coupons.value = [];
    mine.value = [];
  } finally {
    loading.value = false;
  }
}

async function claim(coupon: Coupon) {
  try {
    await http.post("/api/coupons/" + coupon.id + "/claim");
    showToast("领取成功");
    load();
  } catch (err) {
    showToast(errMsg(err, "领取失败"));
  }
}

onMounted(load);
</script>

<template>
  <div class="sub-page">
    <van-nav-bar title="优惠券" left-arrow @click-left="$router.back()" />
    <div v-if="loading" class="loading"><van-loading color="#ff4d67" /></div>

    <template v-else>
      <div class="coupon-section">
        <h3>领券中心</h3>
        <van-empty v-if="!coupons.length" description="暂无可领优惠券" />
        <div v-else class="coupon-list">
          <div v-for="coupon in coupons" :key="coupon.id" class="coupon-card">
            <div class="coupon-amount">
              <span>¥</span>
              {{ coupon.amount }}
            </div>
            <div class="coupon-info">
              <h3>{{ coupon.title }}</h3>
              <p>{{ coupon.condition }}</p>
            </div>
            <van-button
              v-if="!claimedIds.has(coupon.id)"
              size="small"
              round
              type="danger"
              plain
              @click="claim(coupon)"
            >领取</van-button>
            <van-button v-else size="small" round disabled>已领取</van-button>
          </div>
        </div>
      </div>

      <div class="coupon-section">
        <h3>我的券（{{ mine.length }}）</h3>
        <van-empty v-if="!mine.length" description="还没有优惠券" />
        <div v-else class="coupon-list">
          <div
            v-for="m in mine"
            :key="m.id"
            class="coupon-card"
            :class="{ 'coupon-used': m.status !== 'unused' }"
          >
            <div class="coupon-amount">
              <span>¥</span>
              {{ m.amount }}
            </div>
            <div class="coupon-info">
              <h3>{{ m.title }}</h3>
              <p>{{ m.condition }}</p>
              <p v-if="m.status !== 'unused'" class="coupon-used-tip">已使用</p>
            </div>
          </div>
        </div>
      </div>
    </template>
  </div>
</template>
