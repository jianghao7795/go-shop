<script setup lang="ts">
import { nextTick, onMounted, ref } from "vue";
import { useRoute } from "vue-router";
import { showConfirmDialog, showToast } from "vant";
import http from "../lib/http";
import AftersaleForm from "../components/AftersaleForm.vue";
import { aftersaleStatuses, canApplyAftersale, orderStatuses as statusMap, type Order } from "../lib/orders";
import { errMsg } from "../lib/errmsg";

const route = useRoute();
const order = ref<Order | null>(null);
const loading = ref(false);
const aftersaleOrder = ref<Order | null>(null);
const returnCarrier = ref("");
const returnTrackingNo = ref("");
const returning = ref(false);

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

async function load() {
  loading.value = true;
  try {
    const res = await http.get("/api/orders/" + route.params.id);
    order.value = res.data;
    returnCarrier.value = order.value?.returnCarrier || "";
    returnTrackingNo.value = order.value?.returnTrackingNo || "";
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
  } catch (e) { showToast(errMsg(e, "操作失败")); }
}

async function cancelOrder() {
  try {
    await showConfirmDialog({ title: "取消订单", message: "确定取消该订单吗？" });
  } catch { return; }
  await changeStatus("cancelled", "取消订单");
}

async function saveReturn() {
  if (!order.value || returning.value) return;
  returning.value = true;
  try {
    await http.put(`/api/orders/${order.value.id}/return`, { carrier: returnCarrier.value, trackingNo: returnTrackingNo.value });
    showToast("退货运单已提交");
    await load();
  } catch (e) { showToast(errMsg(e, "提交失败")); }
  finally { returning.value = false; }
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
        <van-cell v-for="item in order.items" :key="item.productId + ':' + item.skuId" :title="item.name" :label="item.specs" :value="'x' + item.quantity + '  ¥' + (item.price * item.quantity).toFixed(2)">
          <template #icon><div class="cart-thumb" :style="{ background: item.color }">{{ item.emoji }}</div></template>
        </van-cell>
        <van-cell title="合计" :value="'¥' + order.amount.toFixed(2)" />
      </van-cell-group>
      <van-cell-group v-if="order.trackingNo" inset class="detail-block">
        <van-cell title="快递公司" :value="order.carrier" />
        <van-cell title="快递单号" :value="order.trackingNo" />
        <van-cell title="物流进度" label="以下进度由商家更新" />
        <van-cell v-for="(event, i) in order.logistics || []" :key="i" :title="event.text" :label="new Date(event.time).toLocaleString()" />
      </van-cell-group>
      <van-cell-group v-if="order.aftersaleStatus" inset class="detail-block">
        <van-cell title="售后进度" :value="aftersaleStatuses[order.aftersaleStatus] || order.aftersaleStatus" />
        <van-cell title="售后类型" :value="order.aftersaleType === 'return_refund' ? '退货退款' : '仅退款'" />
        <van-cell title="申请原因" :label="order.aftersaleReason" />
        <van-cell v-if="order.aftersaleNote" title="商家回复" :label="order.aftersaleNote" />
        <van-cell v-if="order.returnAddress" title="退货地址" :label="order.returnAddress" />
        <van-cell v-if="order.refundedAt" title="退款时间" :value="new Date(order.refundedAt).toLocaleString()" />
        <van-form v-if="order.status === 'aftersale' && order.aftersaleStatus === 'returning'" @submit="saveReturn">
          <van-field v-model="returnCarrier" label="退货快递" maxlength="64" :rules="[{ required: true, message: '请填写快递公司' }]" />
          <van-field v-model="returnTrackingNo" label="退货单号" maxlength="128" :rules="[{ required: true, message: '请填写运单号' }]" />
          <div style="padding:12px"><van-button block type="primary" native-type="submit" :loading="returning">保存退货运单</van-button></div>
        </van-form>
      </van-cell-group>
      <div class="detail-actions-bar">
        <van-button v-if="order.status === 'pending'" round type="danger" @click="$router.push('/pay/' + order.id)">去支付</van-button>
        <van-button v-if="order.status === 'pending'" round plain type="default" @click="cancelOrder()">取消订单</van-button>
        <van-button v-if="order.status === 'shipped'" round type="primary" @click="changeStatus('completed', '确认收货')">确认收货</van-button>
        <van-button v-if="canApplyAftersale(order)" round plain type="warning" @click="aftersaleOrder = order">申请售后</van-button>
        <van-button v-if="order.status === 'completed'" round type="success" @click="$router.push('/review/' + order.id)">评价</van-button>
      </div>
    </template>
    <AftersaleForm :order="aftersaleOrder" @close="aftersaleOrder = null" @submitted="load" />
  </div>
</template>
