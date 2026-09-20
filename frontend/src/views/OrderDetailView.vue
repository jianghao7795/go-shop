<script setup lang="ts">
import { nextTick, onMounted, ref } from "vue";
import { useRoute } from "vue-router";
import { showToast } from "vant";
import http from "../lib/http";

interface OrderItem { productId: number; name: string; price: number; quantity: number; emoji: string; color: string; }
interface Order { id: number; orderNo: string; status: string; amount: number; receiver: string; phone: string; region: string; detail: string; items: OrderItem[]; createdAt: string; }

const route = useRoute();
const order = ref<Order | null>(null);
const loading = ref(false);

const AMAP_KEY = import.meta.env.VITE_AMAP_KEY || "";
const AMAP_SECURITY_KEY = import.meta.env.VITE_AMAP_SECURITY_KEY || "";
const mapRef = ref<HTMLDivElement | null>(null);
const mapError = ref("");

let amapPromise: Promise<any> | null = null;

function loadAMap(): Promise<any> {
  if (!amapPromise) {
    amapPromise = new Promise((resolve, reject) => {
      (window as any)._AMapSecurityConfig = { securityJsCode: AMAP_SECURITY_KEY };
      const script = document.createElement("script");
      script.src = `https://webapi.amap.com/maps?v=2.0&key=${AMAP_KEY}&plugin=AMap.Geocoder`;
      script.onload = () => resolve((window as any).AMap);
      script.onerror = () => { amapPromise = null; reject(new Error("amap load failed")); };
      document.head.appendChild(script);
    });
  }
  return amapPromise;
}

function initMap(AMap: any, position: [number, number], title: string) {
  if (!mapRef.value) return;
  const map = new AMap.Map(mapRef.value, { zoom: 15, center: position });
  map.add(new AMap.Marker({ position, title }));
}

async function renderMap() {
  if (!order.value || !AMAP_KEY) return;
  try {
    const AMap = await loadAMap();
    await nextTick();
    if (!mapRef.value) return;
    const address = `${order.value.region} ${order.value.detail}`.trim();
    const geocoder = new AMap.Geocoder();
    geocoder.getLocation(address, (status: string, result: any) => {
      if (status === "complete" && result.geocodes && result.geocodes.length) {
        const loc = result.geocodes[0].location;
        initMap(AMap, [loc.lng, loc.lat], address);
      } else {
        geocoder.getLocation(order.value!.region || "深圳", (s2: string, r2: any) => {
          if (s2 === "complete" && r2.geocodes && r2.geocodes.length) {
            const loc2 = r2.geocodes[0].location;
            initMap(AMap, [loc2.lng, loc2.lat], address);
          } else {
            mapError.value = "地图定位失败";
          }
        });
      }
    });
  } catch {
    mapError.value = "地图暂不可用";
  }
}

const statusMap: Record<string, { text: string; color: string }> = {
  pending: { text: "待付款", color: "#ff976a" },
  shipped: { text: "待收货", color: "#1989fa" },
  completed: { text: "待评价", color: "#07c160" },
  aftersale: { text: "售后", color: "#ee0a24" },
  finished: { text: "已完成", color: "#969ba5" },
};

async function load() {
  loading.value = true;
  try {
    const res = await http.get("/api/orders/" + route.params.id);
    order.value = res.data;
  } catch {
    order.value = null;
  } finally {
    loading.value = false;
  }
}

async function changeStatus(status: string, text: string) {
  if (!order.value) return;
  try {
    await http.put("/api/orders/" + order.value.id + "/status", { status });
    showToast(text + "成功");
    load();
  } catch { /* 忽略 */ }
}

onMounted(async () => {
  await load();
  renderMap();
});
</script>

<template>
  <div class="sub-page">
    <van-nav-bar title="订单详情" left-arrow @click-left="$router.back()" />
    <div v-if="loading" class="loading"><van-loading color="#ff4d67" /></div>
    <van-empty v-else-if="!order" description="订单不存在" />
    <template v-else>
      <van-cell-group inset class="detail-block">
        <van-cell title="订单号" :value="order.orderNo" />
        <van-cell title="状态">
          <template #value><span :style="{ color: statusMap[order.status]?.color }">{{ statusMap[order.status]?.text || order.status }}</span></template>
        </van-cell>
        <van-cell title="下单时间" :value="new Date(order.createdAt).toLocaleString()" />
      </van-cell-group>
      <van-cell-group v-if="order.receiver" inset class="detail-block">
        <van-cell title="收货信息">
          <template #value>
            <div class="order-receiver">{{ order.receiver }} {{ order.phone }}</div>
            <div class="order-address">{{ order.region }} {{ order.detail }}</div>
          </template>
        </van-cell>
      </van-cell-group>
      <div v-if="AMAP_KEY && !mapError" ref="mapRef" class="order-map"></div>
      <div v-if="mapError" class="order-map-error">{{ mapError }}</div>
      <van-cell-group inset class="detail-block">
        <van-cell v-for="item in order.items" :key="item.productId" :title="item.name" :value="'x' + item.quantity + '  ¥' + (item.price * item.quantity).toFixed(2)">
          <template #icon><div class="cart-thumb" :style="{ background: item.color }">{{ item.emoji }}</div></template>
        </van-cell>
        <van-cell title="合计" :value="'¥' + order.amount.toFixed(2)" />
      </van-cell-group>
      <div class="detail-actions-bar">
        <van-button v-if="order.status === 'pending'" round type="danger" @click="changeStatus('shipped', '支付')">去支付</van-button>
        <van-button v-if="order.status === 'shipped'" round type="primary" @click="changeStatus('completed', '确认收货')">确认收货</van-button>
        <van-button v-if="order.status === 'completed'" round type="success" @click="changeStatus('finished', '评价')">评价</van-button>
      </div>
    </template>
  </div>
</template>
