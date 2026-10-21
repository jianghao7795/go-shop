<script setup lang="ts">
import { onUnmounted, ref, watch } from "vue";
import { showConfirmDialog, showToast } from "vant";
import http from "../lib/http";
import { errMsg } from "../lib/errmsg";

const props = defineProps<{ ticketId: number; staff?: boolean }>();
const emit = defineEmits<{ (e: "updated"): void }>();
const ticket = ref<{ id: number; subject: string; status: string; userId: string } | null>(null);
const messages = ref<{ id: number; staff: boolean; content: string; createdAt: string }[]>([]);
const content = ref("");
const busy = ref(false);
const error = ref("");
let timer: number | undefined;
let generation = 0;
const base = () => props.staff ? "/api/admin/support" : "/api/support";

async function load() {
  const version = generation;
  if (!props.ticketId) return;
  try {
    const r = await http.get(`${base()}/${props.ticketId}`);
    if (version !== generation) return;
    ticket.value = r.data.ticket; messages.value = r.data.messages || []; error.value = "";
  } catch (e) { if (version === generation) error.value = errMsg(e, "读取消息失败"); }
}

async function send(close = false) {
  if (!ticket.value || busy.value) return;
  if (close) { try { await showConfirmDialog({ title: "结束咨询", message: "结束后可重新发起咨询，确认结束吗？" }); } catch { return; } }
  else if (!content.value.trim()) { showToast("请填写消息"); return; }
  busy.value = true;
  try {
    await http.post(`${base()}/${props.ticketId}/messages`, { content: close ? "" : content.value.trim(), close });
    content.value = ""; await load(); emit("updated");
  } catch (e) { showToast(errMsg(e, "发送失败")); }
  finally { busy.value = false; }
}

watch(() => props.ticketId, () => {
  generation++; content.value = ""; ticket.value = null; messages.value = []; error.value = "";
  if (timer) window.clearInterval(timer);
  load();
  timer = window.setInterval(() => { if (document.visibilityState === "visible" && ticket.value?.status === "open" && !busy.value) load(); }, 5000);
}, { immediate: true });
onUnmounted(() => { generation++; if (timer) window.clearInterval(timer); });
</script>

<template>
  <div class="conversation">
    <van-cell v-if="ticket" :title="ticket.subject" :value="ticket.status === 'open' ? '处理中' : '已结束'" :label="staff ? '用户：' + ticket.userId : ''" />
    <van-cell v-if="error" :title="error" is-link value="重试" @click="load" />
    <div class="messages" role="log" aria-label="咨询记录" aria-live="polite">
      <article v-for="m in messages" :key="m.id" class="message" :class="{ staff: m.staff }">
        <small>{{ m.staff ? '客服' : '买家' }} · {{ new Date(m.createdAt).toLocaleString() }}</small>
        <p>{{ m.content }}</p>
      </article>
    </div>
    <van-form v-if="ticket?.status === 'open'" @submit="send(false)">
      <van-field v-model="content" type="textarea" rows="3" maxlength="2000" show-word-limit label="消息" placeholder="请描述问题，客服回复后会通知你" />
      <div class="actions"><van-button native-type="submit" type="primary" :loading="busy">发送消息</van-button><van-button :disabled="busy" @click="send(true)">结束咨询</van-button></div>
    </van-form>
  </div>
</template>

<style scoped>
.messages { padding: 12px; max-height: 50vh; overflow-y: auto; }
.message { background: #f5f6f8; border-radius: 10px; padding: 12px; margin-bottom: 10px; }
.message.staff { background: #edf7ff; }
.message small { color: #777; }
.message p { margin: 8px 0 0; white-space: pre-wrap; overflow-wrap: anywhere; }
.actions { display: flex; gap: 12px; padding: 16px; }
</style>
