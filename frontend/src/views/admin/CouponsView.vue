<script setup lang="ts">
import { onMounted, reactive, ref } from "vue";
import { showConfirmDialog, showToast } from "vant";
import http from "../../lib/http";
import { errMsg } from "../../lib/errmsg";

interface Coupon {
  id: number;
  title: string;
  amount: number;
  minAmount: number;
  condition: string;
  startAt: string;
  endAt: string;
}

const list = ref<Coupon[]>([]);
const loading = ref(false);
const saving = ref(false);
const showForm = ref(false);
const editingId = ref<number | null>(null);

const form = reactive({
  title: "",
  amount: "",
  minAmount: "",
  condition: "",
  startAt: "",
  endAt: "",
});

async function load() {
  loading.value = true;
  try {
    const res = await http.get("/api/admin/coupons");
    list.value = res.data;
  } catch {
    showToast("加载失败");
  } finally {
    loading.value = false;
  }
}

function openCreate() {
  editingId.value = null;
  Object.assign(form, {
    title: "",
    amount: "",
    minAmount: "",
    condition: "",
    startAt: "",
    endAt: "",
  });
  showForm.value = true;
}

function formatTime(s: string) {
  if (!s) return "-";
  // 将 RFC3339(Nano) 或 "2006-01-02 15:04:05" 统一展示为 "YYYY-MM-DD HH:mm"
  const t = s.replace("T", " ").replace(/\.\d+/, "").replace(/([+-]\d{2}:\d{2}|Z)$/, "");
  return t.length >= 16 ? t.slice(0, 16) : t;
}

function openEdit(c: Coupon) {
  editingId.value = c.id;
  Object.assign(form, {
    title: c.title,
    amount: String(c.amount),
    minAmount: String(c.minAmount),
    condition: c.condition,
    startAt: c.startAt || "",
    endAt: c.endAt || "",
  });
  showForm.value = true;
}

function buildBody() {
  return {
    title: form.title,
    amount: Number(form.amount) || 0,
    minAmount: Number(form.minAmount) || 0,
    condition: form.condition,
    startAt: form.startAt,
    endAt: form.endAt,
  };
}

async function submit() {
  saving.value = true;
  try {
    const body = buildBody();
    if (editingId.value) {
      await http.put("/api/admin/coupons/" + editingId.value, body);
    } else {
      await http.post("/api/admin/coupons", body);
    }
    showToast("已保存");
    showForm.value = false;
    load();
  } catch (err) {
    showToast(errMsg(err, "保存失败"));
  } finally {
    saving.value = false;
  }
}

async function remove(c: Coupon) {
  try {
    await showConfirmDialog({
      title: "删除优惠券",
      message: `确定删除「${c.title}」吗？`,
    });
  } catch {
    return;
  }
  try {
    await http.delete("/api/admin/coupons/" + c.id);
    showToast("已删除");
    load();
  } catch (err) {
    showToast(errMsg(err, "删除失败"));
  }
}

onMounted(load);
</script>

<template>
  <div class="admin-page">
    <div class="admin-head">
      <h2>优惠券管理</h2>
      <van-button size="small" type="primary" @click="openCreate">新建</van-button>
    </div>

    <van-loading v-if="loading" class="loading" color="#ff4d67" />

    <table v-else class="admin-table">
      <thead>
        <tr>
          <th>标题</th><th>金额</th><th>门槛</th><th>条件</th>
          <th>开始</th><th>结束</th><th>操作</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="c in list" :key="c.id">
          <td>{{ c.title }}</td>
          <td>¥{{ c.amount }}</td>
          <td>¥{{ c.minAmount }}</td>
          <td>{{ c.condition || "-" }}</td>
          <td>{{ formatTime(c.startAt) }}</td>
          <td>{{ formatTime(c.endAt) }}</td>
          <td class="admin-ops">
            <van-button size="small" @click="openEdit(c)">编辑</van-button>
            <van-button size="small" type="danger" @click="remove(c)">删除</van-button>
          </td>
        </tr>
      </tbody>
    </table>

    <van-popup v-model:show="showForm" position="bottom" round>
      <div class="form-popup">
        <h3>{{ editingId ? "编辑优惠券" : "新建优惠券" }}</h3>
        <van-field v-model="form.title" label="标题" placeholder="优惠券标题" />
        <van-field v-model="form.amount" label="金额(元)" type="number" placeholder="金额，单位元" />
        <van-field v-model="form.minAmount" label="门槛(元)" type="number" placeholder="使用门槛，单位元" />
        <van-field v-model="form.condition" label="条件" placeholder="使用条件说明" />
        <van-field v-model="form.startAt" label="开始" placeholder="2006-01-02 15:04:05" />
        <van-field v-model="form.endAt" label="结束" placeholder="2006-01-02 15:04:05" />
        <div class="form-actions">
          <van-button block type="primary" :loading="saving" @click="submit">保存</van-button>
        </div>
      </div>
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
.form-popup {
  padding: 16px 16px 24px;
}
.form-popup h3 {
  margin: 0 0 12px;
  font-size: 18px;
  text-align: center;
}
.form-actions {
  margin-top: 16px;
}
</style>
