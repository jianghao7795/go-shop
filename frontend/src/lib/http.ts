import axios from "axios";

// http 是共享的 axios 实例，统一 baseURL 与鉴权 header。
const http = axios.create({
  baseURL: import.meta.env.VITE_API_BASE || "http://127.0.0.1:8080",
});

http.interceptors.request.use((config) => {
  const token = localStorage.getItem("shop_token");
  if (token) {
    config.headers.set("Authorization", "Bearer " + token);
  }
  return config;
});

export default http;
