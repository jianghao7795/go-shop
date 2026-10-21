<script setup lang="ts">
import { ref, watch } from "vue";
import { showToast } from "vant";
import http from "../lib/http";
import { errMsg } from "../lib/errmsg";
import type { Order } from "../lib/orders";

const props = defineProps<{ order: Order | null }>();
const emit = defineEmits<{ (e: "close"): void; (e: "submitted"): void }>();
const type = ref("refund_only");
const reason = ref("");
const submitting = ref(false);
watch(() => props.order?.id, () => { type.value = "refund_only"; reason.value = ""; });

async function submit() {
  if (!props.order || submitting.value) return;
  if (reason.value.trim().length < 2) { showToast("请填写至少 2 字售后原因"); return; }
  submitting.value = true;
  try {
    await http.post(`/api/orders/${props.order.id}/aftersale`, { type: type.value, reason: reason.value.trim() });
    showToast("售后申请已提交");
    emit("submitted");
    emit("close");
  } catch (e) { showToast(errMsg(e, "提交失败")); }
  finally { submitting.value = false; }
}
</script>

<template>
  <van-popup :show="!!order" position="bottom" round @update:show="v => { if (!v && !submitting) emit('close'); }">
    <van-form @submit="submit">
      <van-nav-bar title="申请售后" right-text="关闭" @click-right="emit('close')" />
      <van-cell title="订单号" :value="order?.orderNo" />
      <van-cell title="售后类型">
        <template #value>
          <label><input v-model="type" type="radio" value="refund_only" />仅退款</label>
          <label v-if="order?.status !== 'paid'" style="margin-left:12px"><input v-model="type" type="radio" value="return_refund" />退货退款</label>
        </template>
      </van-cell>
      <van-field v-model="reason" label="申请原因" type="textarea" rows="3" maxlength="500" show-word-limit placeholder="请描述商品或订单问题" :rules="[{ required: true, message: '请填写售后原因' }]" />
      <van-cell title="退款金额" :value="'¥' + (order?.amount || 0).toFixed(2)" label="支持整单退款，申请后等待商家审核" />
      <div style="padding:16px"><van-button block round type="danger" native-type="submit" :loading="submitting">提交申请</van-button></div>
    </van-form>
  </van-popup>
</template>
