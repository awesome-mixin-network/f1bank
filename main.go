package main

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"os"
	"time"

	"github.com/fox-one/f1bank/bank"
	"github.com/fox-one/f1bank/config"
	"github.com/fox-one/fox-wallet/models"
	"github.com/fox-one/fox-wallet/store/mysql"
	"github.com/fox-one/foxone-api/aws_utils"
	"github.com/fox-one/foxone-api/db_helper"
	"github.com/fox-one/foxone-api/redis_helper"
	"github.com/fox-one/mixin-sdk/mixin"
	"github.com/fox-one/ocean.one/persistence"
	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

func main() {
	dapp := &mixin.User{
		UserID:    config.ClientID,
		SessionID: config.SessionID,
		PINToken:  config.PINToken,
	}

	block, _ := pem.Decode([]byte(config.SessionKey))
	privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		panic(err)
	}
	dapp.SetPrivateKey(privateKey)

	aws_utils.SetupAwsSession(config.AWS)
	redis_helper.SetupRedisPool(config.Redis)
	dbs := db_helper.SetupDB(config.Database, false, nil)

	ctx := context.Background()

	app := cli.NewApp()
	app.Name = "F1Bank"
	app.Version = "1.0.0"
	app.Usage = "Wish U Good luck"

	app.Flags = []cli.Flag{
		cli.BoolFlag{Name: "debug, d", Usage: "enable debug log"},
		cli.BoolFlag{Name: "setdb, s", Usage: "set up database"},
	}

	app.Before = func(c *cli.Context) error {
		if c.GlobalBool("debug") {
			log.SetLevel(log.DebugLevel)
		}

		if log.GetLevel() != log.DebugLevel {
			gin.SetMode(gin.ReleaseMode)
		}

		if c.GlobalBool("setdb") {
			dbs.DBWrite.AutoMigrate(persistence.Property{}, models.User{}, models.UserAddress{}, bank.Order{})
		}

		return nil
	}

	app.Commands = append(app.Commands, cli.Command{
		Name:        "launch-engine",
		Aliases:     []string{"e"},
		Description: "launch engine",
		Action: func(c *cli.Context) error {
			engine, err := bank.NewEngine(dapp, dbs)
			if err != nil {
				log.Panicln(err)
			}
			engine.Run(ctx)
			return nil
		},
	})

	app.Commands = append(app.Commands, cli.Command{
		Name:        "create-brokers",
		Aliases:     []string{"cb"},
		Description: "create brokers",
		Flags: []cli.Flag{
			cli.IntFlag{Name: "count, c", Value: 100, Usage: "new broker count"},
		},
		Action: func(c *cli.Context) error {
			for idx := 0; idx < c.Int("count"); idx++ {
				if err := createBroker(ctx, dbs, dapp); err != nil {
					return err
				}
			}

			return nil
		},
	})

	app.ExitErrHandler = func(c *cli.Context, err error) {
		if err != nil {
			log.Println(err)
		}
	}

	app.Run(os.Args)
}

func createBroker(ctx context.Context, dbs db_helper.Databases, dapp *mixin.User) error {
	user, err := models.NewUser(ctx, dapp)
	if err != nil {
		return err
	}

	w, _ := user.Wallet()
	if err := w.ModifyPIN(ctx, "", config.PIN); err != nil {
		return err
	}

	store := mysql.NewStore()
	for {
		tx := dbs.DBWrite.Begin()
		if tx.Error != nil {
			time.Sleep(time.Millisecond * 100)
			continue
		}

		if err := store.SaveUsers(tx, user); err != nil {
			tx.Rollback()
			time.Sleep(time.Millisecond * 100)
			continue
		}

		addresses := loadAssets(ctx, w)
		for {
			if err := store.SaveUserAddresses(tx, addresses...); err != nil {
				tx.Rollback()
				time.Sleep(time.Millisecond * 100)
				continue
			}
			break
		}
		if err := tx.Commit().Error; err != nil {
			tx.Rollback()
		}
		break
	}

	return nil
}

func loadAssets(ctx context.Context, user *mixin.User) []*models.UserAddress {
	assets := []string{
		"c6d0c728-2624-429b-8e0d-d9d19b6592fa", // BTC
		"43d61dcd-e413-450d-80b8-101d5e903357", // ETH
		"23dfb5a5-5d7b-48b6-905f-3970e3176e27", // XRP
		"fd11b6e3-0b87-41f1-a41f-f0e9b49e5bf0", // BCH
		"76c802a2-7c88-447f-a93e-c29c9e5dd9c8", // LTC
		"6cfe566e-4aad-470b-8c9a-2fd35b49c68d", // EOS
		"6472e7e3-75fd-48b6-b1dc-28d294ee1476", // DASH
		"27921032-f73e-434e-955f-43d55672ee31", // NEM
		"2204c1ee-0ea2-4add-bb9a-b3719cfff93a", // ETC
		"c996abc9-d94e-4494-b1cf-2a3fd3ac5714", // ZEC
		"990c4c29-57e9-48f6-9819-7d986ea44985", // SC
		"a2c5d22b-62a2-4c13-b3f0-013290dbac60", // ZEN
		"6770a1e5-6086-44d5-b60f-545f9d9e8ffd", // DOGE
	}

	addresses := make([]*models.UserAddress, len(assets))
	for idx, id := range assets {
		for {
			asset, err := user.ReadAsset(ctx, id)
			log.Println(asset, addresses)
			if err != nil {
				time.Sleep(time.Second)
				continue
			}

			addresses[idx] = (&models.UserAddress{UserID: user.UserID}).FromAsset(asset)
			break
		}
	}

	return addresses
}
