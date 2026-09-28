<script setup lang="ts">
import { onMounted, reactive, ref } from "vue";
import { showConfirmDialog, showToast } from "vant";
import http from "../../lib/http";
import { errMsg } from "../../lib/errmsg";

interface Permission {
  id: number;
  code: string;
  name: string;
  path: string;
}

const list = ref<Permission[]>([]);
const loading = ref(false);
const saving = ref(false);
const showForm = ref(false);

const form = reactive({ code: "", name: "", path: "" });

async function load() {
  loading.value = true;
  try {
    const res = await http.get("/api/admin/permissions");
    list.value = res.data;
  } catch {
    showToast("加载失败");
  } finally {
    loading.value = false;
  }
}

function openCreate() {
  Object.assign(form, { code: "", name: "", path: "" });
  showForm.value = true;
}

async function submit() {
  if (!form.code.trim()) {
    showToast("请输入编码");
    return;
  }
  if (!form.name.trim()) {
    showToast("请输入显示名");
    return;
  }
  saving.value = true;
  try {
    await http.post("/api/admin/permissions", {
      code: form.code.trim(),
      name: form.name.trim(),
      path: form.path.trim(),
    });
    showToast("已保存");
    showForm.value = false;
    load();
  } catch (err) {
    showToast(errMsg(err, "保存失败"));
  } finally {
    saving.value = false;
  }
}

async function remove(p: Permission) {
  try {
    await showConfirmDialog({
      title: "删除权限",
      message: `确定删除「${p.name}」吗？`,
    });
  } catch {
    return;
  }
  try {
    await http.delete("/api/admin/permissions/" + p.id);
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
      <h2>权限管理</h2>
      <van-button size="small" type="primary" @click="openCreate">新建</van-button>
    </div>

    <van-loading v-if="loading" class="loading" color="#ff4d67" />
    <van-empty v-else-if="!list.length" description="暂无权限" />

    <table v-else class="admin-table">
      <thead>
        <tr><th>编码</th><th>显示名</th><th>路由</th><th>操作</th></tr>
      </thead>
      <tbody>
        <tr v-for="p in list" :key="p.id">
          <td>{{ p.code }}</td>
          <td>{{ p.name }}</td>
          <td>{{ p.path || "-" }}</td>
          <td class="admin-ops">
            <van-button size="small" type="danger" @click="remove(p)">删除</van-button>
          </td>
        </tr>
      </tbody>
    </table>

    <van-popup v-model:show="showForm" position="bottom" round>
      <div class="form-popup">
        <h3>新建权限</h3>
        <van-field v-model="form.code" label="编码" placeholder="如 product:manage" />
        <van-field v-model="form.name" label="显示名" placeholder="如 商品管理" />
        <van-field v-model="form.path" label="路由" placeholder="如 /admin/products" />
        <div class="form-actions">
          <van-button block type="primary" :loading="saving" @click="submit">保存</van-button>
        </div>
      </div>
    </van-popup>
  </div>
</template>
