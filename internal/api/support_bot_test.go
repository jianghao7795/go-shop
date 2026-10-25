package api

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"shop/internal/model"
)

func decodeSupportTicket(t *testing.T, w *httptest.ResponseRecorder) model.SupportTicket {
	t.Helper()
	requireStatus(t, w, 200)
	var ticket model.SupportTicket
	if err := json.Unmarshal(w.Body.Bytes(), &ticket); err != nil {
		t.Fatal(err)
	}
	return ticket
}

func supportMessages(t *testing.T, db *gorm.DB, id uint) []model.SupportMessage {
	t.Helper()
	var messages []model.SupportMessage
	if err := db.Where("ticket_id = ?", id).Order("id ASC").Find(&messages).Error; err != nil {
		t.Fatal(err)
	}
	return messages
}

func TestSupportAnswersAutomaticallyBeforeHandoff(t *testing.T) {
	db := newTestDB(t)
	hub := newNotificationHub()
	ticket := decodeSupportTicket(t, lifecycleRequest(t, createSupport(db), 0, `{"content":"如何申请退款？","mode":"human"}`))
	if ticket.Mode != model.SupportModeBot || ticket.HumanRequestedAt != nil || ticket.Subject != "如何申请退款？" {
		t.Fatalf("new conversation must be automated: %+v", ticket)
	}
	messages := supportMessages(t, db, ticket.ID)
	if len(messages) != 2 || messages[0].Bot || messages[0].Staff || !messages[1].Bot || messages[1].Staff || !strings.Contains(messages[1].Content, "申请售后") {
		t.Fatalf("missing automatic reply: %+v", messages)
	}
	requireStatus(t, lifecycleRequest(t, replySupport(db, hub, false), ticket.ID, `{"content":"如何修改收货地址？"}`), 200)
	messages = supportMessages(t, db, ticket.ID)
	if len(messages) != 4 || !messages[3].Bot || !strings.Contains(messages[3].Content, "不会改变已提交订单") {
		t.Fatalf("follow-up not answered: %+v", messages)
	}
	// Server rejects staff replies and closes until the buyer explicitly requests a handoff.
	requireStatus(t, lifecycleRequest(t, replySupport(db, hub, true), ticket.ID, `{"content":"人工强行回复"}`), 400)
	requireStatus(t, lifecycleRequest(t, replySupport(db, hub, true), ticket.ID, `{"close":true}`), 400)
	if len(supportMessages(t, db, ticket.ID)) != 4 {
		t.Fatal("staff modified bot conversation")
	}
	var count int64
	db.Model(&model.Notification{}).Where("type = ?", model.NotificationTypeSupport).Count(&count)
	if count != 0 {
		t.Fatal("automatic replies should not generate human reply notifications")
	}
}

