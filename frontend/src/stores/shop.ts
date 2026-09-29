import { computed, ref } from "vue";
import { defineStore } from "pinia";
import { showToast } from "vant";

export interface ShopProduct {
  id: number; name: string; description: string; price: number; originalPrice: number;
  sales: number; emoji: string; color: string; category: string;
}
interface CartLine { product: ShopProduct; quantity: number; checked: boolean }

const CART_KEY = "shop_cart";

function loadCart(): Record<number, CartLine> {
  try {
    const raw = localStorage.getItem(CART_KEY);
    if (!raw) return {};
    const data = JSON.parse(raw) as Record<number, CartLine>;
    // 兼容旧数据：旧结构没有 checked 字段，默认勾选
    for (const k in data) {
      if (data[k].checked === undefined) data[k].checked = true;
    }
    return data;
  } catch {
    return {};
  }
}

export const useShopCart = defineStore("shopCart", () => {
  const items = ref<Record<number, CartLine>>(loadCart());
  const cartItems = computed(() => Object.values(items.value));
  const count = computed(() => cartItems.value.reduce((sum, item) => sum + item.quantity, 0));
  const total = computed(() => cartItems.value.reduce((sum, item) => sum + item.product.price * item.quantity, 0));
  const checkedItems = computed(() => cartItems.value.filter(it => it.checked));
  const checkedCount = computed(() => checkedItems.value.reduce((sum, item) => sum + item.quantity, 0));
  const checkedTotal = computed(() => checkedItems.value.reduce((sum, item) => sum + item.product.price * item.quantity, 0));
  const allChecked = computed(() => cartItems.value.length > 0 && cartItems.value.every(it => it.checked));

  function persist() {
    localStorage.setItem(CART_KEY, JSON.stringify(items.value));
  }

  function add(product: ShopProduct) {
    const line = items.value[product.id];
    if (line) {
      line.quantity += 1;
      line.checked = true;
    } else {
      items.value[product.id] = { product, quantity: 1, checked: true };
    }
    persist();
    showToast("已加入购物车");
  }
  function increase(id: number) {
    const line = items.value[id]; if (!line) return;
    line.quantity += 1;
    persist();
  }
  function decrease(id: number) {
    const line = items.value[id]; if (!line) return;
    if (line.quantity <= 1) delete items.value[id]; else line.quantity -= 1;
    persist();
  }
  function toggleChecked(id: number) {
    const line = items.value[id]; if (!line) return;
    line.checked = !line.checked;
    persist();
  }
  function setAllChecked(checked: boolean) {
    for (const k in items.value) items.value[k].checked = checked;
    persist();
  }
  function remove(ids: number[]) {
    for (const id of ids) delete items.value[id];
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
  return { items, cartItems, count, total, checkedItems, checkedCount, checkedTotal, allChecked, add, increase, decrease, toggleChecked, setAllChecked, remove, removeChecked, clear };
});
