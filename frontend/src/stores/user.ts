import { computed, ref } from "vue";
import { defineStore } from "pinia";
import { isAxiosError } from "axios";
import http from "../lib/http";

const TOKEN_KEY = "shop_token";
const USER_KEY = "shop_user";

export interface Profile {
  nickname: string;
  avatar: string;
  mobile: string;
  email: string;
}

export const useUserStore = defineStore("user", () => {
  const token = ref(localStorage.getItem(TOKEN_KEY) || "");
  const username = ref(localStorage.getItem(USER_KEY) || "");
  const nickname = ref("");
  const avatar = ref("");
  const mobile = ref("");
  const email = ref("");
  const role = ref("");
  const isLoggedIn = computed(() => !!token.value);
  const displayName = computed(() => nickname.value || username.value);

  function applyProfile(p: any) {
    if (p.username) username.value = p.username;
    nickname.value = p.nickname || "";
    avatar.value = p.avatar || "";
    mobile.value = p.mobile || "";
    email.value = p.email || "";
    role.value = p.role || "customer";
  }

  function setAuth(newToken: string, name: string) {
    token.value = newToken;
    username.value = name;
    localStorage.setItem(TOKEN_KEY, newToken);
    localStorage.setItem(USER_KEY, name);
  }

  function logout() {
    token.value = "";
    username.value = "";
    nickname.value = "";
    avatar.value = "";
    mobile.value = "";
    email.value = "";
    role.value = "";
    localStorage.removeItem(TOKEN_KEY);
    localStorage.removeItem(USER_KEY);
  }

  async function validate() {
    if (!token.value) return;
    try {
      const res = await http.get("/api/me");
      applyProfile(res.data);
    } catch (err) {
      // HTTP 错误（含 401）时登出；网络不可用时不强制登出，保留本地登录态
      if (isAxiosError(err) && err.response) logout();
    }
  }

  async function saveProfile(data: Profile) {
    const res = await http.put("/api/profile", data);
    applyProfile(res.data);
  }

  return { token, username, nickname, avatar, mobile, email, role, isLoggedIn, displayName, setAuth, logout, validate, saveProfile };
});
