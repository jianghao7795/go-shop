export interface OrderItem {
  productId: number; skuId: number; specs: string; name: string;
  price: number; quantity: number; emoji: string; color: string;
}

export interface Order {
  id: number; orderNo: string; userId: string; status: string; amount: number;
  receiver: string; phone: string; region: string; detail: string;
  items: OrderItem[]; createdAt: string; paidAt: string | null;
  carrier: string; trackingNo: string; shippedAt: string | null;
  logistics: { time: string; text: string }[] | null;
  aftersaleType: string; aftersaleReason: string; aftersaleStatus: string;
  aftersaleNote: string; returnAddress: string; returnCarrier: string;
  returnTrackingNo: string; refundedAt: string | null;
}

export const orderStatuses: Record<string, { text: string; color: string }> = {
  pending: { text: "待付款", color: "#ff976a" },
  paid: { text: "待发货", color: "#7c4dff" },
  shipped: { text: "待收货", color: "#1989fa" },
  completed: { text: "待评价", color: "#07c160" },
  aftersale: { text: "售后处理中", color: "#ee0a24" },
  finished: { text: "已完成", color: "#969ba5" },
  cancelled: { text: "已取消", color: "#969ba5" },
  refunded: { text: "已退款", color: "#969ba5" },
};

export const aftersaleStatuses: Record<string, string> = {
  requested: "等待审核", returning: "等待退货并验收", refunding: "退款确认中",
  rejected: "申请已驳回", refunded: "退款已完成",
};

export function canApplyAftersale(order: Order): boolean {
  return ["paid", "shipped", "completed", "finished"].includes(order.status);
}
