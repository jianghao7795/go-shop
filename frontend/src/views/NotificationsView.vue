<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { showConfirmDialog, showToast } from "vant";
import { useNotificationStore } from "../stores/notification";

const store = useNotificationStore();
const loading = ref(false);
const activeType = ref("");
const onlyUnread = ref(false);
const detail = ref<any>(null);
const showDetail = ref(false);

const typeEmoji: Record<string, string> = {
  order: "📦",
  coupon: "🎟️",
};

const filteredItems = computed(() => {
  return store.items.filter((n) => {
    if (activeType.value && n.type !== activeType.value) return false;
    if (onlyUnread.value && n.read) return false;
    return true;
  });
});

async function load() {
  loading.value = true;
  await store.fetchList();
  await store.fetchUnread();
  loading.value = false;
}

function openDetail(n: any) {
  detail.value = n;
  showDetail.value = true;
  if (!n.read) store.markRead(n.id);
}

async function clearAll() {
  try {
    await showConfirmDialog({ title: "清空消息", message: "确定清空全部消息吗？" });
  } catch {
    return;
  }
  await store.clearAll();
  showToast("已清空");
}

onMounted(load);
</script>

<template>
  <div class="sub-page">
    <van-nav-bar title="消息中心" left-arrow @click-left="$router.back()" />
    <van-tabs v-model:active="activeType" sticky>
      <van-tab title="全部" name="" />
      <van-tab title="订单" name="order" />
      <van-tab title="优惠券" name="coupon" />
    </van-tabs>
    <div class="notify-toolbar">
      <van-checkbox v-model="onlyUnread">只看未读</van-checkbox>
      <div class="notify-toolbar-btns">
        <van-button v-if="store.unread > 0" size="small" round plain type="danger" @click="store.markAllRead()">全部已读</van-button>
        <van-button v-if="store.items.length" size="small" round plain @click="clearAll">清空</van-button>
      </div>
    </div>
    <div v-if="loading" class="loading"><van-loading color="#ff4d67" /></div>
    <van-empty v-else-if="!filteredItems.length" description="暂无消息" />
    <van-cell-group v-else inset>
      <van-swipe-cell v-for="n in filteredItems" :key="n.id">
        <van-cell class="notify-cell" :class="{ 'is-unread': !n.read }" @click="openDetail(n)">
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
        <template #right>
          <van-button square type="danger" text="删除" class="notify-del" @click="store.remove(n.id)" />
        </template>
      </van-swipe-cell>
    </van-cell-group>

    <van-dialog v-model:show="showDetail" :title="detail?.title" cancel-button-text="关闭" @cancel="showDetail = false">
      <div class="notify-detail">
        <div class="notify-detail-meta">{{ typeEmoji[detail?.type] || "🔔" }} {{ detail?.createdAt?.slice(5, 16).replace("T", " ") }}</div>
        <p>{{ detail?.content }}</p>
      </div>
    </van-dialog>
  </div>
</template>

<style scoped>
.notify-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 16px;
}
.notify-toolbar-btns {
  display: flex;
  gap: 8px;
}
.notify-cell.is-unread {
  background: #f7f8ff;
}
.notify-dot {
  display: inline-block;
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: #ff4d67;
  margin-left: 6px;
}
.notify-emoji {
  font-size: 20px;
  margin-right: 8px;
}
.notify-del {
  height: 100%;
}
.notify-detail {
  padding: 16px 24px;
}
.notify-detail-meta {
  margin-bottom: 10px;
  font-size: 12px;
  color: #969ba5;
}
.notify-detail p {
  margin: 0;
  font-size: 14px;
  color: #1f2430;
  line-height: 1.6;
}
</style>
