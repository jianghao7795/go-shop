<script setup lang="ts">
import { computed, ref } from "vue";
import { useRouter } from "vue-router";
import { showToast } from "vant";
import http from "../lib/http";

const router = useRouter();
const username = ref("");
const password = ref("");
const confirm = ref("");
const loading = ref(false);

const passwordRules = [
  { required: true, message: "请输入密码" },
  { validator: (val: string) => val.length >= 6 && val.length <= 20, message: "密码长度需为 6-20 位" },
  { validator: (val: string) => /[A-Za-z]/.test(val) && /\d/.test(val), message: "密码需同时包含字母和数字" },
];

const passwordStrength = computed(() => {
  const val = password.value;
  if (!val) return { score: 0, text: "", color: "#e5e7eb" };
  let score = 0;
  if (val.length >= 6) score += 1;
  if (val.length >= 10) score += 1;
  if (/[A-Za-z]/.test(val) && /\d/.test(val)) score += 1;
  if (/[^A-Za-z0-9]/.test(val)) score += 1;
  if (score <= 1) return { score, text: "弱", color: "#ee0a24" };
  if (score === 2) return { score, text: "中", color: "#ff976a" };
  return { score, text: "强", color: "#07c160" };
});

async function onSubmit() {
  if (password.value !== confirm.value) {
    showToast("两次输入的密码不一致");
    return;
  }
  loading.value = true;
  try {
    await http.post("/api/register", { username: username.value, password: password.value });
    showToast("注册成功，请登录");
    router.replace({ name: "login", query: { username: username.value } });
  } catch (err) {
    const e = err as any;
    if (e?.response) showToast(e.response.data?.message || "注册失败");
    else showToast("注册服务暂不可用，请稍后重试");
  } finally {
    loading.value = false;
  }
}
</script>

<template>
  <div class="login-page">
    <van-nav-bar title="注册" left-arrow @click-left="$router.back()" />
    <div class="login-body">
      <div class="login-logo">🛍️</div>
      <h2>注册优选商城账号</h2>
      <van-form @submit="onSubmit">
        <van-cell-group inset>
          <van-field v-model="username" name="username" label="用户名" placeholder="请输入用户名" clearable :rules="[{ required: true, message: '请输入用户名' }]" />
          <van-field v-model="password" type="password" name="password" label="密码" placeholder="6-20位，含字母和数字" clearable :rules="passwordRules" />
          <van-field v-model="confirm" type="password" name="confirm" label="确认密码" placeholder="请再次输入密码" clearable :rules="[{ required: true, message: '请再次输入密码' }]" />
        </van-cell-group>
        <div class="pwd-strength" v-if="password">
          <span class="pwd-strength-bars">
            <i v-for="n in 3" :key="n" :style="{ background: n <= passwordStrength.score ? passwordStrength.color : '#e5e7eb' }"></i>
          </span>
          <span class="pwd-strength-text" :style="{ color: passwordStrength.color }">密码强度：{{ passwordStrength.text }}</span>
        </div>
        <div class="login-submit">
          <van-button round block type="danger" native-type="submit" :loading="loading">注册</van-button>
        </div>
      </van-form>
      <p class="login-hint">注册成功后即可登录</p>
    </div>
  </div>
</template>
