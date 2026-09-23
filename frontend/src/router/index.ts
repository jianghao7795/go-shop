import { createRouter, createWebHashHistory } from "vue-router";
import { useUserStore } from "../stores/user";

const router = createRouter({
  history: createWebHashHistory(),
  scrollBehavior: () => ({ top: 0 }),
  routes: [
    { path: "/", name: "home", component: () => import("../views/HomeView.vue"), meta: { title: "优选商城" } },
    { path: "/category", name: "category", component: () => import("../views/CategoryView.vue"), meta: { title: "商品分类" } },
    { path: "/cart", name: "cart", component: () => import("../views/CartView.vue"), meta: { title: "购物车" } },
    { path: "/profile", name: "profile", component: () => import("../views/ProfileView.vue"), meta: { title: "个人中心", requiresAuth: true } },
    { path: "/profile/edit", name: "profile-edit", component: () => import("../views/ProfileEditView.vue"), meta: { title: "编辑资料", hideTabbar: true, requiresAuth: true } },
    { path: "/product/:id", name: "product-detail", component: () => import("../views/ProductDetailView.vue"), meta: { title: "商品详情", hideTabbar: true } },
    { path: "/login", name: "login", component: () => import("../views/LoginView.vue"), meta: { title: "登录", hideTabbar: true } },
    { path: "/register", name: "register", component: () => import("../views/RegisterView.vue"), meta: { title: "注册", hideTabbar: true } },
    { path: "/orders", name: "orders", component: () => import("../views/OrderListView.vue"), meta: { title: "我的订单", hideTabbar: true, requiresAuth: true } },
    { path: "/orders/:id", name: "order-detail", component: () => import("../views/OrderDetailView.vue"), meta: { title: "订单详情", hideTabbar: true, requiresAuth: true } },
    { path: "/address", name: "address", component: () => import("../views/AddressListView.vue"), meta: { title: "收货地址", hideTabbar: true, requiresAuth: true } },
    { path: "/address/edit", name: "address-edit", component: () => import("../views/AddressEditView.vue"), meta: { title: "编辑地址", hideTabbar: true, requiresAuth: true } },
    { path: "/checkout", name: "checkout", component: () => import("../views/CheckoutView.vue"), meta: { title: "确认订单", hideTabbar: true, requiresAuth: true } },
    { path: "/coupons", name: "coupons", component: () => import("../views/CouponView.vue"), meta: { title: "优惠券", hideTabbar: true, requiresAuth: true } },
    { path: "/service", name: "service", component: () => import("../views/ServiceView.vue"), meta: { title: "客服与帮助", hideTabbar: true, requiresAuth: true } },
    { path: "/notifications", name: "notifications", component: () => import("../views/NotificationsView.vue"), meta: { title: "消息通知", hideTabbar: true, requiresAuth: true } },
    { path: "/:pathMatch(.*)*", redirect: { name: "home" } },
  ],
});

router.beforeEach((to) => {
  const userStore = useUserStore();
  if (to.meta.requiresAuth && !userStore.isLoggedIn) {
    return { name: "login", query: { redirect: to.fullPath } };
  }
  return;
});

router.afterEach((to) => {
  document.title = String(to.meta.title || "优选商城");
});

export default router;
