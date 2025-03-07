package db

import (
	"fmt"
	"strings"
)

type BackendType string

// These are valid backend types.
const (
	// GoLevelDBBackend represents goleveldb (github.com/syndtr/goleveldb - most
	// popular implementation)
	//   - pure go
	//   - stable
	//   - unmaintained
	GoLevelDBBackend BackendType = "goleveldb"
	// MemDBBackend represents in-memory key value store, which is mostly used
	// for testing.
	MemDBBackend BackendType = "memdb"
	// RocksDBBackend represents rocksdb (uses github.com/tecbot/gorocksdb)
	//   - EXPERIMENTAL
	//   - requires gcc
	//   - use rocksdb build tag (go build -tags rocksdb)
	RocksDBBackend BackendType = "rocksdb"
	// BadgerDBBackend represents badger (uses github.com/dgraph-io/badger)
	//   - EXPERIMENTAL
	//   - use badgerdb build tag (go build -tags badgerdb)
	BadgerDBBackend BackendType = "badgerdb"
	// PebbleDBDBBackend represents pebble (uses github.com/cockroachdb/pebble)
	//   - EXPERIMENTAL
	//   - use pebbledb build tag (go build -tags pebbledb)
	PebbleDBBackend BackendType = "pebbledb"
)

type dbCreator func(name string, dir string) (DB, error)

var backends = map[BackendType]dbCreator{}

func registerDBCreator(backend BackendType, creator dbCreator) {
	_, ok := backends[backend]
	if ok {
		return
	}
	backends[backend] = creator
}

// NewDB creates a new database of type backend with the given name.
func NewDB(name string, backend BackendType, dir string) (DB, error) {
	dbCreator, ok := backends[backend]
	if !ok {
		keys := make([]string, 0, len(backends))
		for k := range backends {
			keys = append(keys, string(k))
		}
		return nil, fmt.Errorf("unknown db_backend %s, expected one of %v",
			backend, strings.Join(keys, ","))
	}

	db, err := dbCreator(name, dir)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}
	return db, nil
}
