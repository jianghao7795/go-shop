import { computed, ref } from "vue";
import { defineStore } from "pinia";
import { showToast } from "vant";

export interface ShopProduct {
  id: number; name: string; description: string; price: number; originalPrice: number;
  sales: number; emoji: string; color: string; category: string;
}
interface CartLine { product: ShopProduct; quantity: number }

const CART_KEY = "shop_cart";

function loadCart(): Record<number, CartLine> {
  try {
    const raw = localStorage.getItem(CART_KEY);
    return raw ? JSON.parse(raw) as Record<number, CartLine> : {};
  } catch {
    return {};
  }
}

export const useShopCart = defineStore("shopCart", () => {
  const items = ref<Record<number, CartLine>>(loadCart());
  const cartItems = computed(() => Object.values(items.value));
  const count = computed(() => cartItems.value.reduce((sum, item) => sum + item.quantity, 0));
  const total = computed(() => cartItems.value.reduce((sum, item) => sum + item.product.price * item.quantity, 0));

  function persist() {
    localStorage.setItem(CART_KEY, JSON.stringify(items.value));
  }

  function add(product: ShopProduct) {
    const line = items.value[product.id];
    if (line) line.quantity += 1;
    else items.value[product.id] = { product, quantity: 1 };
    persist();
    showToast("已加入购物车");
  }
  function decrease(id: number) {
    const line = items.value[id]; if (!line) return;
    if (line.quantity <= 1) delete items.value[id]; else line.quantity -= 1;
    persist();
  }
  function remove(ids: number[]) {
    for (const id of ids) delete items.value[id];
    persist();
  }
  function clear() {
    items.value = {};
    persist();
  }
  return { items, cartItems, count, total, add, decrease, remove, clear };
});
