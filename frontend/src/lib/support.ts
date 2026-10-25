export interface SupportTicket {
  id: number;
  subject: string;
  status: "open" | "closed";
  userId: string;
  mode: "bot" | "waiting" | "human";
  humanRequestedAt: string | null;
  updatedAt: string;
}

export interface SupportMessage {
  id: number;
  staff: boolean;
  bot: boolean;
  content: string;
  createdAt: string;
}

export const supportModeLabels: Record<SupportTicket["mode"], string> = {
  bot: "自动应答", waiting: "等待人工", human: "人工服务中",
};

export function supportStatusLabel(ticket: SupportTicket): string {
  return ticket.status === "closed" ? "已结束" : supportModeLabels[ticket.mode] || "处理中";
}
