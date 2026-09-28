<script setup lang="ts">
import { onMounted, reactive, ref } from "vue";
import { showConfirmDialog, showToast } from "vant";
import http from "../../lib/http";
import { errMsg } from "../../lib/errmsg";

interface Role {
  id: number;
  name: string;
  description: string;
  permissions: string[];
}

const PERMISSIONS = [
  { key: "product:manage", label: "商品" },
  { key: "category:manage", label: "分类" },
  { key: "order:manage", label: "订单" },
  { key: "user:manage", label: "用户" },
  { key: "coupon:manage", label: "优惠券" },
  { key: "notification:manage", label: "通知" },
];

const permLabels: Record<string, string> = Object.fromEntries(
  PERMISSIONS.map((p) => [p.key, p.label])
);

const list = ref<Role[]>([]);
const loading = ref(false);
const saving = ref(false);
const showForm = ref(false);
const editingId = ref<number | null>(null);

const form = reactive({ name: "", description: "", permissions: [] as string[] });

async function load() {
  loading.value = true;
  try {
    const res = await http.get("/api/admin/roles");
    list.value = res.data;
  } catch {
    showToast("加载失败");
  } finally {
    loading.value = false;
  }
}

function isBuiltin(r: Role) {
  return r.name === "admin" || r.name === "customer";
}

function openCreate() {
  editingId.value = null;
  Object.assign(form, { name: "", description: "", permissions: [] });
  showForm.value = true;
}

function openEdit(r: Role) {
  editingId.value = r.id;
  Object.assign(form, {
    name: r.name,
    description: r.description,
    permissions: [...(r.permissions || [])],
  });
  showForm.value = true;
}

function togglePerm(key: string) {
  const idx = form.permissions.indexOf(key);
  if (idx >= 0) form.permissions.splice(idx, 1);
  else form.permissions.push(key);
}

async function submit() {
  saving.value = true;
  try {
    const body = {
      name: form.name,
      description: form.description,
      permissions: [...form.permissions],
    };
    if (editingId.value) {
      await http.put("/api/admin/roles/" + editingId.value, body);
    } else {
      await http.post("/api/admin/roles", body);
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

async function remove(r: Role) {
  try {
    await showConfirmDialog({
      title: "删除角色",
      message: `确定删除「${r.name}」吗？`,
    });
  } catch {
    return;
  }
  try {
    await http.delete("/api/admin/roles/" + r.id);
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
      <h2>角色管理</h2>
      <van-button size="small" type="primary" @click="openCreate">新建</van-button>
    </div>

    <van-loading v-if="loading" class="loading" color="#ff4d67" />

    <table v-else class="admin-table">
      <thead>
        <tr><th>名称</th><th>描述</th><th>权限</th><th>操作</th></tr>
      </thead>
      <tbody>
        <tr v-for="r in list" :key="r.id">
          <td>{{ r.name }}</td>
          <td>{{ r.description || "-" }}</td>
          <td>
            <span
              v-for="p in r.permissions"
              :key="p"
              class="perm-tag"
            >{{ permLabels[p] || p }}</span>
            <span v-if="!r.permissions || r.permissions.length === 0">-</span>
          </td>
          <td class="admin-ops">
            <van-button size="small" :disabled="isBuiltin(r)" @click="openEdit(r)">编辑</van-button>
            <van-button size="small" type="danger" :disabled="isBuiltin(r)" @click="remove(r)">删除</van-button>
          </td>
        </tr>
      </tbody>
    </table>

    <van-popup v-model:show="showForm" position="bottom" round>
      <div class="form-popup">
        <h3>{{ editingId ? "编辑角色" : "新建角色" }}</h3>
        <van-field v-model="form.name" label="名称" placeholder="角色名称" />
        <van-field
          v-model="form.description"
          label="描述"
          type="textarea"
          rows="2"
          autosize
          placeholder="角色描述"
        />
        <van-field label="权限">
          <template #input>
            <div class="perm-checks">
              <van-checkbox
                v-for="p in PERMISSIONS"
                :key="p.key"
                :model-value="form.permissions.includes(p.key)"
                @click="togglePerm(p.key)"
              >{{ p.label }}</van-checkbox>
            </div>
          </template>
        </van-field>
        <div class="form-actions">
          <van-button block type="primary" :loading="saving" @click="submit">保存</van-button>
        </div>
      </div>
    </van-popup>
  </div>
</template>

<style scoped>
.perm-tag {
  display: inline-block;
  margin: 0 4px 4px 0;
  padding: 2px 8px;
  border-radius: 10px;
  background: #ffeef0;
  color: #ff4d67;
  font-size: 12px;
  white-space: nowrap;
}

.perm-checks {
  display: flex;
  flex-wrap: wrap;
  gap: 4px 16px;
}
</style>
