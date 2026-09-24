<template>
  <div class="admin-layout">
    <aside class="admin-sidebar">
      <div class="admin-brand">🛍️ 管理后台</div>
      <router-link to="/admin">概览</router-link>
      <router-link to="/admin/products">商品</router-link>
      <router-link to="/admin/categories">分类</router-link>
      <router-link to="/admin/orders">订单</router-link>
      <router-link to="/admin/users">用户</router-link>
      <router-link to="/admin/coupons">优惠券</router-link>
      <router-link to="/admin/notifications">通知</router-link>
      <router-link to="/profile">← 返回商城</router-link>
    </aside>
    <main class="admin-main"><router-view /></main>
  </div>
</template>
