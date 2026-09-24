<script setup lang="ts">
import { computed, onMounted, reactive, ref } from "vue";
import { Picker, showConfirmDialog, showToast, Switch } from "vant";
import http from "../../lib/http";
import { errMsg } from "../../lib/errmsg";

interface Product {
  id: number;
  name: string;
  description: string;
  price: number;
  originalPrice: number;
  sales: number;
  emoji: string;
  color: string;
  category: string;
  onShelf: boolean;
  featured: boolean;
}

interface Category {
  id: number;
  key: string;
  name: string;
  icon: string;
  note: string;
}

const list = ref<Product[]>([]);
const total = ref(0);
const page = ref(1);
const pageSize = 20;
const totalPages = computed(() => Math.ceil(total.value / pageSize));
const categories = ref<Category[]>([]);
const loading = ref(false);
const saving = ref(false);
const showForm = ref(false);
const showCategoryPicker = ref(false);
const editingId = ref<number | null>(null);

const form = reactive({
  name: "",
  description: "",
  price: "",
  originalPrice: "",
  emoji: "",
  color: "",
  category: "",
  onShelf: true,
  featured: false,
});

const categoryNames = computed(() => {
  const map: Record<string, string> = {};
  categories.value.forEach((c) => (map[c.key] = c.name));
  return map;
});

const categoryColumns = computed(() =>
  categories.value.map((c) => ({ text: c.name, value: c.key }))
);

const categoryLabel = computed(
  () => categoryNames.value[form.category] || form.category || "请选择分类"
);

async function load() {
  loading.value = true;
  try {
    const res = await http.get("/api/admin/products", {
      params: { page: page.value, pageSize },
    });
    list.value = res.data.items;
    total.value = res.data.total;
  } catch {
    showToast("加载失败");
  } finally {
    loading.value = false;
  }
}

function changePage(p: number) {
  page.value = p;
  load();
}

async function loadCategories() {
  try {
    const res = await http.get("/api/admin/categories");
    categories.value = res.data;
  } catch {
    /* 分类加载失败不阻塞商品列表 */
  }
}

function openCreate() {
  editingId.value = null;
  Object.assign(form, {
    name: "",
    description: "",
    price: "",
    originalPrice: "",
    emoji: "",
    color: "",
    category: "",
    onShelf: true,
    featured: false,
  });
  showForm.value = true;
}

function openEdit(p: Product) {
  editingId.value = p.id;
  Object.assign(form, {
    name: p.name,
    description: p.description,
    price: String(p.price),
    originalPrice: String(p.originalPrice),
    emoji: p.emoji,
    color: p.color,
    category: p.category,
    onShelf: p.onShelf,
    featured: p.featured,
  });
  showForm.value = true;
}

function buildBody(onShelf: boolean) {
  return {
    name: form.name,
    description: form.description,
    price: Number(form.price) || 0,
    originalPrice: Number(form.originalPrice) || 0,
    emoji: form.emoji,
    color: form.color,
    category: form.category,
    onShelf,
    featured: form.featured,
  };
}

async function submit() {
  saving.value = true;
  try {
    const body = buildBody(form.onShelf);
    if (editingId.value) {
      await http.put("/api/admin/products/" + editingId.value, body);
    } else {
      await http.post("/api/admin/products", body);
    }
    showToast("已保存");
    showForm.value = false;
    load();
  } catch (err) {
    showToast(errMsg(err, "保存失败"));
  } finally {
    saving.value = false;
  }
}

async function toggleShelf(p: Product) {
  const next = !p.onShelf;
  try {
    await http.put("/api/admin/products/" + p.id, {
      name: p.name,
      description: p.description,
      price: p.price,
      originalPrice: p.originalPrice,
      emoji: p.emoji,
      color: p.color,
      category: p.category,
      onShelf: next,
    });
    p.onShelf = next;
    showToast(next ? "已上架" : "已下架");
  } catch (err) {
    showToast(errMsg(err, "操作失败"));
  }
}

