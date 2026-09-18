import { computed, ref } from "vue";
import { defineStore } from "pinia";

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
    const apiBase = import.meta.env.VITE_API_BASE || "http://127.0.0.1:8080";
    try {
      const response = await fetch(apiBase + "/api/me", {
        headers: { Authorization: "Bearer " + token.value },
      });
      if (!response.ok) { logout(); return; }
      const data = await response.json();
      if (data.username) username.value = data.username;
    } catch {
      // 网络不可用时不强制登出，保留本地登录态
    }
  }

  return { token, username, isLoggedIn, setAuth, logout, validate };
});
