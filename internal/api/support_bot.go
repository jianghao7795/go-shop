package api

import (
	"strings"
	"time"
	"unicode"

	"gorm.io/gorm"
	"shop/internal/model"
)

const supportHandoffHint = "如需人工帮助，请发送“联系人工”。"

// Only an explicit command transfers the conversation; questions mentioning the
// command or phrases such as “不要联系人工” must remain in automatic mode.
func requestsHumanSupport(content string) bool {
	return strings.TrimFunc(content, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsPunct(r) }) == "联系人工"
}

// Automatic replies describe supported workflows without inventing delivery
// promises, order details, refund results, or staff availability.
func supportAutoAnswer(content string) string {
	contains := func(words ...string) bool {
		for _, word := range words {
			if strings.Contains(content, word) {
				return true
			}
		}
		return false
	}
	answer := "暂时没有找到这个问题的答案。你可以继续询问订单、付款、物流、售后、地址、优惠券或商品规格。"
	switch {
	case contains("人工"):
		answer = "当前由自动客服为你解答，发送“联系人工”后才会转入人工客服队列，并保留本次咨询记录。"
	case contains("取消订单", "关闭订单", "不想买", "不要了"):
		answer = "待付款订单可以在「我的 → 我的订单 → 订单详情」点击「取消订单」。已付款订单请点击「申请售后」，选择仅退款并填写原因，等待商家审核。"
	case contains("退款", "退货", "售后", "换货", "损坏", "破损"):
		answer = "进入「我的 → 我的订单 → 订单详情」，点击「申请售后」并填写原因。未发货可申请仅退款；退货退款审核通过后，按页面上的退货地址寄回并填写运单。退款进度可在订单详情查看，目前支持整单退款。"
	case contains("地址", "收货人", "手机号", "电话"):
		answer = "在「我的 → 收货地址」可新增、编辑、删除或设置默认地址。修改地址簿不会改变已提交订单中的收货信息，已下单的地址需要联系人工核实。"
	case contains("优惠券", "领券", "折扣", "满减"):
		answer = "在「我的 → 优惠券」查看或领取优惠券，结算时选择符合金额门槛和有效期的券。每单可选择一张优惠券，优惠金额会在提交订单前显示。"
	case contains("付款", "支付", "支付宝", "扣款"):
		answer = "在待付款订单中点击「去支付」，使用支付宝扫描收款码。已付款但页面未更新时，点击「已付款？点此刷新」核对状态。订单付款期限为创建后 30 分钟，请以订单详情中的状态为准。"
	case contains("物流", "快递", "运单", "配送", "发货", "到货", "送达", "收货"):
		answer = "进入「我的 → 我的订单 → 订单详情」查看发货状态。商家发货后会显示快递公司、单号和商家更新的物流进度；具体发货及送达时间需要商家确认。收到商品后可在订单详情确认收货。"
	case contains("规格", "尺寸", "颜色", "库存", "缺货", "售罄"):
		answer = "在商品详情选择规格后，可查看对应价格和库存。售罄规格无法购买；多规格商品需先选择规格，再加入购物车或立即购买。"
	case contains("购物车", "结算", "下单", "购买"):
		answer = "在商品详情选择规格和数量后加入购物车，勾选商品进入结算，确认收货地址和优惠券后提交订单。也可以点击「立即购买」进入结算。"
	case contains("评价", "评分"):
		answer = "确认收货后，订单会进入「待评价」。在订单列表或详情点击「评价」即可填写评分和使用感受。"
	case contains("订单", "订单号"):
		answer = "进入「我的 → 我的订单」可按待付款、待发货、待收货等状态查看订单，点击订单可查看商品、收货信息、物流与售后进度。"
	case contains("你好", "您好", "在吗", "帮助"):
		answer = "你好，我是自动客服，可以解答订单、付款、物流、售后、地址、优惠券和商品规格等常见问题。请直接发送你的问题。"
	}
	return answer + "\n" + supportHandoffHint
}

// appendSupportAutoReply runs in the same transaction as the customer's message
// and mode change, so a handoff and its acknowledgement cannot be stored separately.
func appendSupportAutoReply(tx *gorm.DB, ticket *model.SupportTicket, content string) error {
	if ticket.Mode != model.SupportModeBot {
		return nil
	}
	answer := supportAutoAnswer(content)
	if requestsHumanSupport(content) {
		now := time.Now()
		ticket.Mode, ticket.HumanRequestedAt = model.SupportModeWaiting, &now
		answer = "已转入人工客服队列，客服会查看本次聊天记录后回复。你可以继续补充问题，人工回复后会收到站内通知。"
	}
	return tx.Create(&model.SupportMessage{TicketID: ticket.ID, Bot: true, Content: answer}).Error
}
