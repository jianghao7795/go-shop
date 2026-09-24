// errMsg 从 axios 错误中取出后端返回的 message，取不到时回退到给定提示。
export function errMsg(err: unknown, fallback: string): string {
  const e = err as { response?: { data?: { message?: string } } };
  return e?.response?.data?.message || fallback;
}
