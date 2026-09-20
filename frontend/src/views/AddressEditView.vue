<script setup lang="ts">
import { onMounted, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { showToast } from "vant";
import { areaList, useCascaderAreaData } from "@vant/area-data";
import http from "../lib/http";

interface Address { id: number; name: string; phone: string; region: string; detail: string; tag: string; isDefault: boolean; }

const route = useRoute();
const router = useRouter();

const id = ref(Number(route.query.id) || 0);
const name = ref("");
const phone = ref("");
const region = ref("");
const detail = ref("");
const tag = ref("");
const isDefault = ref(false);
const loading = ref(false);

const showCascader = ref(false);
const cascaderValue = ref("");
const tags = ["家", "公司", "学校"];
const regionOptions = useCascaderAreaData();

function onFinish({ selectedOptions }: { selectedOptions: { text?: string }[] }) {
  region.value = selectedOptions.map(o => o.text || "").join(" ");
  showCascader.value = false;
}

function selectTag(t: string) {
  tag.value = tag.value === t ? "" : t;
}

const rawAddress = ref("");

function recognize() {
  const text = rawAddress.value.trim();
  if (!text) { showToast("请先粘贴完整地址"); return; }
  const { province_list, city_list, county_list } = areaList;
  let province = "", provinceCode = "";
  for (const [code, name] of Object.entries(province_list)) {
    if (name && text.includes(name)) { province = name; provinceCode = code; break; }
  }
  let city = "", cityCode = "";
  const cityPrefix = provinceCode.slice(0, 2);
  for (const [code, name] of Object.entries(city_list)) {
    if (code.startsWith(cityPrefix) && name && text.includes(name)) { city = name; cityCode = code; break; }
  }
  let county = "";
  const countyPrefix = cityCode.slice(0, 4);
  for (const [code, name] of Object.entries(county_list)) {
    if (code.startsWith(countyPrefix) && name && text.includes(name)) { county = name; break; }
  }
  const regionText = [province, city, county].filter(Boolean).join(" ");
  if (!regionText) { showToast("未能识别出省市区，请手动选择"); return; }
  region.value = regionText;
  let detailText = text;
  for (const name of [province, city, county]) {
    if (name) detailText = detailText.replace(name, "");
  }
  detail.value = detailText.trim();
  showToast("识别成功");
}

async function loadExisting() {
  if (!id.value) return;
  const res = await http.get("/api/addresses");
  const list = res.data as Address[];
  const addr = list.find(a => a.id === id.value);
  if (addr) {
    name.value = addr.name;
    phone.value = addr.phone;
    region.value = addr.region;
    detail.value = addr.detail;
    tag.value = addr.tag;
    isDefault.value = addr.isDefault;
  }
}

async function onSubmit() {
  loading.value = true;
  try {
    const payload = { name: name.value, phone: phone.value, region: region.value, detail: detail.value, tag: tag.value, isDefault: isDefault.value };
    if (id.value) await http.put("/api/addresses/" + id.value, payload);
    else await http.post("/api/addresses", payload);
    showToast("保存成功");
    router.back();
  } catch (err) {
    const e = err as any;
    if (e?.response) showToast(e.response.data?.message || "保存失败");
    else showToast("保存失败，请稍后重试");
  } finally {
    loading.value = false;
  }
}

onMounted(loadExisting);
</script>

<template>
  <div class="sub-page">
    <van-nav-bar :title="id ? '编辑地址' : '新增地址'" left-arrow @click-left="$router.back()" />
    <van-form @submit="onSubmit">
      <van-cell-group inset class="recognize-group">
        <van-field v-model="rawAddress" type="textarea" rows="2" autosize placeholder="粘贴完整地址，自动识别省市区（如：广东省深圳市南山区科技园1号）" />
        <div class="recognize-btn">
          <van-button size="small" round type="primary" @click="recognize">智能识别</van-button>
        </div>
      </van-cell-group>
      <van-cell-group inset>
        <van-field v-model="name" name="name" label="收货人" placeholder="请输入收货人姓名" :rules="[{ required: true, message: '请输入收货人' }]" />
        <van-field v-model="phone" name="phone" label="手机号" placeholder="请输入手机号" :rules="[{ required: true, message: '请输入手机号' }]" />
        <van-field v-model="region" name="region" label="所在地区" placeholder="请选择省 / 市 / 区" readonly is-link :rules="[{ required: true, message: '请选择所在地区' }]" @click="showCascader = true" />
        <van-field v-model="detail" name="detail" label="详细地址" placeholder="街道、楼牌号等" :rules="[{ required: true, message: '请输入详细地址' }]" />
        <van-field name="tag" label="标签">
          <template #input>
            <div class="tag-select">
              <span v-for="t in tags" :key="t" :class="['tag-option', { active: tag === t }]" @click="selectTag(t)">{{ t }}</span>
            </div>
          </template>
        </van-field>
        <van-field name="isDefault" label="设为默认地址">
          <template #input><van-checkbox v-model="isDefault">设为默认收货地址</van-checkbox></template>
        </van-field>
      </van-cell-group>
      <div class="login-submit">
        <van-button round block type="danger" native-type="submit" :loading="loading">保存</van-button>
      </div>
    </van-form>

    <van-popup v-model:show="showCascader" position="bottom" round>
      <van-cascader v-model="cascaderValue" title="请选择所在地区" :options="regionOptions" @close="showCascader = false" @finish="onFinish" />
    </van-popup>
  </div>
</template>
