<script setup lang="ts">
import { computed, onMounted, reactive, ref } from "vue";
import { DropdownItem, DropdownMenu, showConfirmDialog, showToast } from "vant";
import http from "../../lib/http";
import { errMsg } from "../../lib/errmsg";
import { aftersaleStatuses, orderStatuses, type Order } from "../../lib/orders";

const list = ref<Order[]>([]);
const total = ref(0);
const page = ref(1);
const totalPages = computed(() => Math.ceil(total.value / 20));
const loading = ref(false);
const busy = ref(false);
const status = ref("");
const current = ref<Order | null>(null);
const form = reactive({ carrier: "", trackingNo: "", text: "", note: "", returnAddress: "", restock: false });
const statusOptions = [{ text: "全部", value: "" }, ...Object.entries(orderStatuses).map(([value, s]) => ({ value, text: s.text }))];

async function load() {
  loading.value = true;
  try {
    const r = await http.get("/api/admin/orders", { params: { status: status.value || undefined, page: page.value, pageSize: 20 } });
    list.value = r.data.items || []; total.value = r.data.total;
  } catch (e) { showToast(errMsg(e, "加载失败")); }
  finally { loading.value = false; }
}

async function open(o: Order) {
  try {
    const r = await http.get("/api/admin/orders/" + o.id);
    current.value = r.data;
    Object.assign(form, { carrier: o.carrier || "", trackingNo: o.trackingNo || "", text: "", note: "", returnAddress: o.returnAddress || "", restock: false });
  } catch (e) { showToast(errMsg(e, "读取订单失败")); }
}

async function act(action: string) {
  if (!current.value || busy.value) return;
  const id = current.value.id;
  if (action === "refund") {
    try { await showConfirmDialog({ title: "确认整单退款", message: "将向买家原路退还 ¥" + current.value.amount.toFixed(2) + "。退货订单请先验收；勾选回库后会恢复库存。" }); }
    catch { return; }
  }
  busy.value = true;
  try {
    if (action === "cancel") await http.put("/api/admin/orders/" + id + "/status", { status: "cancelled" });
    else if (action === "ship") await http.post("/api/admin/orders/" + id + "/ship", { carrier: form.carrier, trackingNo: form.trackingNo });
    else if (action === "logistics") await http.post("/api/admin/orders/" + id + "/logistics", { text: form.text });
    else await http.post("/api/admin/orders/" + id + "/aftersale", { action, note: form.note, returnAddress: form.returnAddress, restock: form.restock });
    showToast("处理成功");
    current.value = (await http.get("/api/admin/orders/" + id)).data;
    form.text = "";
    await load();
  } catch (e) {
    showToast(errMsg(e, "操作失败"));
    try { current.value = (await http.get("/api/admin/orders/" + id)).data; } catch { /* retain details for retry */ }
  } finally { busy.value = false; }
}

function changePage(p: number) { page.value = p; load(); }
onMounted(load);
</script>

