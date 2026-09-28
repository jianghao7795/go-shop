export interface AdminMenuItem {
  title: string;
  path: string;
  perm?: string; // 空 = 始终显示（概览）
}

export const adminMenu: AdminMenuItem[] = [
  { title: "概览", path: "/admin" },
  { title: "商品", path: "/admin/products", perm: "product:manage" },
  { title: "分类", path: "/admin/categories", perm: "category:manage" },
  { title: "订单", path: "/admin/orders", perm: "order:manage" },
  { title: "用户", path: "/admin/users", perm: "user:manage" },
  { title: "优惠券", path: "/admin/coupons", perm: "coupon:manage" },
  { title: "通知", path: "/admin/notifications", perm: "notification:manage" },
  { title: "权限", path: "/admin/permissions", perm: "user:manage" },
  { title: "角色", path: "/admin/roles", perm: "user:manage" },
];