async function toggleFeatured(p: Product) {
  const next = !p.featured;
  try {
    await http.put("/api/admin/products/" + p.id + "/featured", { featured: next });
    p.featured = next;
    showToast(next ? "已设为优选" : "已取消优选");
  } catch (err) {
    showToast(errMsg(err, "操作失败"));
  }
}

async function remove(p: Product) {
  try {
    await showConfirmDialog({
      title: "删除商品",
      message: `确定删除「${p.name}」吗？`,
    });
  } catch {
    return;
  }
  try {
    await http.delete("/api/admin/products/" + p.id);
    showToast("已删除");
    load();
  } catch (err) {
    showToast(errMsg(err, "删除失败"));
  }
}

function onCategoryConfirm({ selectedOptions }: { selectedOptions: any[] }) {
  form.category = selectedOptions[0]?.value || "";
  showCategoryPicker.value = false;
}

onMounted(() => {
  load();
  loadCategories();
});
</script>

<template>
  <div class="admin-page">
    <div class="admin-head">
      <h2>商品管理</h2>
      <van-button size="small" type="primary" @click="openCreate">新建</van-button>
    </div>

    <van-loading v-if="loading" class="loading" color="#ff4d67" />

    <table v-else class="admin-table">
      <thead>
        <tr>
          <th>ID</th><th>名称</th><th>价格</th><th>原价</th>
          <th>分类</th><th>上架</th><th>优选</th><th>操作</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="p in list" :key="p.id">
          <td>{{ p.id }}</td>
          <td>{{ p.emoji }} {{ p.name }}</td>
          <td>¥{{ p.price }}</td>
          <td>¥{{ p.originalPrice }}</td>
          <td>{{ categoryNames[p.category] || p.category }}</td>
          <td>
            <Switch
              :model-value="p.onShelf"
              size="20"
              @update:model-value="toggleShelf(p)"
            />
          </td>
          <td>
            <Switch
              :model-value="p.featured"
              size="20"
              @update:model-value="toggleFeatured(p)"
            />
          </td>
          <td class="admin-ops">
            <van-button size="small" @click="openEdit(p)">编辑</van-button>
            <van-button size="small" type="danger" @click="remove(p)">删除</van-button>
          </td>
        </tr>
      </tbody>
    </table>

    <div class="pagination" v-if="totalPages > 1">
      <van-button size="small" :disabled="page <= 1" @click="changePage(page - 1)">上一页</van-button>
      <span>{{ page }} / {{ totalPages }}（共 {{ total }} 条）</span>
      <van-button size="small" :disabled="page >= totalPages" @click="changePage(page + 1)">下一页</van-button>
    </div>

    <van-popup v-model:show="showForm" position="bottom" round>
      <div class="form-popup">
        <h3>{{ editingId ? "编辑商品" : "新建商品" }}</h3>
        <van-field v-model="form.name" label="名称" placeholder="商品名称" />
        <van-field
          v-model="form.description"
          label="描述"
          type="textarea"
          rows="2"
          autosize
          placeholder="商品描述"
        />
        <van-field v-model="form.price" label="价格" type="number" placeholder="0.00" />
        <van-field v-model="form.originalPrice" label="原价" type="number" placeholder="0.00" />
        <van-field v-model="form.emoji" label="Emoji" placeholder="如 🍎" />
        <van-field v-model="form.color" label="颜色" placeholder="如 #ff6b6b" />
        <van-field
          :model-value="categoryLabel"
          readonly
          is-link
          label="分类"
          placeholder="请选择分类"
          @click="showCategoryPicker = true"
        />
        <van-field label="上架">
          <template #input><Switch v-model="form.onShelf" /></template>
        </van-field>
        <van-field label="优选好物">
          <template #input><Switch v-model="form.featured" /></template>
        </van-field>
        <div class="form-actions">
          <van-button block type="primary" :loading="saving" @click="submit">保存</van-button>
        </div>
      </div>
    </van-popup>

    <van-popup v-model:show="showCategoryPicker" position="bottom" round>
      <Picker
        :columns="categoryColumns"
        title="选择分类"
        @confirm="onCategoryConfirm"
        @cancel="showCategoryPicker = false"
      />
    </van-popup>
  </div>
</template>

