import { createRouter, createWebHashHistory } from "vue-router";
import HomeView from "../views/HomeView.vue";
import CategoryView from "../views/CategoryView.vue";
import CartView from "../views/CartView.vue";
import ProfileView from "../views/ProfileView.vue";
import ProductDetailView from "../views/ProductDetailView.vue";
import LoginView from "../views/LoginView.vue";
import RegisterView from "../views/RegisterView.vue";
import OrderListView from "../views/OrderListView.vue";
import AddressListView from "../views/AddressListView.vue";
import AddressEditView from "../views/AddressEditView.vue";
import CheckoutView from "../views/CheckoutView.vue";
import CouponView from "../views/CouponView.vue";
import ServiceView from "../views/ServiceView.vue";
import OrderDetailView from "../views/OrderDetailView.vue";
import { useUserStore } from "../stores/user";

const router = createRouter({
  history: createWebHashHistory(),
  scrollBehavior: () => ({ top: 0 }),
  routes: [
    { path: "/", name: "home", component: HomeView, meta: { title: "优选商城" } },
    { path: "/category", name: "category", component: CategoryView, meta: { title: "商品分类" } },
    { path: "/cart", name: "cart", component: CartView, meta: { title: "购物车" } },
    { path: "/profile", name: "profile", component: ProfileView, meta: { title: "个人中心", requiresAuth: true } },
    { path: "/product/:id", name: "product-detail", component: ProductDetailView, meta: { title: "商品详情", hideTabbar: true } },
    { path: "/login", name: "login", component: LoginView, meta: { title: "登录", hideTabbar: true } },
    { path: "/register", name: "register", component: RegisterView, meta: { title: "注册", hideTabbar: true } },
    { path: "/orders", name: "orders", component: OrderListView, meta: { title: "我的订单", hideTabbar: true, requiresAuth: true } },
    { path: "/orders/:id", name: "order-detail", component: OrderDetailView, meta: { title: "订单详情", hideTabbar: true, requiresAuth: true } },
    { path: "/address", name: "address", component: AddressListView, meta: { title: "收货地址", hideTabbar: true, requiresAuth: true } },
    { path: "/address/edit", name: "address-edit", component: AddressEditView, meta: { title: "编辑地址", hideTabbar: true, requiresAuth: true } },
    { path: "/checkout", name: "checkout", component: CheckoutView, meta: { title: "确认订单", hideTabbar: true, requiresAuth: true } },
    { path: "/coupons", name: "coupons", component: CouponView, meta: { title: "优惠券", hideTabbar: true, requiresAuth: true } },
    { path: "/service", name: "service", component: ServiceView, meta: { title: "客服与帮助", hideTabbar: true, requiresAuth: true } },
    { path: "/:pathMatch(.*)*", redirect: { name: "home" } },
  ],
});

router.beforeEach((to) => {
  const userStore = useUserStore();
  if (to.meta.requiresAuth && !userStore.isLoggedIn) {
    return { name: "login", query: { redirect: to.fullPath } };
  }
});

router.afterEach((to) => {
  document.title = String(to.meta.title || "优选商城");
});

export default router;

