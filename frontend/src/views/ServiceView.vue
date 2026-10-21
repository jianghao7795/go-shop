<script setup lang="ts">
import { onMounted, ref } from "vue";
import { showToast } from "vant";
import http from "../lib/http";
import { errMsg } from "../lib/errmsg";
import SupportConversation from "../components/SupportConversation.vue";

const faqs = [
  { q: "如何查询我的订单？", a: "进入「我的」-「我的订单」，可查看全部订单及各状态订单。" },
  { q: "如何修改收货地址？", a: "进入「我的」-「收货地址」，可新增、编辑、删除或设为默认地址。" },
  { q: "如何查看物流？", a: "商家发货后，在订单详情中查看快递公司、单号与商家更新的进度。" },
  { q: "如何申请售后？", a: "在订单详情中点击「申请售后」，填写原因。退货审核通过后，请按退货地址寄回并填写运单。" },
];

const list = ref<{ id: number; subject: string; status: string }[]>([]);
const total = ref(0);
const page = ref(1);
const loading = ref(false);
const error = ref("");
const selected = ref(0);
const creating = ref(false);
const subject = ref("");
const content = ref("");
const busy = ref(false);

async function load() {
  loading.value = true;
  try {
    const r = await http.get("/api/support", { params: { page: page.value, pageSize: 20 } });
    list.value = r.data.items || []; total.value = r.data.total; error.value = "";
  } catch (e) { error.value = errMsg(e, "读取咨询失败"); }
  finally { loading.value = false; }
}
async function create() {
  if (busy.value) return;
  busy.value = true;
  try {
    const r = await http.post("/api/support", { subject: subject.value.trim(), content: content.value.trim() });
    creating.value = false; subject.value = ""; content.value = ""; selected.value = r.data.id; page.value = 1; await load();
    showToast("咨询已提交，客服回复后会通知你");
  } catch (e) { showToast(errMsg(e, "提交失败")); }
  finally { busy.value = false; }
}
onMounted(load);
</script>

<template>
  <div class="sub-page">
    <van-nav-bar title="客服与帮助" left-arrow @click-left="$router.back()" />
    <van-cell-group inset class="service-list">
      <van-cell v-for="faq in faqs" :key="faq.q" :title="faq.q" :label="faq.a" />
    </van-cell-group>
    <div class="service-contact">
      <van-button block round type="danger" @click="creating = true">发起客服咨询</van-button>
    </div>
    <van-cell title="我的咨询" value="刷新" is-link @click="load" />
    <van-cell v-if="error" :title="error" value="重试" is-link @click="load" />
    <van-loading v-if="loading" />
    <van-empty v-else-if="!list.length && !error" description="暂无咨询" />
    <van-cell v-for="t in list" :key="t.id" :title="t.subject" :value="t.status === 'open' ? '处理中' : '已结束'" is-link @click="selected = t.id" />
    <div v-if="total > 20" style="display:flex;gap:16px;justify-content:center;padding:16px">
      <van-button size="small" :disabled="page <= 1" @click="page--; load()">上一页</van-button><span>{{ page }}</span><van-button size="small" :disabled="page * 20 >= total" @click="page++; load()">下一页</van-button>
    </div>
    <van-popup v-model:show="creating" position="bottom" round :close-on-click-overlay="!busy">
      <van-nav-bar title="发起咨询" right-text="关闭" @click-right="!busy && (creating = false)" />
      <van-form @submit="create">
        <van-field v-model="subject" label="标题" maxlength="100" :rules="[{ required: true, message: '请填写标题' }]" />
        <van-field v-model="content" label="问题描述" type="textarea" rows="4" maxlength="2000" show-word-limit :rules="[{ required: true, message: '请描述问题' }]" />
        <div style="padding:16px"><van-button block type="primary" native-type="submit" :loading="busy">提交咨询</van-button></div>
      </van-form>
    </van-popup>
    <van-popup :show="!!selected" position="bottom" round style="height:85%;overflow-y:auto" @update:show="v => { if (!v) selected = 0; }">
      <van-nav-bar title="客服咨询" right-text="关闭" @click-right="selected = 0" />
      <SupportConversation v-if="selected" :ticket-id="selected" @updated="load" />
    </van-popup>
  </div>
</template>
