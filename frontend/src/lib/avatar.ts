const apiBase = () => import.meta.env.VITE_API_BASE || "http://127.0.0.1:8080";

// avatarSrc 把头像值解析成可直接用于 <img src> 的完整 URL；
// 若不是图片（emoji 或空）返回空字符串，交由调用方按 emoji/默认头像展示。
export function avatarSrc(avatar: string): string {
  if (!avatar) return "";
  if (avatar.startsWith("http://") || avatar.startsWith("https://")) return avatar;
  if (avatar.startsWith("/uploads")) return apiBase() + avatar;
  return "";
}
