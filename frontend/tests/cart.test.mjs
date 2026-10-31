import assert from "node:assert/strict";
import { after, test } from "node:test";
import { readFileSync } from "node:fs";
import { createRequire } from "node:module";
import ts from "typescript";
import { Window } from "happy-dom";

const require = createRequire(import.meta.url);
const window = new Window({ url: "http://localhost" });
const originals = new Map();
for (const name of ["window", "document", "navigator", "Node", "Element", "HTMLElement", "HTMLInputElement", "SVGElement", "Event", "MouseEvent", "getComputedStyle", "requestAnimationFrame", "cancelAnimationFrame", "setTimeout", "clearTimeout", "setInterval", "clearInterval"]) {
  originals.set(name, Object.getOwnPropertyDescriptor(globalThis, name));
  Object.defineProperty(globalThis, name, { value: name === "window" ? window : window[name], writable: true, configurable: true });
}
const vue = require("vue");
const pinia = require("pinia");
const vant = require("vant");
const compiler = require("@vue/compiler-sfc");
const read = name => readFileSync(new URL("../src/" + name, import.meta.url), "utf8");
const compile = code => ts.transpileModule(code, { compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.CommonJS, esModuleInterop: true } }).outputText;
const userCode = compile(read("stores/user.ts"));
const cartCode = compile(read("stores/shop.ts"));
const componentCode = name => compile(compiler.compileScript(compiler.parse(read(name)).descriptor, { id: name, inlineTemplate: true }).content);
const quantityCode = componentCode("components/CartQuantity.vue");
const viewCode = componentCode("views/CartView.vue");
const product = { id: 1, name: "测试商品", description: "", price: 10, originalPrice: 10, sales: 0, emoji: "", color: "", category: "", stock: 10 };
const clone = value => JSON.parse(JSON.stringify(value));
const settle = async () => { for (let i = 0; i < 12; i++) await Promise.resolve(); await vue.nextTick(); };
const deferred = () => { let resolve, reject; const promise = new Promise((a, b) => { resolve = a; reject = b; }); return { promise, resolve, reject }; };
const fixtures = [];

function fixture(seed = {}) {
  const saved = new Map(Object.entries(seed));
  const storage = { getItem: key => saved.get(key) ?? null, setItem: (key, value) => saved.set(key, String(value)), removeItem: key => saved.delete(key) };
  const instance = pinia.createPinia();
  pinia.setActivePinia(instance);
  const state = { requests: [], servers: new Map(), catalog: new Map([[1, clone(product)]]), hooks: {}, toasts: [], confirmations: 0 };
  const server = token => {
    if (!state.servers.has(token)) state.servers.set(token, { version: 0, items: [] });
    return state.servers.get(token);
  };
  const view = row => {
    const p = state.catalog.get(row.productId);
    return { ...row, name: p?.name || "已失效商品", price: p?.price || 0, specs: "", emoji: "", color: "", stock: p?.stock || 0,
      available: !!p && row.quantity <= p.stock, reason: !p ? "商品已删除" : row.quantity > p.stock ? "库存不足，请调整数量" : "" };
  };
  const response = token => ({ data: { version: server(token).version, items: server(token).items.map(view) } });
  const write = (body, config) => {
    const target = server(config.headers.Authorization);
    if (body.version !== target.version) throw { response: { status: 409, data: { message: "购物车已更新" } } };
    target.version++; target.items = clone(body.items);
    return response(config.headers.Authorization);
  };
  const http = {
    async get(url, config) {
      state.requests.push({ method: "GET", url, config: clone(config) });
      if (state.hooks.get) return state.hooks.get(url, config);
      return response(config.headers.Authorization);
    },
    async put(url, body, config) {
      body = clone(body);
      state.requests.push({ method: "PUT", url, body, config: clone(config) });
      return state.hooks.put ? state.hooks.put(body, config) : write(body, config);
    },
    async post(url, body, config) {
      body = clone(body);
      state.requests.push({ method: "POST", url, body, config: clone(config) });
      if (url.endsWith("preview")) return { data: body.map(view) };
      if (state.hooks.checkout) return state.hooks.checkout(body, config);
      throw new Error("Unexpected checkout");
    },
  };
  const mocks = {
    "../lib/http": { __esModule: true, default: http },
    vant: { showToast: text => state.toasts.push(text), showConfirmDialog: async () => { state.confirmations++; } },
    "vue-router": { useRouter: () => ({ push() {} }) },
  };
  const evaluate = code => {
    const mod = { exports: {} };
    new Function("require", "module", "exports", "localStorage", code)(id => Object.hasOwn(mocks, id) ? mocks[id] : require(id), mod, mod.exports, storage);
    return mod.exports;
  };
  const users = evaluate(userCode);
  mocks["./user"] = users;
  const carts = evaluate(cartCode);
  mocks["../stores/shop"] = carts;
  mocks["../components/CartQuantity.vue"] = evaluate(quantityCode);
  const f = { saved, storage, instance, state, server, write, response, user: users.useUserStore(), cart: carts.useShopCart(), exports: carts,
    mount() {
      pinia.setActivePinia(instance);
      const host = document.createElement("div"); document.body.append(host);
      const app = vue.createApp(evaluate(viewCode).default); app.use(instance); app.use(vant); app.mount(host);
      return { host, close() { app.unmount(); host.remove(); } };
    },
  };
  fixtures.push(f);
  return f;
}
after(async () => {
  for (const f of fixtures) pinia.disposePinia(f.instance);
  // All component timers belong to this DOM, so abort cancels delayed layout
  // work before the global browser objects are restored.
  await window.happyDOM.abort();
  for (const [name, descriptor] of originals) { if (descriptor) Object.defineProperty(globalThis, name, descriptor); else delete globalThis[name]; }
});

