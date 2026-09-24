<script setup lang="ts">
import { onMounted, reactive, ref } from "vue";
import { showConfirmDialog, showToast } from "vant";
import http from "../../lib/http";

interface Category {
  id: number;
  key: string;
  name: string;
  icon: string;
  note: string;
}

const list = ref<Category[]>([]);
const loading = ref(false);
const saving = ref(false);
const showForm = ref(false);
const editingId = ref<number | null>(null);

const form = reactive({ key: "", name: "", icon: "", note: "" });

async function load() {
  loading.value = true;
  try {
    const res = await http.get("/api/admin/categories");
    list.value = res.data;
  } catch {
    showToast("加载失败");
  } finally {
    loading.value = false;
  }
}

function openCreate() {
  editingId.value = null;
  Object.assign(form, { key: "", name: "", icon: "", note: "" });
  showForm.value = true;
}

function openEdit(c: Category) {
  editingId.value = c.id;
  Object.assign(form, { key: c.key, name: c.name, icon: c.icon, note: c.note });
  showForm.value = true;
}

async function submit() {
  saving.value = true;
  try {
    const body = { ...form };
    if (editingId.value) {
      await http.put("/api/admin/categories/" + editingId.value, body);
    } else {
      await http.post("/api/admin/categories", body);
    }
    showToast("已保存");
    showForm.value = false;
    load();
  } catch {
    showToast("保存失败");
  } finally {
    saving.value = false;
  }
}

async function remove(c: Category) {
  try {
    await showConfirmDialog({
      title: "删除分类",
      message: `确定删除「${c.name}」吗？`,
    });
  } catch {
    return;
  }
  try {
    await http.delete("/api/admin/categories/" + c.id);
    showToast("已删除");
    load();
  } catch {
    showToast("删除失败");
  }
}

onMounted(load);
</script>

<template>
  <div class="admin-page">
    <div class="admin-head">
      <h2>分类管理</h2>
      <van-button size="small" type="primary" @click="openCreate">新建</van-button>
    </div>

    <van-loading v-if="loading" class="loading" color="#ff4d67" />

    <table v-else class="admin-table">
      <thead>
        <tr><th>ID</th><th>Key</th><th>名称</th><th>图标</th><th>备注</th><th>操作</th></tr>
      </thead>
      <tbody>
        <tr v-for="c in list" :key="c.id">
          <td>{{ c.id }}</td>
          <td>{{ c.key }}</td>
          <td>{{ c.name }}</td>
          <td>{{ c.icon }}</td>
          <td>{{ c.note || "-" }}</td>
          <td class="admin-ops">
            <van-button size="small" @click="openEdit(c)">编辑</van-button>
            <van-button size="small" type="danger" @click="remove(c)">删除</van-button>
          </td>
        </tr>
      </tbody>
    </table>

    <van-popup v-model:show="showForm" position="bottom" round>
      <div class="form-popup">
        <h3>{{ editingId ? "编辑分类" : "新建分类" }}</h3>
        <van-field v-model="form.key" label="Key" placeholder="如 fruit" />
        <van-field v-model="form.name" label="名称" placeholder="分类名称" />
        <van-field v-model="form.icon" label="图标" placeholder="如 🍎" />
        <van-field
          v-model="form.note"
          label="备注"
          type="textarea"
          rows="2"
          autosize
          placeholder="备注说明"
        />
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
