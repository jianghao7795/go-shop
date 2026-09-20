<script setup lang="ts">
import { onMounted, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { showConfirmDialog, showToast } from "vant";
import http from "../lib/http";

interface Address { id: number; name: string; phone: string; region: string; detail: string; tag: string; isDefault: boolean; }

const router = useRouter();
const route = useRoute();
const selecting = route.query.select === "1";
const addresses = ref<Address[]>([]);
const loading = ref(false);

const tagColor: Record<string, string> = { 家: "#ee0a24", 公司: "#1989fa", 学校: "#07c160" };
const tagBg: Record<string, string> = { 家: "#fff1f2", 公司: "#e8f3ff", 学校: "#e8fff0" };

async function load() {
  loading.value = true;
  try {
    const res = await http.get("/api/addresses");
    addresses.value = res.data;
  } catch {
    addresses.value = [];
  } finally {
    loading.value = false;
  }
}

function edit(addr: Address) {
  router.push({ name: "address-edit", query: { id: addr.id } });
}

function onCardClick(addr: Address) {
  if (selecting) {
    localStorage.setItem("shop_checkout_address_id", String(addr.id));
    router.back();
    return;
  }
  edit(addr);
}

function maskPhone(phone: string) {
  return phone.replace(/^(\d{3})\d{4}(\d{4})$/, "$1****$2");
}

async function setDefault(addr: Address) {
  try {
    await http.put("/api/addresses/" + addr.id + "/default");
    showToast("已设为默认");
    addresses.value = addresses.value.map(a => ({ ...a, isDefault: a.id === addr.id }));
  } catch {
    showToast("设置默认失败，请重试");
  }
}

async function remove(addr: Address) {
  try {
    await showConfirmDialog({ title: "删除地址", message: "确定删除该收货地址吗？" });
  } catch {
    return;
  }
  try {
    await http.delete("/api/addresses/" + addr.id);
    showToast("已删除");
    load();
  } catch { /* 忽略 */ }
}

onMounted(load);
</script>

<template>
  <div class="sub-page">
    <van-nav-bar :title="selecting ? '选择收货地址' : '收货地址'" left-arrow @click-left="$router.back()" />
    <div v-if="loading" class="loading"><van-loading color="#ff4d67" /></div>
    <van-empty v-else-if="!addresses.length" description="还没有收货地址" />
    <div v-else class="address-cards">
      <van-swipe-cell v-for="addr in addresses" :key="addr.id">
        <div class="address-card" @click="onCardClick(addr)">
          <div class="address-card-top">
            <span v-if="addr.tag" class="address-tag" :style="{ color: tagColor[addr.tag], background: tagBg[addr.tag] }">{{ addr.tag }}</span>
            <strong>{{ addr.name }}</strong>
            <span class="address-phone">{{ maskPhone(addr.phone) }}</span>
            <span v-if="addr.isDefault" class="address-default-tag">默认</span>
          </div>
          <div class="address-card-detail">{{ addr.region }} {{ addr.detail }}</div>
          <div class="address-card-ops" @click.stop>
            <span v-if="!addr.isDefault" class="op" @click="setDefault(addr)">设为默认</span>
            <span class="op" @click="edit(addr)">编辑</span>
            <span class="op op-danger" @click="remove(addr)">删除</span>
          </div>
        </div>
        <template #right>
          <van-button square type="danger" text="删除" class="swipe-delete" @click="remove(addr)" />
        </template>
      </van-swipe-cell>
    </div>
    <div class="address-add">
      <van-button block round type="danger" @click="router.push('/address/edit')">新增收货地址</van-button>
    </div>
  </div>
</template>
