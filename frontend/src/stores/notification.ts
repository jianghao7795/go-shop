import { ref } from "vue";
import { defineStore } from "pinia";
import { showToast } from "vant";
import { useUserStore } from "./user";

export interface AppNotification {
  id: number;
  userId: string;
  type: "order" | "coupon";
  title: string;
  content: string;
  orderNo: string;
  read: boolean;
  createdAt: string;
}

const apiBase = () => import.meta.env.VITE_API_BASE || "http://127.0.0.1:8080";

export const useNotificationStore = defineStore("notification", () => {
  const userStore = useUserStore();
  const items = ref<AppNotification[]>([]);
  const unread = ref(0);
  let source: EventSource | null = null;

  const authHeaders = () => ({ Authorization: "Bearer " + userStore.token });

  async function fetchList() {
    const response = await fetch(apiBase() + "/api/notifications", { headers: authHeaders() });
    if (response.ok) items.value = await response.json();
  }

  async function fetchUnread() {
    const response = await fetch(apiBase() + "/api/notifications/unread", { headers: authHeaders() });
    if (response.ok) {
      const data = await response.json();
      unread.value = data.count || 0;
    }
  }

  async function markRead(id: number) {
    await fetch(apiBase() + "/api/notifications/" + id + "/read", {
      method: "PUT",
      headers: authHeaders(),
    });
    const item = items.value.find(i => i.id === id);
    if (item && !item.read) {
      item.read = true;
      unread.value = Math.max(0, unread.value - 1);
    }
  }

  async function markAllRead() {
    await fetch(apiBase() + "/api/notifications/read-all", {
      method: "PUT",
      headers: authHeaders(),
    });
    items.value.forEach(i => { i.read = true; });
    unread.value = 0;
  }

  function connect() {
    if (!userStore.token || source) return;
    source = new EventSource(apiBase() + "/api/notifications/stream?token=" + encodeURIComponent(userStore.token));
    source.addEventListener("notification", (e: MessageEvent) => {
      try {
        const n = JSON.parse(e.data) as AppNotification;
        items.value.unshift(n);
        unread.value += 1;
        showToast(n.title);
      } catch { /* 忽略无法解析的数据 */ }
    });
    source.onopen = () => { fetchUnread(); };
  }

  function disconnect() {
    if (source) {
      source.close();
      source = null;
    }
  }

  return { items, unread, fetchList, fetchUnread, markRead, markAllRead, connect, disconnect };
});
