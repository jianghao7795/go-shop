import { computed, ref } from "vue";
import { defineStore } from "pinia";
import { showToast } from "vant";
import http from "../lib/http";
import { useUserStore } from "./user";

export interface ShopSku {
  id: number;
  specs: string;
  price: number;
  stock: number;
}

export interface ShopProduct {
  id: number; name: string; description: string; detail?: string; price: number; originalPrice: number;
  sales: number; emoji: string; color: string; category: string;
  skus?: ShopSku[];
}
interface CartLine { product: ShopProduct; skuId: number; specs: string; quantity: number; checked: boolean }

const CART_KEY = "shop_cart";

// cartKey 生成购物车行的唯一 key（商品 + 规格）。
export function cartKey(productId: number, skuId: number): string {
  return productId + ":" + skuId;
}

function loadCart(): Record<string, CartLine> {
  try {
    const raw = localStorage.getItem(CART_KEY);
    if (!raw) return {};
    const data = JSON.parse(raw) as Record<string, CartLine>;
    // 兼容旧数据：旧结构没有 checked / skuId 字段
    for (const k in data) {
      if (data[k].checked === undefined) data[k].checked = true;
      if (data[k].skuId === undefined) data[k].skuId = 0;
    }
    return data;
  } catch {
    return {};
  }
}

export const useShopCart = defineStore("shopCart", () => {
  const items = ref<Record<string, CartLine>>(loadCart());
  const cartItems = computed(() => Object.values(items.value));
  const total = computed(() => cartItems.value.reduce((sum, item) => sum + item.product.price * item.quantity, 0));
  const checkedItems = computed(() => cartItems.value.filter(it => it.checked));
  const checkedCount = computed(() => checkedItems.value.reduce((sum, item) => sum + item.quantity, 0));
  const checkedTotal = computed(() => checkedItems.value.reduce((sum, item) => sum + item.product.price * item.quantity, 0));
  const allChecked = computed(() => cartItems.value.length > 0 && cartItems.value.every(it => it.checked));

  function persist() {
    localStorage.setItem(CART_KEY, JSON.stringify(items.value));
    syncToServer();
  }

  // 登录后把本地购物车全量同步到服务端（fire-and-forget，失败不阻断本地）。
  async function syncToServer() {
    const userStore = useUserStore();
    if (!userStore.isLoggedIn) return;
    const body = cartItems.value.map((it) => ({ productId: it.product.id, skuId: it.skuId, quantity: it.quantity }));
    try {
      await http.put("/api/cart", body);
    } catch { /* 同步失败本地仍有效 */ }
  }

  // 登录后合并服务端购物车与本地购物车（数量取大），并写回。
  async function mergeFromServer() {
    const userStore = useUserStore();
    if (!userStore.isLoggedIn) return;
    try {
      const res = await http.get("/api/cart");
      const serverItems = res.data as { productId: number; skuId: number; specs: string; quantity: number; name: string; price: number; emoji: string; color: string }[];
      for (const si of serverItems) {
        const key = cartKey(si.productId, si.skuId);
        const line = items.value[key];
        if (line) {
          line.quantity = Math.max(line.quantity, si.quantity);
          line.product = { ...line.product, name: si.name, price: si.price, emoji: si.emoji, color: si.color };
        } else {
          items.value[key] = {
            product: { id: si.productId, name: si.name, description: "", price: si.price, originalPrice: si.price, sales: 0, emoji: si.emoji, color: si.color, category: "" },
            skuId: si.skuId,
            specs: si.specs || "",
            quantity: si.quantity,
            checked: true,
          };
        }
      }
      persist();
    } catch { /* 忽略 */ }
  }

  function add(product: ShopProduct, skuId = 0, specs = "") {
    const key = cartKey(product.id, skuId);
    const line = items.value[key];
    if (line) {
      line.quantity += 1;
      line.checked = true;
    } else {
      items.value[key] = { product, skuId, specs, quantity: 1, checked: true };
    }
    persist();
    showToast("已加入购物车");
  }
  function increase(key: string) {
    const line = items.value[key]; if (!line) return;
    line.quantity += 1;
    persist();
  }
  function decrease(key: string) {
    const line = items.value[key]; if (!line) return;
    if (line.quantity <= 1) delete items.value[key]; else line.quantity -= 1;
    persist();
  }
  function toggleChecked(key: string) {
    const line = items.value[key]; if (!line) return;
    line.checked = !line.checked;
    persist();
  }
  function setAllChecked(checked: boolean) {
    for (const k in items.value) items.value[k].checked = checked;
    persist();
  }
  function remove(keys: string[]) {
    for (const k of keys) delete items.value[k];
    persist();
  }
  function removeChecked() {
    for (const k in items.value) {
      if (items.value[k].checked) delete items.value[k];
    }
    persist();
  }
  function clear() {
    items.value = {};
    persist();
  }
  return { items, cartItems, total, checkedItems, checkedCount, checkedTotal, allChecked, add, increase, decrease, toggleChecked, setAllChecked, remove, removeChecked, clear, mergeFromServer };
});
