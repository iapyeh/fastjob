/*
 API: https://godoc.org/github.com/syndtr/goleveldb/leveldb
*/
package model

import (
)

type Dict interface {
	Get(key []byte) ([]byte, error)
	GetString(key string) ([]byte, error)
	Set(key []byte, value []byte) error
	SetString(key string, value []byte) error
	Del(key []byte) error
	DelString(key string) error
}
