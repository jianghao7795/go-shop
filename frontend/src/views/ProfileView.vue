<script setup lang="ts">
import { useRouter } from "vue-router";
import { showToast } from "vant";
import { useUserStore } from "../stores/user";
import { useNotificationStore } from "../stores/notification";

const router = useRouter();
const userStore = useUserStore();
const notificationStore = useNotificationStore();

function goLogin() {
  router.push({ name: "login", query: { redirect: "/profile" } });
}

function logout() {
  userStore.logout();
  showToast("已退出登录");
}

function goOrders(status = "") {
  router.push({ name: "orders", query: status ? { status } : {} });
}
</script>

<template>
  <div class="sub-page profile-page">
    <van-nav-bar title="个人中心" />
    <section class="profile-header">
      <div class="avatar">
        <img v-if="userStore.avatar" :src="userStore.avatar" alt="头像" />
        <span v-else>👤</span>
      </div>
      <div>
        <h2>{{ userStore.isLoggedIn ? userStore.displayName : "优选用户" }}</h2>
        <p>
          {{ userStore.isLoggedIn ? "欢迎回来" : "登录后享受更多会员权益" }}
        </p>
      </div>
      <van-button
        v-if="!userStore.isLoggedIn"
        round
        plain
        type="danger"
        size="small"
        @click="goLogin"
        >立即登录</van-button
      >
      <van-button v-else round plain size="small" @click="logout"
        >退出登录</van-button
      >
    </section>
    <van-cell-group inset>
      <van-cell title="我的订单" value="查看全部" is-link @click="goOrders()" />
      <van-grid :column-num="4" :border="false">
        <van-grid-item
          icon="pending-payment"
          text="待付款"
          @click="goOrders('pending')"
        />
        <van-grid-item
          icon="logistics"
          text="待收货"
          @click="goOrders('shipped')"
        />
        <van-grid-item
          icon="comment-o"
          text="待评价"
          @click="goOrders('completed')"
        />
        <van-grid-item
          icon="after-sale"
          text="售后"
          @click="goOrders('aftersale')"
        />
      </van-grid>
    </van-cell-group>
    <van-cell-group inset class="profile-tools">
      <van-cell
        title="编辑资料"
        icon="edit"
        is-link
        @click="router.push('/profile/edit')"
      />
      <van-cell
        title="优惠券"
        icon="coupon-o"
        is-link
        @click="router.push('/coupons')"
      />
      <van-cell
        title="收货地址"
        icon="location-o"
        is-link
        @click="router.push('/address')"
      />
      <van-cell
        title="消息通知"
        icon="bell"
        is-link
        @click="router.push('/notifications')"
      >
        <template #value>
          <van-badge
            v-if="notificationStore.unread > 0"
            :content="notificationStore.unread"
            style="margin-top: 13px; margin-right: 15px"
          />
        </template>
      </van-cell>
      <van-cell
        title="客服与帮助"
        icon="service-o"
        is-link
        @click="router.push('/service')"
      />
    </van-cell-group>
  </div>
</template>

<style scoped>
.profile-header .avatar img {
  width: 100%;
  height: 100%;
  border-radius: 50%;
  object-fit: cover;
}
</style>
