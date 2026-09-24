<script setup lang="ts">
import { onMounted, ref } from "vue";
import { DropdownItem, DropdownMenu, Picker, showToast } from "vant";
import http from "../../lib/http";

interface Order {
  id: number;
  orderNo: string;
  userId: number;
  status: string;
  amount: number;
  receiver: string;
  phone: string;
  region: string;
  detail: string;
  items: any[];
  createdAt: string;
}

const STATUSES = [
  { value: "pending", label: "待付款" },
  { value: "shipped", label: "待收货" },
  { value: "completed", label: "待评价" },
  { value: "aftersale", label: "售后" },
  { value: "finished", label: "已完成" },
];

const STATUS_MAP: Record<string, string> = STATUSES.reduce(
  (map, s) => {
    map[s.value] = s.label;
    return map;
  },
  {} as Record<string, string>
);

const list = ref<Order[]>([]);
const loading = ref(false);
const status = ref("");
const showStatusPicker = ref(false);
const current = ref<Order | null>(null);

const statusOptions = [
  { text: "全部", value: "" },
  ...STATUSES.map((s) => ({ text: s.label, value: s.value })),
];

const statusColumns = STATUSES.map((s) => ({ text: s.label, value: s.value }));

function statusLabel(value: string) {
  return STATUS_MAP[value] || value || "-";
}

function formatTime(value: string) {
  if (!value) return "-";
  const d = new Date(value);
  if (isNaN(d.getTime())) return value;
  const p = (n: number) => String(n).padStart(2, "0");
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(
    d.getHours()
  )}:${p(d.getMinutes())}`;
}

async function load() {
  loading.value = true;
  try {
    const res = await http.get("/api/admin/orders", {
      params: status.value ? { status: status.value } : {},
    });
    list.value = res.data;
  } catch {
    showToast("加载失败");
  } finally {
    loading.value = false;
  }
}

function openStatus(o: Order) {
  current.value = o;
  showStatusPicker.value = true;
}

async function changeStatus(o: Order, target: string) {
  if (!o || !target || target === o.status) return;
  try {
    await http.put("/api/admin/orders/" + o.id + "/status", { status: target });
    showToast("已更新");
    load();
  } catch {
    showToast("更新失败");
  }
}

function onStatusConfirm({ selectedOptions }: { selectedOptions: any[] }) {
  const target = selectedOptions[0]?.value;
  showStatusPicker.value = false;
  if (current.value) changeStatus(current.value, target);
}

onMounted(load);
</script>

<template>
  <div class="admin-page">
    <div class="admin-head">
      <h2>订单管理</h2>
    </div>

    <DropdownMenu class="order-filter">
      <DropdownItem v-model="status" :options="statusOptions" @change="load" />
    </DropdownMenu>

    <van-loading v-if="loading" class="loading" color="#ff4d67" />

    <table v-else class="admin-table">
      <thead>
        <tr>
          <th>订单号</th><th>用户</th><th>金额</th>
          <th>状态</th><th>下单时间</th><th>操作</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="o in list" :key="o.id">
          <td>{{ o.orderNo }}</td>
          <td>{{ o.userId }}</td>
          <td>¥{{ o.amount }}</td>
          <td>{{ statusLabel(o.status) }}</td>
          <td>{{ formatTime(o.createdAt) }}</td>
          <td class="admin-ops">
            <van-button size="small" @click="openStatus(o)">改状态</van-button>
          </td>
        </tr>
      </tbody>
    </table>

    <van-popup v-model:show="showStatusPicker" position="bottom" round>
      <Picker
        :columns="statusColumns"
        title="修改状态"
        @confirm="onStatusConfirm"
        @cancel="showStatusPicker = false"
      />
    </van-popup>
  </div>
</template>

<style scoped>
.admin-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 12px;
}
.admin-head h2 {
  margin: 0;
  font-size: 20px;
}
.order-filter {
  margin-bottom: 12px;
  border-radius: 8px;
  overflow: hidden;
}
.admin-table {
  width: 100%;
  border-collapse: collapse;
  background: #fff;
  border-radius: 8px;
  overflow: hidden;
}
.admin-table th,
.admin-table td {
  padding: 10px 8px;
  text-align: left;
  font-size: 13px;
  border-bottom: 1px solid #f2f3f5;
  white-space: nowrap;
}
.admin-table th {
  color: #969ba5;
  font-weight: 500;
  background: #fafafa;
}
.admin-ops {
  display: flex;
  gap: 6px;
}
</style>