func TestSupportOnlyExplicitCommandTransfers(t *testing.T) {
	db := newTestDB(t)
	hub := newNotificationHub()
	ticket := decodeSupportTicket(t, lifecycleRequest(t, createSupport(db), 0, `{"subject":"联系人工","content":"如何查看订单？"}`))
	for _, question := range []string{"怎样联系人工？", "不要联系人工", "我不需要联系人工", "不知道问什么", "转人工"} {
		body, _ := json.Marshal(map[string]string{"content": question, "mode": "waiting"})
		requireStatus(t, lifecycleRequest(t, replySupport(db, hub, false), ticket.ID, string(body)), 200)
		if err := db.First(&ticket, ticket.ID).Error; err != nil {
			t.Fatal(err)
		}
		if ticket.Mode != model.SupportModeBot || ticket.HumanRequestedAt != nil {
			t.Fatalf("unexpected handoff for %q", question)
		}
		messages := supportMessages(t, db, ticket.ID)
		if !messages[len(messages)-1].Bot {
			t.Fatalf("question not automatically answered: %q", question)
		}
	}
	requireStatus(t, lifecycleRequest(t, replySupport(db, hub, false), ticket.ID, `{"content":"  联系人工！  "}`), 200)
	db.First(&ticket, ticket.ID)
	if ticket.Mode != model.SupportModeWaiting || ticket.HumanRequestedAt == nil {
		t.Fatalf("handoff failed: %+v", ticket)
	}
	requestedAt := *ticket.HumanRequestedAt
	before := supportMessages(t, db, ticket.ID)
	if !before[len(before)-1].Bot || !strings.Contains(before[len(before)-1].Content, "人工客服队列") {
		t.Fatal("missing handoff acknowledgement")
	}
	for _, content := range []string{"我想再补充一个订单问题", "联系人工"} {
		body, _ := json.Marshal(map[string]string{"content": content})
		requireStatus(t, lifecycleRequest(t, replySupport(db, hub, false), ticket.ID, string(body)), 200)
	}
	messages := supportMessages(t, db, ticket.ID)
	if len(messages) != len(before)+2 || messages[len(messages)-1].Bot {
		t.Fatal("automatic replies continued after handoff")
	}
	db.First(&ticket, ticket.ID)
	if ticket.HumanRequestedAt == nil || !ticket.HumanRequestedAt.Equal(requestedAt) {
		t.Fatal("duplicate handoff reset the queue timestamp")
	}
	requireStatus(t, lifecycleRequest(t, replySupport(db, hub, true), ticket.ID, `{"content":"你好，请提供订单号"}`), 200)
	db.First(&ticket, ticket.ID)
	if ticket.Mode != model.SupportModeHuman {
		t.Fatal("staff reply did not activate human mode")
	}
	messages = supportMessages(t, db, ticket.ID)
	if !messages[len(messages)-1].Staff || messages[len(messages)-1].Bot {
		t.Fatal("human reply marked as bot")
	}
	requireStatus(t, lifecycleRequest(t, replySupport(db, hub, false), ticket.ID, `{"content":"订单问题已解决"}`), 200)
	if len(supportMessages(t, db, ticket.ID)) != len(messages)+1 {
		t.Fatal("bot resumed after staff response")
	}
}

