package bank

import (
	"context"
	"time"

	"github.com/fox-one/mixin-sdk/messenger"
	"github.com/fox-one/mixin-sdk/utils"
	log "github.com/sirupsen/logrus"
)

// OnMessage on message
func (e *Engine) OnMessage(ctx context.Context, msgView messenger.MessageView, userID string) error {
	// TODO create order / cancel order / query balance info
	return nil
}

// RunMessenger run
func (e *Engine) RunMessenger(ctx context.Context) {
	for {
		if err := e.messenger.Loop(ctx, e); err != nil {
			log.Println("something is wrong", err)
			time.Sleep(1 * time.Second)
		}
	}
}

// Send send message
func (e *Engine) Send(ctx context.Context, userID, content string) error {
	msgView := messenger.MessageView{
		ConversationId: utils.UniqueConversationID(e.messenger.UserID, userID),
		UserId:         userID,
	}
	err := e.messenger.SendPlainText(ctx, msgView, content)
	log.Println("Send message", err)
	return err
}
