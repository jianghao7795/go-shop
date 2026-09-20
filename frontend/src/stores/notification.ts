import { ref } from "vue";
import { defineStore } from "pinia";
import { showToast } from "vant";
import http from "../lib/http";
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

  async function fetchList() {
    try {
      const res = await http.get("/api/notifications");
      items.value = res.data;
    } catch { /* 忽略加载失败 */ }
  }

  async function fetchUnread() {
    try {
      const res = await http.get("/api/notifications/unread");
      unread.value = res.data.count || 0;
    } catch { /* 忽略加载失败 */ }
  }

  async function markRead(id: number) {
    try {
      await http.put("/api/notifications/" + id + "/read");
    } catch { /* 忽略 */ }
    const item = items.value.find(i => i.id === id);
    if (item && !item.read) {
      item.read = true;
      unread.value = Math.max(0, unread.value - 1);
    }
  }

  async function markAllRead() {
    try {
      await http.put("/api/notifications/read-all");
    } catch { /* 忽略 */ }
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
