<script setup lang="ts">
import { onMounted, ref } from "vue";
import { DropdownMenu, DropdownItem, showToast } from "vant";
import http from "../../lib/http";
import { errMsg } from "../../lib/errmsg";
import SupportConversation from "../../components/SupportConversation.vue";
import { supportStatusLabel, supportModeLabels, type SupportTicket } from "../../lib/support";

const list = ref<SupportTicket[]>([]);
const status = ref("open");
const mode = ref("queue");
const page = ref(1);
const total = ref(0);
const selected = ref(0);
const loading = ref(false);
const options = [{ text: "待处理", value: "open" }, { text: "已结束", value: "closed" }, { text: "全部", value: "" }];
const modeOptions = [{ text: "人工咨询", value: "queue" }, { text: "等待人工", value: "waiting" }, { text: "人工服务中", value: "human" }, { text: "自动应答", value: "bot" }, { text: "全部咨询", value: "all" }];
async function load() {
  loading.value = true;
  try {
    const r = await http.get("/api/admin/support", { params: { status: status.value, mode: mode.value, page: page.value, pageSize: 20 } });
    list.value = r.data.items || []; total.value = r.data.total;
  } catch (e) { showToast(errMsg(e, "读取咨询失败")); }
  finally { loading.value = false; }
}
onMounted(load);
</script>
<template>
  <div class="admin-page">
    <div class="admin-head"><h2>客服管理</h2><van-button size="small" @click="load">刷新</van-button></div>
    <van-cell title="仅用户发送「联系人工」后进入人工队列" label="自动应答会话可查看记录，不能直接人工回复。" />
    <DropdownMenu>
      <DropdownItem v-model="mode" :options="modeOptions" @change="page = 1; load()" />
      <DropdownItem v-model="status" :options="options" @change="page = 1; load()" />
    </DropdownMenu>
    <van-loading v-if="loading" />
    <table v-else class="admin-table">
      <thead><tr><th>标题</th><th>用户</th><th>接待方式</th><th>状态</th><th>更新时间</th><th>操作</th></tr></thead>
      <tbody><tr v-for="t in list" :key="t.id"><td>{{ t.subject }}</td><td>{{ t.userId }}</td><td>{{ supportModeLabels[t.mode] }}</td><td>{{ supportStatusLabel(t) }}</td><td>{{ new Date(t.updatedAt).toLocaleString() }}</td><td><van-button size="small" @click="selected = t.id">{{ t.mode === 'bot' || t.status === 'closed' ? '查看记录' : '查看 / 回复' }}</van-button></td></tr></tbody>
    </table>
    <van-empty v-if="!loading && !list.length" description="暂无咨询" />
    <div v-if="total > 20" class="pagination"><van-button size="small" :disabled="page <= 1" @click="page--; load()">上一页</van-button><span>{{ page }} / {{ Math.ceil(total / 20) }}</span><van-button size="small" :disabled="page * 20 >= total" @click="page++; load()">下一页</van-button></div>
    <van-popup :show="!!selected" position="right" style="width:min(600px,100vw);height:100%;overflow-y:auto" @update:show="v => { if (!v) selected = 0; }">
      <van-nav-bar title="客服对话" right-text="关闭" @click-right="selected = 0" />
      <SupportConversation v-if="selected" :ticket-id="selected" staff @updated="load" />
    </van-popup>
  </div>
</template>
