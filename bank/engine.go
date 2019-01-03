package bank

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/fox-one/mixin-sdk/messenger"

	"github.com/Workiva/go-datastructures/queue"
	"github.com/fox-one/f1bank/config"
	"github.com/fox-one/fox-wallet/models"
	"github.com/fox-one/fox-wallet/store"
	"github.com/fox-one/foxone-api/db_helper"
	"github.com/fox-one/foxone-api/mysql_store"
	"github.com/fox-one/mixin-sdk/mixin"
	"github.com/fox-one/mixin-sdk/utils"
	"github.com/fox-one/spider-utils/properties"
)

const (
	pullInterval           = time.Second / 2
	retryInterval          = time.Second
	transactionCheckoutKey = "f1bank_engine_transaction_checkout"
	externalCheckoutKey    = "f1bank_engine_external_checkout"
)

// Engine bank engine
type Engine struct {
	dapp      *mixin.User
	messenger *messenger.Messenger
	dbs       db_helper.Databases

	propertyStore properties.PropertyStore
	store         *store.Store

	externalCheckpoint time.Time
	snapshotCheckpoint time.Time

	snapshots         map[string]bool
	externalSnapshots map[string]bool

	brokers      map[string]*models.User
	availBrokers *queue.Queue
	orders       map[uint]*Order
	brokerOrders map[string]uint
}

func parseCheckpoint(ctx context.Context, store properties.PropertyStore, key string) (time.Time, error) {
	value, err := store.ReadProperty(ctx, key)
	log.Println(value, err)
	if err != nil {
		return time.Time{}, err
	}

	if len(value) == 0 {
		return time.Time{}, nil
	}

	return time.Parse(time.RFC3339Nano, value)
}

// NewEngine create engine
func NewEngine(dapp *mixin.User, dbs db_helper.Databases) (*Engine, error) {
	engine := &Engine{
		dapp:      dapp,
		messenger: messenger.NewMessenger(dapp),
		dbs:       dbs,
		store:     store.NewStore(&dbs),

		brokers:           map[string]*models.User{},
		orders:            map[uint]*Order{},
		brokerOrders:      map[string]uint{},
		snapshots:         map[string]bool{},
		externalSnapshots: map[string]bool{},
		propertyStore:     mysql_store.PropertyStore(dbs),

		externalCheckpoint: time.Now(),
		snapshotCheckpoint: time.Now(),
	}

	ctx := context.TODO()
	var err error

	engine.externalCheckpoint, err = parseCheckpoint(ctx, engine.propertyStore, externalCheckoutKey)
	if err != nil {
		return nil, err
	}

	engine.snapshotCheckpoint, err = parseCheckpoint(ctx, engine.propertyStore, transactionCheckoutKey)
	if err != nil {
		return nil, err
	}

	brokers, err := engine.store.QueryUsers("", "", "", time.Time{}, 100)
	if err != nil {
		return nil, err
	}

	orders, err := listPendingOrders(dbs)
	if err != nil {
		return nil, err
	}

	engine.availBrokers = queue.New(int64(len(brokers)))

	for _, order := range orders {
		engine.orders[order.ID] = order
		engine.brokerOrders[order.BrokerID] = order.ID
	}

	for _, broker := range brokers {
		engine.brokers[broker.UserID] = broker
		if _, found := engine.brokerOrders[broker.BrokerID]; !found {
			if err := engine.availBrokers.Put(broker); err != nil {
				return nil, err
			}
		}
	}

	return engine, nil
}

// Run run engine
func (engine *Engine) Run(ctx context.Context) {
	go engine.pollNetwork(ctx)
	go engine.pollExternal(ctx)
	engine.syncOrders(ctx)
}

func (engine *Engine) pollNetwork(ctx context.Context) {
	const limit = 500
	for {
		check := engine.snapshotCheckpoint
		snapshots, err := engine.dapp.ReadNetwork(ctx, "", check, true, 500)
		if err != nil {
			log.Println("PullMixinNetwork ERROR ", err)
			time.Sleep(retryInterval)
			continue
		}

		for _, snapshot := range snapshots {
			if _, ok := engine.snapshots[snapshot.SnapshotID]; ok {
				continue
			}

			if snapshot.UserID == engine.dapp.UserID {
				// TODO update balance info
				continue
			}

			if snapshot.Amount[0] == '-' {
				continue
			}

			broker, found := engine.brokers[snapshot.UserID]
			if !found {
				continue
			}

			w, err := broker.Wallet()
			if err != nil {
				break
			}

			w.Transfer(ctx, &mixin.TransferInput{
				AssetID:    snapshot.AssetID,
				OpponentID: engine.dapp.UserID,
				Amount:     snapshot.Amount,
				TraceID:    utils.UUIDWithString("transfer:" + snapshot.SnapshotID),
				Memo:       "Transfer " + snapshot.SnapshotID,
			}, config.PIN)

			engine.snapshots[snapshot.SnapshotID] = true
			check = snapshot.CreatedAt
		}

		if err == nil && check != engine.snapshotCheckpoint {
			engine.snapshotCheckpoint = check
			if err := engine.propertyStore.WriteProperty(ctx, transactionCheckoutKey, check.Format(time.RFC3339Nano)); err != nil {
				log.Printf("save checkpoint %s error : %s\n", check, err)
			}
		}

		if err != nil {
			time.Sleep(retryInterval)
		} else {
			engine.snapshotCheckpoint = check
			time.Sleep(pullInterval)
		}
	}
}

func (engine *Engine) pollExternal(ctx context.Context) {
	const limit = 500
	for {
		check := engine.externalCheckpoint
		snapshots, err := engine.dapp.ReadExternal(ctx, "", "", "", "", time.Time{}, 100)
		if err != nil {
			log.Println("PullMixinNetwork ERROR ", err)
			time.Sleep(retryInterval)
			continue
		}

		for _, snapshot := range snapshots {
			if _, ok := engine.snapshots[snapshot.TransactionID]; ok {
				continue
			}

			log.Println(snapshot)
			addr, err := engine.store.QueryAddress(snapshot.ChainID, snapshot.PublicKey, snapshot.AccountName, snapshot.AccountTag)
			if err != nil {
				log.Printf("process transaction %s faild: %s\n", snapshot.TransactionID, err)
				break
			}

			orderID, found := engine.brokerOrders[addr.UserID]
			if !found {
				continue
			}

			order, found := engine.orders[orderID]
			if !found {
				continue
			}

			if _, err := engine.dapp.Transfer(ctx, &mixin.TransferInput{
				AssetID:    snapshot.AssetID,
				OpponentID: order.UserID,
				Amount:     snapshot.Amount,
				TraceID:    utils.UUIDWithString(fmt.Sprintf("order:%d", order.ID)),
				Memo:       "Fast Deposit From F1Bank",
			}, config.PIN); err != nil {

				log.Printf("process transaction %s faild: %s\n", snapshot.TransactionID, err)
				break
			}

			engine.externalSnapshots[snapshot.TransactionID] = true
			check = snapshot.CreatedAt
		}

		if check != engine.externalCheckpoint {
			engine.externalCheckpoint = check
			if err := engine.propertyStore.WriteProperty(ctx, externalCheckoutKey, check.Format(time.RFC3339Nano)); err != nil {
				log.Printf("save checkpoint %s error : %s\n", check, err)
			}
		}

		if err != nil {
			time.Sleep(retryInterval)
		} else {
			time.Sleep(pullInterval)
		}
	}
}

func (engine *Engine) syncOrders(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
		}
	}
}
