const apiBase = () => import.meta.env.VITE_API_BASE || "http://127.0.0.1:8080";

// avatarSrc 把头像值解析成可直接用于 <img src> 的完整 URL；
// 若不是图片（emoji 或空）返回空字符串，交由调用方按 emoji/默认头像展示。
export function avatarSrc(avatar: string): string {
  if (!avatar) return "";
  if (avatar.startsWith("http://") || avatar.startsWith("https://")) return avatar;
  if (avatar.startsWith("/uploads")) return apiBase() + avatar;
  return "";
}

// resolveDetailHtml 把详情 HTML 里相对的 /uploads/... 图片/链接 src 解析成完整 URL，供 v-html 渲染。
export function resolveDetailHtml(html: string): string {
  if (!html) return "";
  const base = apiBase();
  return html.replace(/(src|href)="(\/uploads\/[^"]*)"/g, (_, attr, path) => `${attr}="${base}${path}"`);
}

// unresolveDetailHtml 把详情 HTML 里的完整 /uploads/... URL 还原为相对路径，用于存库（与 host 无关）。
export function unresolveDetailHtml(html: string): string {
  if (!html) return "";
  return html.split(apiBase() + "/uploads/").join("/uploads/");
}
