<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { showToast } from "vant";
import http from "../../lib/http";

const list = ref<any[]>([]);
const total = ref(0);
const page = ref(1);
const pageSize = 20;
const totalPages = computed(() => Math.ceil(total.value / pageSize));

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

onMounted(load);
</script>

<template>
  <div class="admin-page">
    <h2>用户管理</h2>
    <table class="admin-table">
      <thead><tr><th>ID</th><th>用户名</th><th>昵称</th><th>手机号</th><th>角色</th><th>状态</th><th>操作</th></tr></thead>
      <tbody>
        <tr v-for="u in list" :key="u.id">
          <td>{{ u.id }}</td><td>{{ u.username }}</td><td>{{ u.nickname || "-" }}</td>
          <td>{{ u.mobile || "-" }}</td><td>{{ u.role }}</td>
          <td>{{ u.status === 1 ? "正常" : "禁用" }}</td>
          <td><van-button size="small" @click="toggleStatus(u)">{{ u.status === 1 ? "禁用" : "启用" }}</van-button></td>
        </tr>
      </tbody>
    </table>
    <div class="pagination" v-if="totalPages > 1">
      <van-button size="small" :disabled="page <= 1" @click="changePage(page - 1)">上一页</van-button>
      <span>{{ page }} / {{ totalPages }}（共 {{ total }} 条）</span>
      <van-button size="small" :disabled="page >= totalPages" @click="changePage(page + 1)">下一页</van-button>
    </div>
  </div>
</template>

<style scoped>
.pagination {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 12px;
  margin-top: 12px;
  color: #646566;
  font-size: 13px;
}
</style>
