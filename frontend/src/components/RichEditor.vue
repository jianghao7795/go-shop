<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from "vue";
import { showToast } from "vant";
import { createEditor, createToolbar, type IDomEditor, type Toolbar } from "@wangeditor/editor";
import "@wangeditor/editor/dist/css/style.css";
import http from "../lib/http";
import { avatarSrc } from "../lib/avatar";

const props = defineProps<{ modelValue: string }>();
const emit = defineEmits<{ (e: "update:modelValue", v: string): void }>();

const toolbarEl = ref<HTMLElement>();
const editorEl = ref<HTMLElement>();
let editor: IDomEditor | null = null;
let toolbar: Toolbar | null = null;

onMounted(() => {
  editor = createEditor({
    selector: editorEl.value!,
    html: props.modelValue,
    mode: "default",
    config: {
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
        emit("update:modelValue", ed.getHtml());
      },
    },
  });
  toolbar = createToolbar({
    editor,
    selector: toolbarEl.value!,
    mode: "default",
  });
});

onBeforeUnmount(() => {
  toolbar?.destroy();
  editor?.destroy();
  editor = null;
  toolbar = null;
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
