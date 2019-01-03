// +build template

package config

import (
	"github.com/fox-one/foxone-api/aws_utils"
	"github.com/fox-one/foxone-api/db_helper"
	"github.com/fox-one/foxone-api/redis_helper"
)

const (
	// ClientID mixin user id
	ClientID = "xxx"
	// PIN pin
	PIN = "xxx"
	// SessionID session id
	SessionID = "xxx"
	// PINToken pin token
	PINToken = "xxx"
	// SessionKey private key in pem
	SessionKey = `-----BEGIN RSA PRIVATE KEY-----
xxx
-----END RSA PRIVATE KEY-----`

	AESKEY = "test"
)

// aws
var AWS = aws_utils.AWSConfig{
	AccessKey: "xx",
	Secret:    "xx",
	Region:    "ap-northeast-1",
}

// db
var Database = db_helper.DBConfig{
	HostWrite:    "localhost:3306",
	HostRead:     "localhost:3306",
	DatabaseName: "f1bank",
	Username:     "root",
	Password:     "",

	ReadLogFile:  "./db_read_log.log",
	WriteLogFile: "./db_write_log.log",
}

// redis
var Redis = redis_helper.RedisConfig{
	Host:           "localhost",
	Port:           "6379",
	Password:       "",
	MaxIdleConns:   50,
	MaxActiveConns: 300,
}
