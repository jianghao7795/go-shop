<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { showToast } from "vant";
import http from "../../lib/http";
import { errMsg } from "../../lib/errmsg";

const list = ref<any[]>([]);
const total = ref(0);
const page = ref(1);
const pageSize = 20;
const totalPages = computed(() => Math.ceil(total.value / pageSize));

const roles = ref<any[]>([]);
const showAssign = ref(false);
const assignLoading = ref(false);
const savingRoles = ref(false);
const currentUser = ref<any>(null);
const selectedRoleIds = ref<number[]>([]);

async function load() {
  const res = await http.get("/api/admin/users", { params: { page: page.value, pageSize } });
  list.value = res.data.items;
  total.value = res.data.total;
}

function changePage(p: number) {
  page.value = p;
  load();
}

async function toggleStatus(u: any) {
  const next = u.status === 1 ? 0 : 1;
  await http.put("/api/admin/users/" + u.id + "/status", { status: next });
  showToast("已更新");
  load();
}

async function openAssign(u: any) {
  currentUser.value = u;
  showAssign.value = true;
  assignLoading.value = true;
  try {
    const [rolesRes, userRolesRes] = await Promise.all([
      http.get("/api/admin/roles"),
      http.get("/api/admin/users/" + u.id + "/roles"),
    ]);
    roles.value = rolesRes.data;
    selectedRoleIds.value = [...(userRolesRes.data.roleIds || [])];
  } catch {
    showToast("加载失败");
    showAssign.value = false;
  } finally {
    assignLoading.value = false;
  }
}

function toggleRole(id: number) {
  const idx = selectedRoleIds.value.indexOf(id);
  if (idx >= 0) selectedRoleIds.value.splice(idx, 1);
  else selectedRoleIds.value.push(id);
}

async function submitAssign() {
  if (!currentUser.value) return;
  savingRoles.value = true;
  try {
    await http.put("/api/admin/users/" + currentUser.value.id + "/roles", {
      roleIds: [...selectedRoleIds.value],
    });
    showToast("已更新");
    showAssign.value = false;
  } catch (err) {
    showToast(errMsg(err, "保存失败"));
  } finally {
    savingRoles.value = false;
  }
}

onMounted(load);
</script>

<template>
  <div class="admin-page">
    <h2>用户管理</h2>
    <table class="admin-table">
      <thead><tr><th>ID</th><th>用户名</th><th>昵称</th><th>手机号</th><th>状态</th><th>操作</th></tr></thead>
      <tbody>
        <tr v-for="u in list" :key="u.id">
          <td>{{ u.id }}</td><td>{{ u.username }}</td><td>{{ u.nickname || "-" }}</td>
          <td>{{ u.mobile || "-" }}</td>
          <td>{{ u.status === 1 ? "正常" : "禁用" }}</td>
          <td class="admin-ops">
            <van-button size="small" @click="openAssign(u)">分配角色</van-button>
            <van-button size="small" @click="toggleStatus(u)">{{ u.status === 1 ? "禁用" : "启用" }}</van-button>
          </td>
        </tr>
      </tbody>
    </table>
    <div class="pagination" v-if="totalPages > 1">
      <van-button size="small" :disabled="page <= 1" @click="changePage(page - 1)">上一页</van-button>
      <span>{{ page }} / {{ totalPages }}（共 {{ total }} 条）</span>
      <van-button size="small" :disabled="page >= totalPages" @click="changePage(page + 1)">下一页</van-button>
    </div>

    <van-popup v-model:show="showAssign" position="bottom" round>
      <div class="form-popup">
        <h3>分配角色 - {{ currentUser?.username }}</h3>
        <van-loading v-if="assignLoading" class="loading" color="#ff4d67" />
        <div v-else class="role-checks">
          <van-checkbox
            v-for="r in roles"
            :key="r.id"
            :model-value="selectedRoleIds.includes(r.id)"
            @click="toggleRole(r.id)"
          >{{ r.name }}</van-checkbox>
        </div>
        <div class="form-actions">
          <van-button block type="primary" :loading="savingRoles" @click="submitAssign">保存</van-button>
        </div>
      </div>
    </van-popup>
  </div>
</template>

<style scoped>
.role-checks {
  display: flex;
  flex-wrap: wrap;
  gap: 4px 16px;
}
</style>