<template>
  <div class="admin-page">
    <div class="admin-head"><h2>订单管理</h2></div>
    <DropdownMenu><DropdownItem v-model="status" :options="statusOptions" @change="page = 1; load()" /></DropdownMenu>
    <van-loading v-if="loading" class="loading" />
    <table v-else class="admin-table">
      <thead><tr><th>订单号</th><th>用户</th><th>金额</th><th>状态</th><th>下单时间</th><th>操作</th></tr></thead>
      <tbody><tr v-for="o in list" :key="o.id">
        <td>{{ o.orderNo }}</td><td>{{ o.userId }}</td><td>¥{{ o.amount.toFixed(2) }}</td>
        <td>{{ orderStatuses[o.status]?.text || o.status }}</td><td>{{ new Date(o.createdAt).toLocaleString() }}</td>
        <td><van-button size="small" @click="open(o)">详情 / 处理</van-button></td>
      </tr></tbody>
    </table>
    <van-empty v-if="!loading && !list.length" description="暂无订单" />
    <div v-if="totalPages > 1" class="pagination">
      <van-button size="small" :disabled="page <= 1" @click="changePage(page - 1)">上一页</van-button>
      <span>{{ page }} / {{ totalPages }}（共 {{ total }} 条）</span>
      <van-button size="small" :disabled="page >= totalPages" @click="changePage(page + 1)">下一页</van-button>
    </div>
    <van-popup :show="!!current" position="right" :close-on-click-overlay="!busy" class="order-panel" @update:show="v => { if (!v) current = null; }">
      <template v-if="current">
        <van-nav-bar title="订单详情与处理" right-text="关闭" @click-right="!busy && (current = null)" />
        <van-cell title="订单号" :value="current.orderNo" />
        <van-cell title="订单状态" :value="orderStatuses[current.status]?.text || current.status" />
        <van-cell title="支付金额" :value="'¥' + current.amount.toFixed(2)" />
        <van-cell title="收货人" :value="current.receiver + ' ' + current.phone" :label="current.region + ' ' + current.detail" />
        <van-cell v-for="it in current.items" :key="it.productId + ':' + it.skuId" :title="it.name" :label="it.specs" :value="'x' + it.quantity + ' · ¥' + it.price.toFixed(2)" />
        <div v-if="current.status === 'pending'" class="panel-actions"><van-button :loading="busy" @click="act('cancel')">关闭未付款订单</van-button></div>
        <van-form v-if="current.status === 'paid'" @submit="act('ship')">
          <van-cell title="发货" />
          <van-field v-model="form.carrier" label="快递公司" maxlength="64" :rules="[{ required: true }]" />
          <van-field v-model="form.trackingNo" label="运单号" maxlength="128" :rules="[{ required: true }]" />
          <div class="panel-actions"><van-button native-type="submit" type="primary" :loading="busy">确认发货</van-button></div>
        </van-form>
        <template v-if="current.trackingNo">
          <van-cell title="物流" :value="current.carrier" :label="current.trackingNo" />
          <van-cell v-for="(ev, i) in current.logistics || []" :key="i" :title="ev.text" :label="new Date(ev.time).toLocaleString()" />
          <van-form v-if="current.shippedAt && current.status !== 'refunded'" @submit="act('logistics')">
            <van-field v-model="form.text" label="物流进度" type="textarea" maxlength="500" placeholder="填写真实物流进度，买家可查看" :rules="[{ required: true }]" />
            <div class="panel-actions"><van-button native-type="submit" :loading="busy">更新进度</van-button></div>
          </van-form>
        </template>
        <template v-if="current.aftersaleStatus">
          <van-cell title="售后进度" :value="aftersaleStatuses[current.aftersaleStatus] || current.aftersaleStatus" />
          <van-cell title="售后类型" :value="current.aftersaleType === 'return_refund' ? '退货退款' : '仅退款'" />
          <van-cell title="申请原因" :label="current.aftersaleReason" />
          <van-cell v-if="current.aftersaleNote" title="处理备注" :label="current.aftersaleNote" />
          <van-cell v-if="current.returnAddress" title="退货地址" :label="current.returnAddress" />
          <van-cell v-if="current.returnTrackingNo" title="买家退货运单" :value="current.returnCarrier" :label="current.returnTrackingNo" />
          <template v-if="current.status === 'aftersale'">
            <van-field v-model="form.note" label="处理备注" type="textarea" maxlength="500" placeholder="驳回时必须填写原因" />
            <van-field v-if="current.aftersaleType === 'return_refund' && current.aftersaleStatus === 'requested'" v-model="form.returnAddress" label="退货地址" type="textarea" maxlength="500" />
            <van-checkbox v-if="current.aftersaleType === 'return_refund' && current.aftersaleStatus !== 'refunding'" v-model="form.restock" style="margin:16px">已验收，可售商品退回库存</van-checkbox>
            <div class="panel-actions">
              <van-button v-if="current.aftersaleStatus === 'requested'" :disabled="busy" @click="act('reject')">驳回申请</van-button>
              <van-button v-if="current.aftersaleStatus === 'requested' && current.aftersaleType === 'return_refund'" type="primary" :loading="busy" @click="act('approve_return')">同意退货</van-button>
              <van-button v-if="current.aftersaleType === 'refund_only' || (current.aftersaleStatus === 'returning' && current.returnTrackingNo) || current.aftersaleStatus === 'refunding'" type="danger" :loading="busy" @click="act('refund')">{{ current.aftersaleStatus === 'refunding' ? '重试确认退款' : '确认原路退款' }}</van-button>
            </div>
          </template>
        </template>
      </template>
    </van-popup>
  </div>
</template>

<style scoped>
.order-panel { width: min(560px, 100vw); height: 100%; overflow-y: auto; }
.panel-actions { padding: 16px; display: flex; flex-wrap: wrap; gap: 10px; }
</style>
