import { computed, ref } from "vue";
import { defineStore } from "pinia";
import { isAxiosError } from "axios";
import http from "../lib/http";

const TOKEN_KEY = "shop_token";
const USER_KEY = "shop_user";

export const useUserStore = defineStore("user", () => {
  const token = ref(localStorage.getItem(TOKEN_KEY) || "");
  const username = ref(localStorage.getItem(USER_KEY) || "");
  const isLoggedIn = computed(() => !!token.value);

  function setAuth(newToken: string, name: string) {
    token.value = newToken;
    username.value = name;
    localStorage.setItem(TOKEN_KEY, newToken);
    localStorage.setItem(USER_KEY, name);
  }

  function logout() {
    token.value = "";
    username.value = "";
    localStorage.removeItem(TOKEN_KEY);
    localStorage.removeItem(USER_KEY);
  }

  async function validate() {
    if (!token.value) return;
    try {
      const res = await http.get("/api/me");
      if (res.data.username) username.value = res.data.username;
    } catch (err) {
      // HTTP 错误（含 401）时登出；网络不可用时不强制登出，保留本地登录态
      if (isAxiosError(err) && err.response) logout();
    }
  }

  return { token, username, isLoggedIn, setAuth, logout, validate };
});
