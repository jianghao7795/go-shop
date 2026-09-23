<script setup lang="ts">
import { ref } from "vue";
import { useRouter } from "vue-router";
import { showToast } from "vant";
import { useUserStore } from "../stores/user";

const router = useRouter();
const userStore = useUserStore();

const nickname = ref(userStore.nickname);
const avatar = ref(userStore.avatar);
const mobile = ref(userStore.mobile);
const email = ref(userStore.email);
const loading = ref(false);

const avatarRule = {
  validator: (val: string) => !val || /^https?:\/\//.test(val),
  message: "头像需是 http(s) 开头的链接",
};
const mobileRule = {
  validator: (val: string) => !val || /^1[3-9]\d{9}$/.test(val),
  message: "手机号格式不正确",
};
const emailRule = {
  validator: (val: string) => !val || /^[^@\s]+@[^@\s]+\.[^@\s]+$/.test(val),
  message: "邮箱格式不正确",
};

async function onSubmit() {
  loading.value = true;
  try {
    await userStore.saveProfile({
      nickname: nickname.value,
      avatar: avatar.value,
      mobile: mobile.value,
      email: email.value,
    });
    showToast("保存成功");
    router.back();
  } catch (err) {
    const e = err as any;
    if (e?.response) showToast(e.response.data?.message || "保存失败");
    else showToast("保存失败，请稍后重试");
  } finally {
    loading.value = false;
  }
}
</script>

<template>
  <div class="sub-page">
    <van-nav-bar title="编辑资料" left-arrow @click-left="$router.back()" />
    <van-form @submit="onSubmit">
      <van-cell-group inset>
        <van-field v-model="nickname" name="nickname" label="昵称" placeholder="请输入昵称" clearable />
        <van-field v-model="avatar" name="avatar" label="头像链接" placeholder="http(s):// 开头的图片链接" clearable :rules="[avatarRule]" />
        <van-field v-model="mobile" name="mobile" label="手机号" placeholder="请输入手机号" clearable :rules="[mobileRule]" />
        <van-field v-model="email" name="email" label="邮箱" placeholder="请输入邮箱" clearable :rules="[emailRule]" />
      </van-cell-group>
      <div class="login-submit">
        <van-button round block type="danger" native-type="submit" :loading="loading">保存</van-button>
      </div>
    </van-form>
  </div>
</template>
