<script setup lang="ts">
import { computed, nextTick, onUnmounted, ref, watch } from "vue";
import { showConfirmDialog, showToast } from "vant";
import http from "../lib/http";
import { errMsg } from "../lib/errmsg";
import { supportStatusLabel, type SupportTicket, type SupportMessage } from "../lib/support";

const props = defineProps<{ ticketId: number; staff?: boolean }>();
const emit = defineEmits<{ (e: "updated"): void }>();
const ticket = ref<SupportTicket | null>(null);
const messages = ref<SupportMessage[]>([]);
const messageList = ref<HTMLDivElement | null>(null);
const canReply = computed(() => ticket.value?.status === "open" && (!props.staff || ticket.value.mode !== "bot"));
const hint = computed(() => {
  if (ticket.value?.mode === "bot") return props.staff ? "用户尚未请求人工客服，此会话由自动客服接待。" : "自动客服为你解答，发送「联系人工」即可转接。";
  if (ticket.value?.mode === "waiting") return props.staff ? "用户已请求人工帮助，回复后开始人工接待。" : "已进入人工客服队列，你可以继续补充问题。";
  return "人工客服正在接待本次咨询。";
});
const content = ref("");
const busy = ref(false);
const error = ref("");
let timer: number | undefined;
let generation = 0;
let requestSequence = 0;
const base = () => props.staff ? "/api/admin/support" : "/api/support";

async function load() {
  const version = generation;
  const sequence = ++requestSequence;
  if (!props.ticketId) return;
  try {
    const r = await http.get(`${base()}/${props.ticketId}`);
    if (version !== generation || sequence !== requestSequence) return;
    const lastID = messages.value[messages.value.length - 1]?.id;
    ticket.value = r.data.ticket; messages.value = r.data.messages || []; error.value = "";
    if (lastID !== messages.value[messages.value.length - 1]?.id) {
      await nextTick();
      if (messageList.value) messageList.value.scrollTop = messageList.value.scrollHeight;
    }
  } catch (e) { if (version === generation && sequence === requestSequence) error.value = errMsg(e, "读取消息失败"); }
}

async function send(close = false, text?: string) {
  if (!ticket.value || !canReply.value || busy.value) return;
  const id = props.ticketId;
  const version = generation;
  const body = close ? "" : (text ?? content.value).trim();
  if (close) { try { await showConfirmDialog({ title: "结束咨询", message: "结束后可重新发起咨询，确认结束吗？" }); } catch { return; } }
  else if (!body) { showToast("请填写消息"); return; }
  if (version !== generation || busy.value) return;
  busy.value = true;
  requestSequence++; // discard a poll started before this message
  try {
    await http.post(`${base()}/${id}/messages`, { content: body, close });
    if (version !== generation) return;
    if (text === undefined) content.value = "";
    await load(); emit("updated");
  } catch (e) { if (version === generation) showToast(errMsg(e, "发送失败")); }
  finally { if (version === generation) busy.value = false; }
}

watch(() => props.ticketId, () => {
  generation++; requestSequence++; busy.value = false; content.value = ""; ticket.value = null; messages.value = []; error.value = "";
  if (timer) window.clearInterval(timer);
  load();
  timer = window.setInterval(() => { if (document.visibilityState === "visible" && ticket.value?.status === "open" && !busy.value) load(); }, 5000);
}, { immediate: true });
onUnmounted(() => { generation++; if (timer) window.clearInterval(timer); });
</script>

<template>
  <div class="conversation">
    <van-cell v-if="ticket" :title="ticket.subject" :value="supportStatusLabel(ticket)" :label="staff ? '用户：' + ticket.userId : ''" />
    <van-cell v-if="ticket?.status === 'open'" :title="hint" />
    <van-cell v-if="error" :title="error" is-link value="重试" @click="load" />
    <div ref="messageList" class="messages" role="log" aria-label="咨询记录" aria-live="polite">
      <article v-for="m in messages" :key="m.id" class="message" :class="{ staff: m.staff, bot: m.bot }">
        <small>{{ m.bot ? '自动客服' : m.staff ? '人工客服' : '买家' }} · {{ new Date(m.createdAt).toLocaleString() }}</small>
        <p>{{ m.content }}</p>
      </article>
    </div>
    <van-form v-if="canReply" @submit="send(false)">
      <van-field v-model="content" type="textarea" rows="3" maxlength="2000" show-word-limit label="消息" :placeholder="staff ? '填写给买家的回复' : ticket?.mode === 'bot' ? '请输入问题，需要人工时发送「联系人工」' : '可以继续补充问题，人工回复后会通知你'" />
      <div class="actions">
        <van-button native-type="submit" type="primary" :loading="busy">发送消息</van-button>
        <van-button v-if="!staff && ticket?.mode === 'bot'" native-type="button" :disabled="busy" @click="send(false, '联系人工')">联系人工</van-button>
        <van-button native-type="button" :disabled="busy" @click="send(true)">结束咨询</van-button>
      </div>
    </van-form>
  </div>
</template>

<style scoped>
.messages { padding: 12px; max-height: 50vh; overflow-y: auto; }
.message { background: #f5f6f8; border-radius: 10px; padding: 12px; margin-bottom: 10px; }
.message.staff { background: #edf7ff; }
.message.bot { background: #f1f8ed; }
.message small { color: #777; }
.message p { margin: 8px 0 0; white-space: pre-wrap; overflow-wrap: anywhere; }
.actions { display: flex; flex-wrap: wrap; gap: 12px; padding: 16px; }
</style>