func TestSupportHandoffOnFirstMessageAndAdminQueue(t *testing.T) {
	db := newTestDB(t)
	bot := decodeSupportTicket(t, lifecycleRequest(t, createSupport(db), 0, `{"content":"你好"}`))
	waiting := decodeSupportTicket(t, lifecycleRequest(t, createSupport(db), 0, `{"content":"联系人工"}`))
	if waiting.Mode != model.SupportModeWaiting || waiting.HumanRequestedAt == nil {
		t.Fatal("first message handoff was ignored")
	}
	if len(supportMessages(t, db, waiting.ID)) != 2 {
		t.Fatal("initial handoff history missing")
	}
	w := lifecycleRequest(t, listSupport(db, true), 0, `{}`)
	var result struct {
		Items []model.SupportTicket `json:"items"`
		Total int64                 `json:"total"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Total != 1 || len(result.Items) != 1 || result.Items[0].ID != waiting.ID {
		t.Fatalf("bot conversation leaked into human queue: %s", w.Body.String())
	}
	c, w := createOrderCtx(t, `{}`)
	c.Request.URL.RawQuery = "mode=bot"
	listSupport(db, true)(c)
	requireStatus(t, w, 200)
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Total != 1 || result.Items[0].ID != bot.ID {
		t.Fatal("bot filter failed")
	}
	// Buyers still see both automated and human conversations in their own history.
	w = lifecycleRequest(t, listSupport(db, false), 0, `{}`)
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Total != 2 {
		t.Fatal("buyer lost conversation history")
	}
}

func TestSupportHandoffRollsBackWhenAcknowledgementFails(t *testing.T) {
	db := newTestDB(t)
	ticket := decodeSupportTicket(t, lifecycleRequest(t, createSupport(db), 0, `{"content":"你好"}`))
	if err := db.Exec(`CREATE TRIGGER fail_bot_reply BEFORE INSERT ON shop_support_messages WHEN NEW.bot = 1 BEGIN SELECT RAISE(ABORT, 'test bot insert failure'); END;`).Error; err != nil {
		t.Fatal(err)
	}
	w := lifecycleRequest(t, replySupport(db, newNotificationHub(), false), ticket.ID, `{"content":"联系人工"}`)
	if w.Code == 200 {
		t.Fatal("failed acknowledgement reported as success")
	}
	db.First(&ticket, ticket.ID)
	if ticket.Mode != model.SupportModeBot || ticket.HumanRequestedAt != nil || len(supportMessages(t, db, ticket.ID)) != 2 {
		t.Fatal("partial handoff committed")
	}
	w = lifecycleRequest(t, createSupport(db), 0, `{"content":"如何查看物流？"}`)
	if w.Code == 200 {
		t.Fatal("failed bot response reported as created")
	}
	var count int64
	db.Model(&model.SupportTicket{}).Count(&count)
	if count != 1 {
		t.Fatal("incomplete initial conversation was committed")
	}
}

func TestSupportMessageLimitReservesAutomaticResponse(t *testing.T) {
	db := newTestDB(t)
	ticket := model.SupportTicket{Subject: "many messages", Status: "open", Mode: model.SupportModeBot}
	db.Create(&ticket)
	messages := make([]model.SupportMessage, 199)
	for i := range messages {
		messages[i] = model.SupportMessage{TicketID: ticket.ID, Content: "question"}
	}
	if err := db.Create(&messages).Error; err != nil {
		t.Fatal(err)
	}
	requireStatus(t, lifecycleRequest(t, replySupport(db, newNotificationHub(), false), ticket.ID, `{"content":"如何退款？"}`), 400)
	if len(supportMessages(t, db, ticket.ID)) != 199 {
		t.Fatal("stored question without space for an answer")
	}
	requireStatus(t, lifecycleRequest(t, replySupport(db, newNotificationHub(), false), ticket.ID, `{"close":true}`), 200)
	requireStatus(t, lifecycleRequest(t, replySupport(db, newNotificationHub(), false), ticket.ID, `{"content":"联系人工"}`), 400)
}

func TestSupportAutoAnswerCoversShoppingQuestions(t *testing.T) {
	for _, tt := range []struct{ question, expected string }{
		{"我的订单在哪里？", "我的订单"},
		{"怎么付款？", "支付宝"},
		{"什么时候发货？", "需要商家确认"},
		{"怎么退货？", "申请售后"},
		{"订单地址填错了", "不会改变已提交订单"},
		{"优惠券怎么用？", "有效期"},
		{"颜色规格没有库存", "售罄"},
		{"如何取消订单？", "取消订单"},
		{"购物车怎么结算？", "确认收货地址"},
		{"如何评价？", "待评价"},
		{"没有匹配的其他问题", "暂时没有找到"},
	} {
		t.Run(tt.question, func(t *testing.T) {
			answer := supportAutoAnswer(tt.question)
			if !strings.Contains(answer, tt.expected) || !strings.Contains(answer, "联系人工") {
				t.Fatalf("unexpected answer: %s", answer)
			}
		})
	}
}

func TestSupportUpgradePreservesExistingHumanConversations(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	if err := db.Exec(`CREATE TABLE shop_support_tickets (id integer PRIMARY KEY, user_id text, subject text, status text, created_at datetime, updated_at datetime)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE shop_support_messages (id integer PRIMARY KEY, ticket_id integer, staff numeric, content text, created_at datetime)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO shop_support_tickets (id, user_id, subject, status, created_at, updated_at) VALUES (1, 'buyer', 'existing', 'open', ?, ?)`, time.Now(), time.Now()).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO shop_support_messages (id, ticket_id, staff, content) VALUES (1, 1, 1, 'human reply')`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.SupportTicket{}, &model.SupportMessage{}); err != nil {
		t.Fatal(err)
	}
	var ticket model.SupportTicket
	var message model.SupportMessage
	db.First(&ticket, 1)
	db.First(&message, 1)
	if ticket.Mode != model.SupportModeHuman || !message.Staff || message.Bot {
		t.Fatalf("legacy conversation changed: %+v %+v", ticket, message)
	}
}
