<script setup lang="ts">
import { computed, onMounted, reactive, ref } from "vue";
import { Picker, showConfirmDialog, showToast, Switch } from "vant";
import http from "../../lib/http";
import { errMsg } from "../../lib/errmsg";
import RichEditor from "../../components/RichEditor.vue";

interface Product {
  id: number;
  name: string;
  description: string;
  detail: string;
  price: number;
  originalPrice: number;
  sales: number;
  emoji: string;
  color: string;
  category: string;
  onShelf: boolean;
  featured: boolean;
  stock: number;
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
const showEmojiPicker = ref(false);
const EMOJIS = [
  "🎧", "📱", "💻", "⌚", "📷", "🎮", "⌨️", "🖱️", "🔋", "🔌",
  "👕", "👗", "🧥", "🧢", "👟", "🧶", "👜", "👓", "🧣", "🧤",
  "💄", "🧴", "🧼", "💅", "🪞", "💍",
  "🍎", "🍊", "🍇", "🥕", "🍞", "🍫", "☕", "🍵", "🥜", "🧃",
  "🏠", "🛋️", "🪑", "🖼️", "💡", "🕯️", "🧸", "🎁", "📚", "✏️", "🌂",
];
const editingId = ref<number | null>(null);
const detailEditor = ref<{ getHtml: () => string }>();
const skus = ref<any[]>([]);
const newSku = reactive({ specs: "", price: "", stock: "" });
const editingSku = ref(0);
const skuSaving = ref(false);

const form = reactive({
  name: "",
  description: "",
  detail: "",
  price: "",
  originalPrice: "",
  emoji: "",
  color: "",
  category: "",
  onShelf: true,
  featured: false,
  stock: "",
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
  skus.value = [];
  editingSku.value = 0;
  Object.assign(newSku, { specs: "", price: "", stock: "" });
  Object.assign(form, {
    name: "",
    description: "",
    detail: "",
    price: "",
    originalPrice: "",
    emoji: "",
    color: "",
    category: "",
    onShelf: true,
    featured: false,
    stock: "100",
  });
  showForm.value = true;
}

async function loadSkus(productId: number) {
  try {
    const res = await http.get("/api/admin/products/" + productId + "/skus");
    if (editingId.value !== productId) return;
    skus.value = res.data || [];
    editingSku.value = 0;
    Object.assign(newSku, { specs: "", price: "", stock: "" });
  } catch {
    skus.value = [];
  }
}

async function addSku() {
  if (!editingId.value || skuSaving.value) return;
  if (!newSku.specs.trim() || newSku.price === "" || newSku.stock === "" || !Number.isInteger(Number(newSku.stock))) { showToast("请填写规格、价格和整数库存"); return; }
  skuSaving.value = true;
  try {
    const payload = {
      specs: newSku.specs,
      price: Number(newSku.price) || 0,
      stock: Number(newSku.stock) || 0,
    };
    if (editingSku.value) await http.put("/api/admin/skus/" + editingSku.value, payload);
    else await http.post("/api/admin/products/" + editingId.value + "/skus", payload);
    showToast(editingSku.value ? "规格已更新" : "已添加");
    newSku.specs = "";
    newSku.price = "";
    newSku.stock = "";
    await loadSkus(editingId.value);
  } catch (err) {
    showToast(errMsg(err, "添加失败"));
  } finally { skuSaving.value = false; }
}

function editSku(sku: any) {
  editingSku.value = sku.id;
  Object.assign(newSku, { specs: sku.specs, price: String(sku.price), stock: String(sku.stock) });
}

async function deleteSku(sku: any) {
  if (!editingId.value) return;
  try {
    await http.delete("/api/admin/skus/" + sku.id);
    showToast("已删除");
    loadSkus(editingId.value);
  } catch (err) {
    showToast(errMsg(err, "删除失败"));
  }
}

function openEdit(p: Product) {
  editingId.value = p.id;
  skus.value = [];
  editingSku.value = 0;
  Object.assign(newSku, { specs: "", price: "", stock: "" });
  Object.assign(form, {
    name: p.name,
    description: p.description,
    detail: p.detail || "",
    price: String(p.price),
    originalPrice: String(p.originalPrice),
    emoji: p.emoji,
    color: p.color,
    category: p.category,
    onShelf: p.onShelf,
    featured: p.featured,
    stock: String(p.stock ?? 0),
  });
  loadSkus(p.id);
  showForm.value = true;
}

function buildBody(onShelf: boolean) {
  return {
    name: form.name,
    description: form.description,
    detail: form.detail,
    price: Number(form.price) || 0,
    originalPrice: Number(form.originalPrice) || 0,
    emoji: form.emoji,
    color: form.color,
    category: form.category,
    onShelf,
    featured: form.featured,
    stock: Number(form.stock) || 0,
  };
}

async function submit() {
  saving.value = true;
  try {
    if (detailEditor.value) form.detail = detailEditor.value.getHtml();
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
  p.onShelf = next;
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
    showToast(next ? "已上架" : "已下架");
  } catch (err) {
    p.onShelf = !next;
    showToast(errMsg(err, "操作失败"));
  }
}

async function toggleFeatured(p: Product) {
  const next = !p.featured;
  p.featured = next;
  try {
    await http.put("/api/admin/products/" + p.id + "/featured", { featured: next });
    showToast(next ? "已设为优选" : "已取消优选");
  } catch (err) {
    p.featured = !next;
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

function pickEmoji(e: string) {
  form.emoji = e;
  showEmojiPicker.value = false;
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
          <th>ID</th><th>名称</th><th>价格</th><th>原价</th><th>库存</th>
          <th>分类</th><th>上架</th><th>优选</th><th>操作</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="p in list" :key="p.id">
          <td>{{ p.id }}</td>
          <td>{{ p.emoji }} {{ p.name }}</td>
          <td>¥{{ p.price }}</td>
          <td>¥{{ p.originalPrice }}</td>
          <td>{{ p.stock }}</td>
          <td>{{ categoryNames[p.category] || p.category }}</td>
          <td>
            <Switch
              :model-value="p.onShelf"
              size="20px"
              @change="toggleShelf(p)"
            />
          </td>
          <td>
            <Switch
              :model-value="p.featured"
              size="20px"
              @change="toggleFeatured(p)"
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
        <van-field label="图文详情">
          <template #input>
            <RichEditor ref="detailEditor" v-if="showForm" v-model="form.detail" />
          </template>
        </van-field>
        <van-field v-model="form.price" label="价格" type="number" placeholder="0.00" />
        <van-field v-model="form.originalPrice" label="原价" type="number" placeholder="0.00" />
        <van-field v-model="form.stock" label="库存" type="number" placeholder="库存数量" />
        <van-field :model-value="form.emoji" readonly is-link label="Emoji" placeholder="选择或输入图标" @click="showEmojiPicker = true" />
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
        <div v-if="editingId" class="sku-editor">
          <div class="sku-editor-title">规格管理（多规格独立价格/库存）</div>
          <div v-for="sku in skus" :key="sku.id" class="sku-row">
            <span>{{ sku.specs }} · ¥{{ sku.price }} · 库存 {{ sku.stock }}</span>
            <van-button size="mini" plain @click="editSku(sku)">编辑</van-button>
            <van-button size="mini" type="danger" plain @click="deleteSku(sku)">删除</van-button>
          </div>
          <div class="sku-add">
            <van-field v-model="newSku.specs" label="规格" placeholder="如 红色 / M" />
            <van-field v-model="newSku.price" label="价格" type="number" placeholder="元" />
            <van-field v-model="newSku.stock" label="库存" type="number" placeholder="数量" />
            <van-button size="small" type="primary" plain :loading="skuSaving" @click="addSku">{{ editingSku ? '保存规格' : '添加规格' }}</van-button>
            <van-button v-if="editingSku" size="small" plain @click="editingSku = 0; Object.assign(newSku, { specs: '', price: '', stock: '' })">取消编辑</van-button>
          </div>
        </div>
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

    <van-popup v-model:show="showEmojiPicker" position="bottom" round>
      <div class="emoji-picker">
        <h4>选择图标</h4>
        <div class="emoji-grid">
          <span
            v-for="e in EMOJIS"
            :key="e"
            class="emoji-item"
            :class="{ active: form.emoji === e }"
            @click="pickEmoji(e)"
          >{{ e }}</span>
        </div>
        <van-field v-model="form.emoji" label="自定义" placeholder="直接输入 emoji 或字符" />
      </div>
    </van-popup>
  </div>
</template>

<style scoped>
.sku-editor { margin: 8px 16px; padding: 8px; background: #f7f8fa; border-radius: 8px; }
.sku-editor-title { font-size: 13px; color: #646a73; margin-bottom: 6px; }
.sku-row { display: flex; align-items: center; justify-content: space-between; font-size: 13px; padding: 4px 0; }
.sku-add { margin-top: 6px; padding-top: 6px; border-top: 1px dashed #e5e6eb; }
.emoji-picker { padding: 12px 16px; max-height: 60vh; overflow-y: auto; }
.emoji-picker h4 { margin: 0 0 10px; font-size: 14px; }
.emoji-grid { display: flex; flex-wrap: wrap; gap: 8px; margin-bottom: 12px; }
.emoji-item { width: 40px; height: 40px; display: flex; align-items: center; justify-content: center; font-size: 22px; background: #f5f6f8; border-radius: 8px; cursor: pointer; }
.emoji-item.active { background: #ffecef; outline: 2px solid #ee0a24; }
</style>

