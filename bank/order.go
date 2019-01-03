package bank

import (
	"github.com/fox-one/foxone-api/db_helper"
	"github.com/jinzhu/gorm"
)

const (
	// OrderStatusNew 30分钟内新订单，引擎将锁仓
	OrderStatusNew = iota
	// OrderStatusPending 30分钟以上，2小时以下订单，引擎将不锁仓
	OrderStatusPending
	// OrderStatusFailed 2小时以上没收到转账
	OrderStatusFailed
	// OrderStatusCanceled 已取消
	OrderStatusCanceled
	// OrderStatusReceived 已收到转账，将进行快捷转账
	OrderStatusReceived
	// OrderStatusDone 已完成
	OrderStatusDone
)

// Order transfer order
type Order struct {
	db_helper.Model

	BrokerID string `gorm:"type:varchar(36);NOT NULL;" json:"broker_id"`
	UserID   string `gorm:"type:varchar(36);NOT NULL;" json:"user_id"`
	AssetID  string `gorm:"type:varchar(36);NOT NULL;" json:"asset_id"`
	Amount   string `gorm:"type:varchar(128);NOT NULL;" json:"amount"`
	Status   int    `gorm:"INDEX;" json:"status"`
}

func listPendingOrders(dbs db_helper.Databases) ([]*Order, error) {
	orders := []*Order{}
	err := dbs.DBRead.Where("status < ?", OrderStatusFailed).Find(&orders).Error
	return orders, err
}

func updateOrders(db *gorm.DB, status int, orders ...*Order) error {
	if len(orders) == 0 {
		return nil
	}

	orderIds := make([]uint, 0, len(orders))
	for _, order := range orders {
		orderIds = append(orderIds, order.ID)
	}
	return db.Model(Order{}).Where("id IN (?)", orderIds).Update("status", status).Error
}
