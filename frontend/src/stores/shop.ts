import { computed, onScopeDispose, ref, watch } from "vue";
import { defineStore } from "pinia";
import { showToast } from "vant";
import http from "../lib/http";
import { useUserStore } from "./user";

export interface ShopSku { id: number; specs: string; price: number; stock: number }
export interface ShopProduct {
  id: number; name: string; description: string; detail?: string; price: number; originalPrice: number;
  sales: number; emoji: string; color: string; category: string;
  skus?: ShopSku[]; stock?: number; onShelf?: boolean;
}
export interface CartLine {
  product: ShopProduct; skuId: number; specs: string; quantity: number; checked: boolean;
  stock: number; available: boolean; reason: string; priceChanged?: boolean;
}
interface CartInput { productId: number; skuId: number; quantity: number }
interface CartItemView extends CartInput {
  name: string; price: number; specs: string; emoji: string; color: string;
  stock: number; available: boolean; reason: string;
}
interface CartResponse { version: number; items: CartItemView[] }
interface CheckoutRequest {
  requestId: string; version: number; addressId: number; userCouponId: number; expectedAmount: number;
  items: (CartInput & { price: number })[];
}
export const MAX_CART_QUANTITY = 999;
export function cartKey(productId: number, skuId: number): string { return productId + ":" + skuId; }
export function cartCacheKey(owner: string): string { return "shop_cart:v2:" + (owner ? "user:" + encodeURIComponent(owner) : "guest"); }
const cents = (value: number) => Math.round(value * 100);

// Validate persisted data and rebuild keys, including pre-SKU numeric keys.
export function normalizeCart(data: unknown): Record<string, CartLine> {
  const result: Record<string, CartLine> = {};
  if (!data || typeof data !== "object") return result;
  for (const value of Object.values(data)) {
    const row = value as Partial<CartLine> | null;
    if (!row?.product || !Number.isSafeInteger(row.product.id) || row.product.id <= 0 ||
        typeof row.product.price !== "number" || !Number.isFinite(row.product.price) || row.product.price < 0 ||
        !Number.isSafeInteger(row.quantity) || Number(row.quantity) <= 0) continue;
    const skuId = row.skuId ?? 0;
    if (!Number.isSafeInteger(skuId) || skuId < 0) continue;
    const key = cartKey(row.product.id, skuId);
    const stock = Math.max(0, Math.min(MAX_CART_QUANTITY, Number(row.stock ?? row.product.stock ?? MAX_CART_QUANTITY)));
    const line: CartLine = {
      product: row.product, skuId, specs: row.specs || "", quantity: Math.min(MAX_CART_QUANTITY, Number(row.quantity)),
      checked: row.checked !== false, stock: Number.isFinite(stock) ? stock : 0,
      available: row.available !== false, reason: row.reason || "", priceChanged: !!row.priceChanged,
    };
    if (result[key]) line.quantity = Math.max(result[key].quantity, line.quantity);
    if (Object.keys(result).length < 100 || result[key]) result[key] = line;
  }
  return result;
}

