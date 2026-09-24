<script setup lang="ts">
import { onMounted, ref } from "vue";
import { showToast } from "vant";
import http from "../../lib/http";

const list = ref<any[]>([]);
async function load() {
  const res = await http.get("/api/admin/users");
  list.value = res.data;
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
  </div>
</template>
