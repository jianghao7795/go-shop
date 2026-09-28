<script setup lang="ts">
import { ref } from "vue";
import { useRouter } from "vue-router";
import { showToast, showImagePreview } from "vant";
import { useUserStore } from "../stores/user";
import http from "../lib/http";
import { avatarSrc } from "../lib/avatar";

const router = useRouter();
const userStore = useUserStore();

const nickname = ref(userStore.nickname);
const avatar = ref(userStore.avatar);
const mobile = ref(userStore.mobile);
const email = ref(userStore.email);
const loading = ref(false);
const uploading = ref(false);

const presetAvatars = ["🐱", "🐶", "🦊", "🐼", "🐰", "🦁", "🐯", "🐸"];

function onPickFile() {
  const input = document.createElement("input");
  input.type = "file";
  input.accept = "image/jpeg,image/png,image/gif,image/webp";
  input.onchange = async () => {
    const file = input.files?.[0];
    if (!file) return;
    const form = new FormData();
    form.append("file", file);
    uploading.value = true;
    try {
      const res = await http.post("/api/profile/avatar", form);
      avatar.value = res.data.url;
      showToast("头像已上传");
    } catch (err) {
      const e = err as any;
      showToast(e?.response?.data?.message || "上传失败");
    } finally {
      uploading.value = false;
    }
  };
  input.click();
}

function pickPreset(emoji: string) {
  avatar.value = emoji;
}

function previewAvatar() {
  const src = avatarSrc(avatar.value);
  if (src) showImagePreview({ images: [src], closeable: true, closeOnClickImage: true });
}

const mobileRule = {
  validator: (val: string) => !val || /^1[3-9]\d{9}$/.test(val),
  message: "手机号格式不正确",
};
const emailRule = {
  validator: (val: string) => !val || /^[^@\s]+@[^@\s]+\.[^@\s]+$/.test(val),
  message: "邮箱格式不正确",
};

async function onSubmit() {
  loading.value = true;
  try {
    await userStore.saveProfile({
      nickname: nickname.value,
      avatar: avatar.value,
      mobile: mobile.value,
      email: email.value,
    });
    showToast("保存成功");
    router.back();
  } catch (err) {
    const e = err as any;
    if (e?.response) showToast(e.response.data?.message || "保存失败");
    else showToast("保存失败，请稍后重试");
  } finally {
    loading.value = false;
  }
}
</script>

<template>
  <div class="sub-page">
    <van-nav-bar title="编辑资料" left-arrow @click-left="$router.back()" />
    <van-form @submit="onSubmit">
      <van-cell-group inset class="avatar-group">
        <div class="avatar-editor">
          <div class="avatar-preview" @click="previewAvatar">
            <img v-if="avatarSrc(avatar)" :src="avatarSrc(avatar)" alt="头像" />
            <span v-else>{{ avatar || "👤" }}</span>
          </div>
          <div class="avatar-btns">
            <van-button size="small" type="primary" :loading="uploading" @click="onPickFile">上传头像</van-button>
            <van-button size="small" plain @click="avatar = ''">恢复默认</van-button>
          </div>
        </div>
        <div class="preset-list">
          <span
            v-for="e in presetAvatars"
            :key="e"
            class="preset-avatar"
            :class="{ active: avatar === e }"
            @click="pickPreset(e)"
          >{{ e }}</span>
        </div>
      </van-cell-group>
      <van-cell-group inset>
        <van-field v-model="nickname" name="nickname" label="昵称" placeholder="请输入昵称" clearable />
        <van-field v-model="mobile" name="mobile" label="手机号" placeholder="请输入手机号" clearable :rules="[mobileRule]" />
        <van-field v-model="email" name="email" label="邮箱" placeholder="请输入邮箱" clearable :rules="[emailRule]" />
      </van-cell-group>
      <div class="login-submit">
        <van-button round block type="danger" native-type="submit" :loading="loading">保存</van-button>
      </div>
    </van-form>
  </div>
</template>

<style scoped>
.avatar-group {
  padding: 16px;
}
.avatar-editor {
  display: flex;
  align-items: center;
  gap: 16px;
}
.avatar-preview {
  width: 64px;
  height: 64px;
  border-radius: 50%;
  background: #f5f6f8;
  display: flex;
  align-items: center;
  justify-content: center;
  overflow: hidden;
  font-size: 36px;
  flex-shrink: 0;
  cursor: pointer;
}
.avatar-preview img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}
.avatar-btns {
  display: flex;
  gap: 8px;
}
.preset-list {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  margin-top: 14px;
}
.preset-avatar {
  width: 40px;
  height: 40px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 24px;
  border-radius: 50%;
  background: #f5f6f8;
  cursor: pointer;
}
.preset-avatar.active {
  outline: 2px solid #ff4d67;
  outline-offset: 2px;
}
</style>
