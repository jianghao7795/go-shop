<script setup lang="ts">
import { ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { showToast } from "vant";
import { useUserStore } from "../stores/user";
import http from "../lib/http";

const REMEMBER_KEY = "shop_remember";

const route = useRoute();
const router = useRouter();
const userStore = useUserStore();

const username = ref("");
const password = ref("");
const remember = ref(false);
const loading = ref(false);

// 预填：上次勾选「记住密码」保存的账号，或从注册页带过来的用户名
const saved = localStorage.getItem(REMEMBER_KEY);
if (saved) {
  try {
    const parsed = JSON.parse(saved) as { username?: string; password?: string };
    username.value = parsed.username || "";
    password.value = parsed.password || "";
    remember.value = true;
  } catch { /* 忽略损坏的缓存 */ }
}
if (route.query.username) username.value = String(route.query.username);

async function onSubmit() {
  loading.value = true;
  try {
    const res = await http.post("/api/login", { username: username.value, password: password.value });
    if (remember.value) {
      localStorage.setItem(REMEMBER_KEY, JSON.stringify({ username: username.value, password: password.value }));
    } else {
      localStorage.removeItem(REMEMBER_KEY);
    }
    userStore.setAuth(res.data.token, username.value);
    showToast("登录成功");
    router.replace(String(route.query.redirect || "/profile"));
  } catch (err) {
    const e = err as any;
    if (e?.response) showToast(e.response.data?.message || "账号或密码错误");
    else showToast("登录服务暂不可用，请稍后重试");
  } finally {
    loading.value = false;
  }
}
</script>

<template>
  <div class="login-page">
    <van-nav-bar title="登录" left-arrow @click-left="$router.back()" />
    <div class="login-body">
      <div class="login-logo">🛍️</div>
      <h2>欢迎登录优选商城</h2>
      <van-form @submit="onSubmit">
        <van-cell-group inset>
          <van-field v-model="username" name="username" label="用户名" placeholder="请输入用户名" clearable :rules="[{ required: true, message: '请输入用户名' }]" />
          <van-field v-model="password" type="password" name="password" label="密码" placeholder="请输入密码" clearable :rules="[{ required: true, message: '请输入密码' }]" />
        </van-cell-group>
        <div class="login-options">
          <van-checkbox v-model="remember">记住密码</van-checkbox>
          <router-link to="/register" class="login-register">没有账号？去注册</router-link>
        </div>
        <div class="login-submit">
          <van-button round block type="danger" native-type="submit" :loading="loading">登录</van-button>
        </div>
      </van-form>
      <p class="login-hint">请使用已注册的账号登录</p>
    </div>
  </div>
</template>
