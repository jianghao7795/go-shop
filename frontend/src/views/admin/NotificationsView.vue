<script setup lang="ts">
import { reactive, ref } from "vue";
import { showToast } from "vant";
import http from "../../lib/http";
import { errMsg } from "../../lib/errmsg";

const sending = ref(false);

const form = reactive({
  username: "",
  title: "",
  content: "",
});

async function send() {
  if (!form.title.trim()) {
    showToast("请输入标题");
    return;
  }
  if (!form.content.trim()) {
    showToast("请输入内容");
    return;
  }
  sending.value = true;
  try {
    await http.post("/api/admin/notifications", {
      username: form.username.trim(),
      title: form.title,
      content: form.content,
    });
    showToast("已发送");
    form.title = "";
    form.content = "";
  } catch (err) {
    showToast(errMsg(err, "发送失败"));
  } finally {
    sending.value = false;
  }
}
</script>

<template>
  <div class="admin-page">
    <h2>通知发送</h2>
    <div class="notify-form">
      <van-field
        v-model="form.username"
        label="用户名"
        placeholder="留空则广播给所有人"
      />
      <van-field v-model="form.title" label="标题" placeholder="通知标题" />
      <van-field
        v-model="form.content"
        label="内容"
        type="textarea"
        rows="4"
        autosize
        placeholder="通知内容"
      />
      <div class="notify-actions">
        <van-button block type="primary" :loading="sending" @click="send">
          发送
        </van-button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.notify-form {
  background: #fff;
  border-radius: 8px;
  overflow: hidden;
}
.notify-actions {
  padding: 16px;
}
</style>