export const useShopCart = defineStore("shopCart", () => {
  const user = useUserStore();
  const legacyOwner = localStorage.getItem("shop_token") ? localStorage.getItem("shop_user") : null;
  if (!legacyOwner) {
    try {
      const unowned = localStorage.getItem("shop_cart");
      if (unowned) {
        localStorage.setItem("shop_cart:legacy-unassigned", unowned);
        localStorage.removeItem("shop_cart");
      }
    } catch { /* Do not infer ownership of old data when storage is unavailable. */ }
  }
  const owner = computed(() => user.isLoggedIn ? user.username : "");
  const items = ref<Record<string, CartLine>>({});
  const version = ref<number | null>(null);
  const syncing = ref(false);
  const loading = ref(false);
  const submitting = ref(false);
  const syncError = ref("");
  const pendingCheckout = ref<CheckoutRequest | null>(null);
  const locked = computed(() => loading.value || submitting.value || !!pendingCheckout.value);
  let changes = new Map<string, number>();
  let sequence = 0;
  let generation = 0;
  let syncTask: Promise<boolean> | null = null;
  let refreshTask: Promise<boolean> | null = null;
  const cartItems = computed(() => Object.values(items.value));
  const checkedItems = computed(() => cartItems.value.filter(it => it.checked && it.available));
  const checkedCount = computed(() => checkedItems.value.reduce((n, it) => n + it.quantity, 0));
  const selectedCount = computed(() => cartItems.value.filter(it => it.checked).length);
  const sum = (rows: CartLine[]) => rows.reduce((n, it) => n + cents(it.product.price) * it.quantity, 0) / 100;
  const total = computed(() => sum(cartItems.value));
  const checkedTotal = computed(() => sum(checkedItems.value));
  const allChecked = computed(() => cartItems.value.some(it => it.available) && cartItems.value.every(it => !it.available || it.checked));
  const allSelected = computed(() => cartItems.value.length > 0 && cartItems.value.every(it => it.checked));
  const addressStorageKey = computed(() => "shop_checkout_address_id:" + encodeURIComponent(owner.value));
  const body = (): CartInput[] => cartItems.value.map(it => ({ productId: it.product.id, skuId: it.skuId, quantity: it.quantity }));
  const config = () => ({ headers: { Authorization: owner.value ? "Bearer " + user.token : "" }, timeout: 10000 });

  function persist(): boolean {
    try {
      localStorage.setItem(cartCacheKey(owner.value), JSON.stringify({ items: items.value, version: version.value, changes: [...changes.keys()], pendingCheckout: pendingCheckout.value }));
      return true;
    } catch { syncError.value = "无法保存购物车，请检查本地存储空间"; return false; }
  }

  function loadOwner() {
    generation++;
    syncTask = null; refreshTask = null; changes = new Map();
    items.value = {}; version.value = null; pendingCheckout.value = null;
    syncing.value = false; loading.value = false; submitting.value = false; syncError.value = "";
    try {
      // Only the account present at startup can own legacy data. Unknown
      // ownership is archived separately rather than assigned on a later login.
      const raw = localStorage.getItem(cartCacheKey(owner.value));
      const legacy = !raw && owner.value && legacyOwner === owner.value ? localStorage.getItem("shop_cart") : null;
      const saved = raw ? JSON.parse(raw) : { items: legacy ? JSON.parse(legacy) : {} };
      items.value = normalizeCart(saved.items);
      version.value = Number.isSafeInteger(saved.version) && saved.version >= 0 ? saved.version : null;
      for (const key of (Array.isArray(saved.changes) ? saved.changes : legacy ? Object.keys(items.value) : [])) changes.set(key, ++sequence);
      if (saved.pendingCheckout?.requestId && Array.isArray(saved.pendingCheckout.items)) pendingCheckout.value = saved.pendingCheckout;
      if (legacy && persist()) localStorage.removeItem("shop_cart");
    } catch { syncError.value = "购物车缓存损坏，请刷新重试"; }
  }
  watch(owner, loadOwner, { immediate: true, flush: "sync" });

  function fromView(v: CartItemView, previous?: CartLine): CartLine {
    return {
      product: { id: v.productId, name: v.name, price: v.price, originalPrice: previous?.product.originalPrice ?? v.price,
        description: "", sales: 0, emoji: v.emoji, color: v.color, category: "", stock: v.stock },
      skuId: v.skuId, specs: v.specs, quantity: v.quantity, checked: previous?.checked ?? true,
      stock: Math.min(MAX_CART_QUANTITY, v.stock), available: v.available, reason: v.reason,
      priceChanged: previous ? cents(previous.product.price) !== cents(v.price) || previous.priceChanged : false,
    };
  }
  function replaceCart(data: CartResponse) {
    const next: Record<string, CartLine> = {};
    for (const v of data.items) { const key = cartKey(v.productId, v.skuId); next[key] = fromView(v, items.value[key]); }
    items.value = next; version.value = data.version; changes.clear(); persist();
  }
  function updateMetadata(views: CartItemView[]) {
    for (const v of views) {
      const key = cartKey(v.productId, v.skuId), line = items.value[key];
      if (!line) continue;
      const updated = fromView(v, line);
      updated.quantity = line.quantity;
      if (!v.reason || v.reason === "库存不足，请调整数量") {
        updated.available = line.quantity <= updated.stock && updated.stock > 0;
        updated.reason = updated.available ? "" : "库存不足，请调整数量";
      }
      items.value[key] = updated;
    }
    persist();
  }
  async function preview(epoch: number, requestConfig = config()) {
    const response = await http.post<CartItemView[]>("/api/cart/preview", body(), requestConfig);
    if (epoch !== generation) return false;
    updateMetadata(response.data);
    return true;
  }

  // One in-flight write; edits made during it become the next snapshot.
  async function flush(): Promise<boolean> {
    if (!owner.value || pendingCheckout.value) return !pendingCheckout.value;
    if (syncTask) return syncTask;
    const epoch = generation, requestConfig = config();
    const task = (async () => {
      syncing.value = true;
      try {
        if (version.value === null) {
          const { data } = await http.get<CartResponse>("/api/cart", requestConfig);
          if (epoch !== generation) return false;
          const local = items.value, edits = new Map(changes);
          replaceCart(data);
          for (const key of edits.keys()) { if (local[key]) items.value[key] = local[key]; else delete items.value[key]; }
          changes = edits; persist();
        }
        while (changes.size) {
          const sent = new Map(changes);
          const { data } = await http.put<CartResponse>("/api/cart", { version: version.value, items: body() }, requestConfig);
          if (epoch !== generation) return false;
          version.value = data.version;
          for (const [key, seq] of sent) if (changes.get(key) === seq) changes.delete(key);
          updateMetadata(data.items);
        }
        syncError.value = ""; persist();
        return true;
      } catch (error: any) {
        if (epoch !== generation) return false;
        if (error?.response?.status === 409) {
          try {
            const { data } = await http.get<CartResponse>("/api/cart", requestConfig);
            if (epoch !== generation) return false;
            replaceCart(data);
            showToast("购物车已在其他页面或设备更新，已加载最新内容，请重新操作");
          } catch { /* Keep local edits for an explicit retry. */ }
        } else if (error?.response?.status === 400) {
          try { await preview(epoch, requestConfig); } catch { /* Preserve local data. */ }
        }
        if (epoch === generation) syncError.value = error?.response?.data?.message || "购物车同步失败，点击重试";
        return false;
      } finally { if (epoch === generation) syncing.value = false; }
    })();
    syncTask = task;
    try { return await task; } finally { if (syncTask === task) syncTask = null; }
  }

  async function refresh(): Promise<boolean> {
    if (pendingCheckout.value || submitting.value) return false;
    if (refreshTask) return refreshTask;
    const epoch = generation, requestConfig = config();
    const task = (async () => {
      loading.value = true;
      try {
        if (owner.value) {
          if (!await flush() || epoch !== generation) return false;
          const { data } = await http.get<CartResponse>("/api/cart", requestConfig);
          if (epoch !== generation) return false;
          replaceCart(data);
        } else if (!await preview(epoch, requestConfig)) return false;
        syncError.value = "";
        return true;
      } catch { if (epoch === generation) syncError.value = "商品信息刷新失败，点击重试"; return false; }
      finally { if (epoch === generation) loading.value = false; }
    })();
    refreshTask = task;
    try { return await task; } finally { if (refreshTask === task) refreshTask = null; }
  }

  function changed(keys: string[]) {
    for (const key of keys) changes.set(key, ++sequence);
    persist(); void flush();
  }
  function add(product: ShopProduct, skuId = 0, specs = "", price?: number, quantity = 1): boolean {
    if (locked.value) { showToast("请等待购物车操作完成"); return false; }
    const sku = product.skus?.find(it => it.id === skuId);
    const stock = Math.min(MAX_CART_QUANTITY, skuId ? sku?.stock ?? 0 : product.stock ?? 0);
    if (product.onShelf === false || (product.skus?.length && !sku) || stock <= 0) { showToast("商品不可购买，请重新选择规格"); return false; }
    if (!Number.isSafeInteger(quantity) || quantity <= 0) return false;
    const key = cartKey(product.id, skuId), previous = items.value[key];
    const next = (previous?.quantity ?? 0) + quantity;
    if (next > stock) { showToast("购物车数量不能超过库存（" + stock + "件）"); return false; }
    if (!previous && cartItems.value.length >= 100) { showToast("购物车最多支持100种商品"); return false; }
    const currentPrice = sku?.price ?? price ?? product.price;
    items.value[key] = { product: { ...product, detail: undefined, price: currentPrice }, skuId, specs: sku?.specs ?? specs,
      quantity: next, checked: true, stock, available: true, reason: "",
      priceChanged: previous ? cents(previous.product.price) !== cents(currentPrice) : false };
    changed([key]); showToast("已加入购物车"); return true;
  }
  function setQuantity(key: string, value: number | string): boolean {
    if (locked.value) return false;
    const line = items.value[key], quantity = Number(value);
    if (!line || !Number.isSafeInteger(quantity) || quantity < 1 || quantity > MAX_CART_QUANTITY || line.stock <= 0) return false;
    if (quantity > line.stock && quantity >= line.quantity) return false;
    if (line.reason && line.reason !== "库存不足，请调整数量") return false;
    if (line.quantity === quantity) return true;
    line.quantity = quantity; line.available = quantity <= line.stock;
    line.reason = line.available ? "" : "库存不足，请调整数量"; changed([key]); return true;
  }
  function increase(key: string) { const line = items.value[key]; if (line) setQuantity(key, line.quantity + 1); }
  function decrease(key: string) { const line = items.value[key]; if (line) setQuantity(key, line.quantity - 1); }
  function toggleChecked(key: string, includeInvalid = false) {
    const line = items.value[key];
    if (locked.value || !line || (!line.available && !includeInvalid)) return;
    line.checked = !line.checked; persist();
  }
  function setAllChecked(checked: boolean, includeInvalid = false) {
    if (locked.value) return;
    for (const line of cartItems.value) line.checked = checked && (includeInvalid || line.available);
    persist();
  }
  function remove(keys: string[]) { if (locked.value) return; for (const key of keys) delete items.value[key]; changed(keys); }
  function removeChecked() { remove(Object.keys(items.value).filter(key => items.value[key].checked)); }
  function clear() { remove(Object.keys(items.value)); }

  async function mergeFromServer() {
    if (!owner.value || !await refresh()) return;
    const epoch = generation;
    let guest: Record<string, CartLine>;
    try { guest = normalizeCart(JSON.parse(localStorage.getItem(cartCacheKey("")) || "{}").items); } catch { return; }
    if (!Object.keys(guest).length) return;
    loading.value = true;
    try {
      const guestBody = Object.values(guest).map(it => ({ productId: it.product.id, skuId: it.skuId, quantity: it.quantity }));
      const { data } = await http.post<CartItemView[]>("/api/cart/preview", guestBody, config());
      if (epoch !== generation) return;
      const merged: string[] = [];
      for (const v of data) {
        if (!v.available) continue;
        const key = cartKey(v.productId, v.skuId), existing = items.value[key];
        if (!existing && cartItems.value.length >= 100) continue;
        items.value[key] = fromView({ ...v, quantity: Math.min(v.stock, MAX_CART_QUANTITY, Math.max(v.quantity, existing?.quantity ?? 0)) }, existing || guest[key]);
        merged.push(key);
      }
      for (const key of merged) changes.set(key, ++sequence);
      persist();
      if (await flush() && epoch === generation) {
        for (const key of merged) delete guest[key];
        localStorage.setItem(cartCacheKey(""), JSON.stringify({ items: guest }));
      }
    } catch { if (epoch === generation) syncError.value = "游客购物车合并失败，商品已保留"; }
    finally { if (epoch === generation) loading.value = false; }
  }

  async function checkout(addressId: number, userCouponId: number, expectedAmount: number) {
    if (submitting.value || loading.value || !owner.value) throw new Error("请稍后重试");
    const epoch = generation, requestConfig = config();
    submitting.value = true;
    try {
      if (!pendingCheckout.value) {
        if (!await flush() || epoch !== generation) throw new Error(syncError.value || "购物车同步失败");
        if (!checkedItems.value.length) throw new Error("请选择可购买的商品");
        pendingCheckout.value = { requestId: crypto.randomUUID(), version: version.value!, addressId, userCouponId,
          expectedAmount, items: checkedItems.value.map(it => ({ productId: it.product.id, skuId: it.skuId, quantity: it.quantity, price: it.product.price })) };
        if (!persist()) { pendingCheckout.value = null; throw new Error("无法保存订单请求，请检查本地存储后重试"); }
      }
      const { data } = await http.post<{ order: { id: number }; cart: CartResponse }>("/api/cart/checkout", pendingCheckout.value, requestConfig);
      if (epoch !== generation) throw new Error("账号已切换，请在原账号查看订单");
      pendingCheckout.value = null; replaceCart(data.cart);
      return data.order;
    } catch (error: any) {
      if (epoch === generation && [400, 409, 422].includes(error?.response?.status)) { pendingCheckout.value = null; persist(); }
      throw error;
    } finally { if (epoch === generation) submitting.value = false; }
  }
  const retry = () => { if (!pendingCheckout.value) void refresh(); };
  if (typeof window !== "undefined") {
    window.addEventListener("online", retry);
    onScopeDispose(() => window.removeEventListener("online", retry));
  }
  return { items, cartItems, total, checkedItems, checkedCount, checkedTotal, selectedCount, allChecked, allSelected,
    syncing, loading, locked, submitting, syncError, pendingCheckout, addressStorageKey,
    add, increase, decrease, setQuantity, toggleChecked, setAllChecked, remove, removeChecked, clear,
    mergeFromServer, refresh, flush, checkout };
});