test("logout and account changes isolate local and remote carts", async () => {
  const f = fixture();
  f.user.setAuth("alice-token", "alice");
  f.cart.add(product);
  assert.equal(await f.cart.flush(), true);
  f.user.logout();
  assert.equal(f.cart.cartItems.length, 0);
  f.user.setAuth("bob-token", "bob");
  await f.cart.mergeFromServer();
  assert.equal(f.cart.cartItems.length, 0);
  assert.equal(f.server("Bearer bob-token").items.length, 0);
  f.user.setAuth("alice-token", "alice");
  await f.cart.refresh();
  assert.equal(f.cart.cartItems[0].quantity, 1);
});

test("legacy keys migrate with known ownership; unowned data never follows the next login", async () => {
  const old = JSON.stringify({ "1": { product, quantity: 2 } });
  const f = fixture({ shop_user: "alice", shop_token: "alice-token", shop_cart: old });
  assert.equal(f.cart.items["1:0"].quantity, 2);
  assert.equal(f.cart.setQuantity("1:0", 3), true);
  await f.cart.flush();
  assert.equal(f.server("Bearer alice-token").items[0].quantity, 3);
  f.cart.remove(["1:0"]); await f.cart.flush();
  assert.equal(f.cart.cartItems.length, 0);
  const guest = fixture({ shop_cart: old });
  guest.user.setAuth("bob-token", "bob");
  await guest.cart.mergeFromServer();
  assert.equal(guest.cart.cartItems.length, 0);
  assert.equal(guest.storage.getItem("shop_cart:legacy-unassigned"), old);
});

test("guest cart merges once without resurrecting items deleted on the server", async () => {
  const f = fixture();
  f.cart.add(product, 0, "", undefined, 2);
  f.user.setAuth("alice-token", "alice");
  await f.cart.mergeFromServer();
  assert.equal(f.cart.cartItems[0].quantity, 2);
  f.server("Bearer alice-token").items = [];
  f.server("Bearer alice-token").version++;
  await f.cart.mergeFromServer();
  assert.equal(f.cart.cartItems.length, 0);
});

test("a delayed response cannot load the previous user's cart into another account", async () => {
  const f = fixture();
  const old = deferred();
  f.user.setAuth("alice-token", "alice");
  f.state.hooks.get = (_url, config) => config.headers.Authorization === "Bearer alice-token" ? old.promise : f.response(config.headers.Authorization);
  const readingAlice = f.cart.refresh();
  f.user.setAuth("bob-token", "bob");
  await f.cart.refresh();
  old.resolve({ data: { version: 5, items: [{ ...product, productId: 1, skuId: 0, quantity: 8, specs: "", available: true, reason: "" }] } });
  await readingAlice;
  assert.equal(f.cart.cartItems.length, 0);
});

test("rapid edits are serialized and an older acknowledgement cannot undo the new quantity", async () => {
  const f = fixture();
  f.user.setAuth("alice-token", "alice"); await f.cart.refresh();
  const pending = [];
  f.state.hooks.put = (body, config) => { const wait = deferred(); pending.push({ body, finish: () => wait.resolve(f.write(body, config)) }); return wait.promise; };
  f.cart.add(product);
  f.cart.increase("1:0"); f.cart.increase("1:0");
  assert.equal(pending.length, 1);
  pending[0].finish(); await settle();
  assert.equal(f.cart.items["1:0"].quantity, 3);
  assert.equal(pending.length, 2);
  assert.equal(pending[1].body.items[0].quantity, 3);
  assert.equal(pending[1].body.version, 1);
  pending[1].finish(); await f.cart.flush();
  assert.equal(f.server("Bearer alice-token").items[0].quantity, 3);
  const writes = f.state.requests.filter(r => r.method === "PUT").length;
  f.cart.toggleChecked("1:0"); f.cart.setAllChecked(true); await settle();
  assert.equal(f.state.requests.filter(r => r.method === "PUT").length, writes);
});

