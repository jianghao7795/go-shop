<script setup lang="ts">
import { ref } from "vue";

const active = ref(0);
const categories = [
  { name: "数码电器", icon: "📱", note: "手机、耳机、智能设备" },
  { name: "家居生活", icon: "🏠", note: "居家好物，提升生活品质" },
  { name: "服饰鞋包", icon: "👕", note: "精选潮流穿搭" },
  { name: "美妆护肤", icon: "💄", note: "热门品牌，焕亮好气色" },
  { name: "食品生鲜", icon: "🍊", note: "新鲜直达，安心好味道" },
];

</script>

<template>
  <div class="sub-page">
    <van-nav-bar title="商品分类" />
    <div class="category-layout">
      <van-sidebar v-model="active">
        <van-sidebar-item v-for="item in categories" :key="item.name" :title="item.name" />
      </van-sidebar>
      <section class="category-detail">
        <h2>{{ categories[active].name }}</h2>
        <p>{{ categories[active].note }}</p>
        <div class="category-banner">{{ categories[active].icon }} <strong>精选好物</strong></div>
        <van-cell-group inset>
          <van-cell title="热销商品" is-link value="查看全部" />
          <van-cell title="新品上架" is-link value="去逛逛" />
          <van-cell title="限时折扣" is-link value="立即抢购" />
        </van-cell-group>
      </section>
    </div>
  </div>
</template>



