<script setup lang="ts">
import { onMounted, ref } from "vue";
import { useNotificationStore } from "../stores/notification";

const store = useNotificationStore();
const loading = ref(false);

const typeEmoji: Record<string, string> = {
  order: "📦",
  coupon: "🎟️",
};

async function load() {
  loading.value = true;
  await store.fetchList();
  await store.fetchUnread();
  loading.value = false;
}

onMounted(load);
</script>

<template>
  <div class="sub-page">
    <van-nav-bar title="消息通知" left-arrow @click-left="$router.back()" />
    <div class="notify-toolbar">
      <van-button v-if="store.unread > 0" size="small" round plain type="danger" @click="store.markAllRead()">全部已读</van-button>
    </div>
    <div v-if="loading" class="loading"><van-loading color="#ff4d67" /></div>
    <van-empty v-else-if="!store.items.length" description="暂无消息" />
    <van-cell-group v-else inset>
      <van-cell v-for="n in store.items" :key="n.id" class="notify-cell" :class="{ 'is-unread': !n.read }" @click="store.markRead(n.id)">
        <template #icon>
          <span class="notify-emoji">{{ typeEmoji[n.type] || "🔔" }}</span>
        </template>
        <template #title>
          <span class="notify-title">{{ n.title }}</span>
          <span v-if="!n.read" class="notify-dot" />
        </template>
        <template #label>{{ n.content }}</template>
        <template #value>
          <span class="notify-time">{{ n.createdAt.slice(5, 16).replace("T", " ") }}</span>
        </template>
      </van-cell>
    </van-cell-group>
  </div>
</template>
