import assert from "node:assert/strict";
import { after, before, test } from "node:test";
import { mkdtemp, readdir, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import path from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";
import { Window } from "happy-dom";
import { build } from "vite";
import config from "../vite.config.ts";

let outputDir;
let window;
let createProductEditor;
let createProductToolbar;
const originals = new Map();

before(async () => {
  // Exercise the same split-chunk policy used in production, including loading
  // the generated modules. No application server or production database is used.
  outputDir = await mkdtemp(path.join(tmpdir(), "shop-editor-test-"));
  await build({
    configFile: false,
    logLevel: "silent",
    build: {
      ...config.build,
      outDir: outputDir,
      emptyOutDir: false,
      lib: {
        entry: fileURLToPath(new URL("../src/lib/rich-editor.ts", import.meta.url)),
        formats: ["es"],
      },
      rolldownOptions: {
        ...config.build.rolldownOptions,
        output: {
          ...config.build.rolldownOptions.output,
          entryFileNames: "editor.mjs",
          chunkFileNames: "[name]-[hash].mjs",
        },
      },
    },
  });
  window = new Window({ url: "http://localhost" });
  const globals = ["window", "document", "navigator", "Node", "Element", "HTMLElement", "HTMLInputElement", "HTMLTextAreaElement", "ShadowRoot", "Text", "Document", "DocumentFragment", "MutationObserver", "ResizeObserver", "DOMParser", "Event", "CustomEvent", "KeyboardEvent", "MouseEvent", "File", "FileReader", "Blob", "XMLHttpRequest", "localStorage", "getComputedStyle", "requestAnimationFrame", "cancelAnimationFrame"];
  for (const name of globals) {
    originals.set(name, Object.getOwnPropertyDescriptor(globalThis, name));
    const value = window[name];
    Object.defineProperty(globalThis, name, { value: name === "window" ? window : value, writable: true, configurable: true });
  }
  ({ createProductEditor, createProductToolbar } = await import(pathToFileURL(path.join(outputDir, "editor.mjs")).href));
});

after(async () => {
  if (window) await window.happyDOM.abort();
  for (const [name, descriptor] of originals) {
    if (descriptor) Object.defineProperty(globalThis, name, descriptor);
    else delete globalThis[name];
  }
  if (outputDir) {
    assert.equal(path.dirname(path.resolve(outputDir)), path.resolve(tmpdir()));
    assert.ok(path.basename(outputDir).startsWith("shop-editor-test-"));
    await rm(outputDir, { recursive: true, force: true });
  }
});

async function settleEditor() {
  await new Promise(resolve => setTimeout(resolve, 30));
  await window.happyDOM.whenAsyncComplete();
}

async function mountEditor(html, options = {}) {
  const toolbarHost = document.createElement("div");
  const editorHost = document.createElement("div");
  document.body.append(toolbarHost, editorHost);
  const editor = createProductEditor(editorHost, html, { autoFocus: false, ...options });
  const toolbar = createProductToolbar(editor, toolbarHost);
  await settleEditor();
  return {
    editor, toolbarHost,
    async destroy() { await settleEditor(); toolbar.destroy(); editor.destroy(); toolbarHost.remove(); editorHost.remove(); },
  };
}

test("editor chunks preserve product descriptions, formatting, images and tables", async () => {
  assert.ok((await readdir(outputDir)).filter(name => name.endsWith(".mjs")).length > 1, "editor dependencies should be split");
  const fixture = '<h2>商品说明</h2><p><strong>优质面料</strong>，<a href="https://example.com">查看说明</a></p><p><img src="https://example.com/product.png" alt="商品图片" /></p><ul><li>柔软</li><li>耐用</li></ul><table><tbody><tr><td>尺寸</td><td>M</td></tr></tbody></table>';
  const mounted = await mountEditor(fixture);
  try {
    const content = document.createElement("div");
    content.innerHTML = mounted.editor.getHtml();
    assert.equal(content.querySelector("h2")?.textContent, "商品说明");
    assert.equal(content.querySelector("strong")?.textContent, "优质面料");
    assert.equal(content.querySelector("a")?.getAttribute("href"), "https://example.com");
    assert.equal(content.querySelector("img")?.getAttribute("src"), "https://example.com/product.png");
    assert.equal(content.querySelectorAll("li").length, 2);
    assert.equal(content.querySelectorAll("td").length, 2);
    const menus = mounted.editor.getAllMenuKeys();
    for (const key of ["bold", "uploadImage", "insertTable", "bulletedList", "codeBlock", "insertVideo", "undo", "redo"]) assert.ok(menus.includes(key), "missing menu: " + key);
    assert.ok(mounted.toolbarHost.querySelectorAll("button").length > 10);
  } finally { await mounted.destroy(); }
});

test("editor can be reopened and uses the configured image upload handler", async () => {
  const upload = () => {};
  for (let i = 0; i < 2; i++) {
    const mounted = await mountEditor("<p>原始内容</p>", { MENU_CONF: { uploadImage: { customUpload: upload } } });
    try {
      assert.equal(mounted.editor.getMenuConfig("uploadImage").customUpload, upload);
      mounted.editor.setHtml("<p>修改后的商品详情</p>");
      await settleEditor();
      assert.ok(mounted.editor.getHtml().includes("修改后的商品详情"));
    } finally { await mounted.destroy(); }
  }
});