test("network errors preserve edits for retry; version conflicts load authoritative data", async () => {
  const f = fixture();
  f.user.setAuth("alice-token", "alice"); await f.cart.refresh();
  f.state.hooks.put = () => { throw new Error("offline"); };
  f.cart.add(product); assert.equal(await f.cart.flush(), false);
  assert.ok(f.cart.syncError); assert.equal(f.cart.cartItems[0].quantity, 1);
  delete f.state.hooks.put;
  assert.equal(await f.cart.refresh(), true);
  const remote = f.server("Bearer alice-token"); remote.version++; remote.items[0].quantity = 4;
  f.cart.increase("1:0"); assert.equal(await f.cart.flush(), false);
  assert.equal(f.cart.cartItems[0].quantity, 4);
  assert.ok(f.state.toasts.some(text => text.includes("其他页面或设备")));
});

test("quantity and price edits use current product data and enforce integer stock limits", () => {
  const f = fixture();
  f.cart.add({ ...product, stock: 2 });
  f.cart.add({ ...product, stock: 2, price: 20 });
  assert.equal(f.cart.cartItems[0].product.price, 20);
  assert.equal(f.cart.checkedTotal, 40);
  assert.equal(f.cart.add({ ...product, stock: 2 }), false);
  for (const quantity of [0, -1, 1.5, 3, NaN]) assert.equal(f.cart.setQuantity("1:0", quantity), false);
  assert.equal(f.cart.cartItems[0].quantity, 2);
});

test("real CartView input updates totals and a one-item row can be deleted", async () => {
  const f = fixture(); f.cart.add(product);
  const mounted = f.mount();
  try {
    await settle();
    const input = mounted.host.querySelector(".van-stepper__input");
    input.value = "7";
    input.dispatchEvent(new window.Event("input", { bubbles: true }));
    input.dispatchEvent(new window.Event("blur"));
    await settle();
    assert.equal(input.value, "7");
    assert.equal(f.cart.cartItems[0].quantity, 7);
    assert.equal(f.cart.checkedTotal, 70);
    f.cart.setQuantity("1:0", 1); await settle();
    const remove = [...mounted.host.querySelectorAll("button")].find(button => button.textContent.trim() === "删除");
    remove.click(); await settle();
    assert.equal(f.state.confirmations, 1);
    assert.equal(f.cart.cartItems.length, 0);
  } finally { mounted.close(); }
});

test("invalid products remain removable and cannot be included in checkout totals", async () => {
  const f = fixture(); f.cart.add(product);
  f.state.catalog.delete(1);
  await f.cart.refresh();
  assert.equal(f.cart.cartItems.length, 1);
  assert.equal(f.cart.checkedCount, 0);
  assert.equal(f.cart.checkedTotal, 0);
  assert.equal(f.cart.cartItems[0].reason, "商品已删除");
  f.cart.remove(["1:0"]); assert.equal(f.cart.cartItems.length, 0);
});

test("lost checkout response retains a frozen request across reload and retries the same order", async () => {
  const f = fixture(); f.user.setAuth("alice-token", "alice");
  await f.cart.refresh(); f.cart.add(product, 0, "", undefined, 2); await f.cart.flush();
  let submitted;
  f.state.hooks.checkout = body => { submitted = clone(body); throw new Error("response lost after commit"); };
  await assert.rejects(f.cart.checkout(1, 0, 20));
  assert.ok(f.cart.pendingCheckout?.requestId);
  assert.equal(f.cart.setQuantity("1:0", 1), false);
  f.cart.remove(["1:0"]); assert.equal(f.cart.cartItems[0].quantity, 2);
  const reloaded = fixture(Object.fromEntries(f.saved));
  reloaded.state.hooks.checkout = body => {
    assert.deepEqual(body, submitted);
    return { data: { order: { id: 42 }, cart: { version: 2, items: [] } } };
  };
  const order = await reloaded.cart.checkout(999, 888, 999);
  assert.equal(order.id, 42);
  assert.equal(reloaded.cart.pendingCheckout, null);
  assert.equal(reloaded.cart.cartItems.length, 0);
});
