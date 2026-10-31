<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from "vue";
import { showToast } from "vant";
import type { IDomEditor, Toolbar } from "@wangeditor/core";
import { createProductEditor, createProductToolbar } from "../lib/rich-editor";
import "@wangeditor/editor/dist/css/style.css";
import http from "../lib/http";
import { avatarSrc, resolveDetailHtml, unresolveDetailHtml } from "../lib/avatar";

const props = defineProps<{ modelValue: string }>();
const emit = defineEmits<{ (e: "update:modelValue", v: string): void }>();

const toolbarEl = ref<HTMLElement>();
const editorEl = ref<HTMLElement>();
let editor: IDomEditor | null = null;
let toolbar: Toolbar | null = null;

onMounted(() => {
  editor = createProductEditor(editorEl.value!, resolveDetailHtml(props.modelValue), {
      placeholder: "请输入图文详情",
      MENU_CONF: {
        uploadImage: {
          customUpload(file: File, insertFn: (src: string, alt: string, href: string) => void) {
            const fd = new FormData();
            fd.append("file", file);
            http
              .post("/api/admin/products/upload", fd)
              .then((res) => insertFn(avatarSrc(res.data.url), "", ""))
              .catch((err) => {
                const e = err as { response?: { data?: { message?: string } } };
                showToast(e?.response?.data?.message || "图片上传失败");
              });
          },
        },
      },
      onChange(ed) {
        emit("update:modelValue", unresolveDetailHtml(ed.getHtml()));
      },
  });
  toolbar = createProductToolbar(editor, toolbarEl.value!);
});

onBeforeUnmount(() => {
  toolbar?.destroy();
  editor?.destroy();
  editor = null;
  toolbar = null;
});

// 暴露 getHtml 供父组件在保存时直接读取当前内容，避免仅依赖 onChange 的时序问题。
defineExpose({
  getHtml: () => (editor ? unresolveDetailHtml(editor.getHtml()) : ""),
});
</script>

<template>
  <div class="rich-editor">
    <div ref="toolbarEl" class="rich-toolbar"></div>
    <div ref="editorEl" class="rich-content"></div>
  </div>
</template>

<style scoped>
.rich-editor { width: 100%; border: 1px solid #ebedf0; border-radius: 6px; }
.rich-toolbar { border-bottom: 1px solid #ebedf0; }
.rich-content { min-height: 220px; overflow-y: auto; }
</style>
