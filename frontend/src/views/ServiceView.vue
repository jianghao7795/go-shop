<script setup lang="ts">
import { showToast } from "vant";

const faqs = [
  { q: "如何查询我的订单？", a: "进入「我的」-「我的订单」，可查看全部订单及各状态订单。" },
  { q: "如何修改收货地址？", a: "进入「我的」-「收货地址」，可新增、编辑、删除或设为默认地址。" },
  { q: "下单后多久发货？", a: "商品一般在 24 小时内发货，预计明日送达。" },
  { q: "如何申请售后？", a: "在订单详情中点击「申请售后」，客服会尽快处理。" },
];

function contact() { showToast("客服热线 400-123-4567"); }
</script>

<template>
  <div class="sub-page">
    <van-nav-bar title="客服与帮助" left-arrow @click-left="$router.back()" />
    <van-cell-group inset class="service-list">
      <van-cell v-for="faq in faqs" :key="faq.q" :title="faq.q" :label="faq.a" />
    </van-cell-group>
    <div class="service-contact">
      <van-button block round type="danger" @click="contact">联系在线客服</van-button>
    </div>
  </div>
</template>
