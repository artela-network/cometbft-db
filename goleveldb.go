package db

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/errors"
	"github.com/syndtr/goleveldb/leveldb/opt"
	"github.com/syndtr/goleveldb/leveldb/util"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

func init() {
	dbCreator := func(name string, dir string) (DB, error) {
		return NewGoLevelDB(name, dir)
	}
	registerDBCreator(GoLevelDBBackend, dbCreator, false)

	createPrometheusMetrics()
}

var (
	getDurationNs        prometheus.Gauge
	setDurationNs        prometheus.Gauge
	setSyncDurationNs    prometheus.Gauge
	deleteDurationNs     prometheus.Gauge
	deleteSyncDurationNs prometheus.Gauge
)

func createPrometheusMetrics() {
	getDurationNs = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "cometbft",
		Subsystem: "db",
		Name:      "get_duration_ns",
		Help:      "The duration of the Get() operation in nanoseconds.",
	})
	prometheus.MustRegister(getDurationNs)
	setDurationNs = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "cometbft",
		Subsystem: "db",
		Name:      "set_duration_ns",
		Help:      "The duration of the Set() operation in nanoseconds.",
	})
	prometheus.MustRegister(setDurationNs)
	setSyncDurationNs = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "cometbft",
		Subsystem: "db",
		Name:      "set_sync_duration_ns",
		Help:      "The duration of the SetSync() operation in nanoseconds.",
	})
	prometheus.MustRegister(setSyncDurationNs)
	deleteDurationNs = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "cometbft",
		Subsystem: "db",
		Name:      "delete_duration_ns",
		Help:      "The duration of the Delete() operation in nanoseconds.",
	})
	prometheus.MustRegister(deleteDurationNs)
	deleteSyncDurationNs = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "cometbft",
		Subsystem: "db",
		Name:      "delete_sync_duration_ns",
		Help:      "The duration of the DeleteSync() operation in nanoseconds.",
	})
	prometheus.MustRegister(deleteSyncDurationNs)
}

type GoLevelDB struct {
	db *leveldb.DB
}

var _ DB = (*GoLevelDB)(nil)

func NewGoLevelDB(name string, dir string) (*GoLevelDB, error) {
	return NewGoLevelDBWithOpts(name, dir, nil)
}

func NewGoLevelDBWithOpts(name string, dir string, o *opt.Options) (*GoLevelDB, error) {
	dbPath := filepath.Join(dir, name+".db")
	db, err := leveldb.OpenFile(dbPath, o)
	if err != nil {
		return nil, err
	}

	// Create a new levelDBCollector
	collector := newLevelDBCollector(db)
	// Register the collector with Prometheus
	prometheus.MustRegister(collector)

	database := &GoLevelDB{
		db: db,
	}
	return database, nil
}

// Get implements DB.
func (db *GoLevelDB) Get(key []byte) ([]byte, error) {
	if len(key) == 0 {
		return nil, errKeyEmpty
	}
	start := time.Now()
	res, err := db.db.Get(key, nil)
	getDurationNs.Set(float64(time.Since(start).Nanoseconds()))
	if err != nil {
		if err == errors.ErrNotFound {
			return nil, nil
		}
		return nil, err
	}
	return res, nil
}

// Has implements DB.
func (db *GoLevelDB) Has(key []byte) (bool, error) {
	bytes, err := db.Get(key)
	if err != nil {
		return false, err
	}
	return bytes != nil, nil
}

// Set implements DB.
func (db *GoLevelDB) Set(key []byte, value []byte) error {
	if len(key) == 0 {
		return errKeyEmpty
	}
	if value == nil {
		return errValueNil
	}
	start := time.Now()
	err := db.db.Put(key, value, nil)
	setDurationNs.Set(float64(time.Since(start).Nanoseconds()))
	if err != nil {
		return err
	}
	return nil
}

// SetSync implements DB.
func (db *GoLevelDB) SetSync(key []byte, value []byte) error {
	if len(key) == 0 {
		return errKeyEmpty
	}
	if value == nil {
		return errValueNil
	}
	start := time.Now()
	err := db.db.Put(key, value, &opt.WriteOptions{Sync: true})
	setSyncDurationNs.Set(float64(time.Since(start).Nanoseconds()))
	if err != nil {
		return err
	}
	return nil
}

// Delete implements DB.
func (db *GoLevelDB) Delete(key []byte) error {
	if len(key) == 0 {
		return errKeyEmpty
	}
	start := time.Now()
	err := db.db.Delete(key, nil)
	deleteDurationNs.Set(float64(time.Since(start).Nanoseconds()))
	if err != nil {
		return err
	}
	return nil
}

// DeleteSync implements DB.
func (db *GoLevelDB) DeleteSync(key []byte) error {
	if len(key) == 0 {
		return errKeyEmpty
	}
	start := time.Now()
	err := db.db.Delete(key, &opt.WriteOptions{Sync: true})
	deleteSyncDurationNs.Set(float64(time.Since(start).Nanoseconds()))
	if err != nil {
		return err
	}
	return nil
}

func (db *GoLevelDB) DB() *leveldb.DB {
	return db.db
}

// Close implements DB.
func (db *GoLevelDB) Close() error {
	if err := db.db.Close(); err != nil {
		return err
	}
	return nil
}

// Print implements DB.
func (db *GoLevelDB) Print() error {
	str, err := db.db.GetProperty("leveldb.stats")
	if err != nil {
		return err
	}
	fmt.Printf("%v\n", str)

	itr := db.db.NewIterator(nil, nil)
	for itr.Next() {
		key := itr.Key()
		value := itr.Value()
		fmt.Printf("[%X]:\t[%X]\n", key, value)
	}
	return nil
}

// Stats implements DB.
func (db *GoLevelDB) Stats() map[string]string {
	keys := []string{
		"leveldb.num-files-at-level{n}",
		"leveldb.stats",
		"leveldb.sstables",
		"leveldb.blockpool",
		"leveldb.cachedblock",
		"leveldb.openedtables",
		"leveldb.alivesnaps",
		"leveldb.aliveiters",
	}

	stats := make(map[string]string)
	for _, key := range keys {
		str, err := db.db.GetProperty(key)
		if err == nil {
			stats[key] = str
		}
	}
	return stats
}

// NewBatch implements DB.
func (db *GoLevelDB) NewBatch() Batch {
	return newGoLevelDBBatch(db)
}

// Iterator implements DB.
func (db *GoLevelDB) Iterator(start, end []byte) (Iterator, error) {
	if (start != nil && len(start) == 0) || (end != nil && len(end) == 0) {
		return nil, errKeyEmpty
	}
	itr := db.db.NewIterator(&util.Range{Start: start, Limit: end}, nil)
	return newGoLevelDBIterator(itr, start, end, false), nil
}

// ReverseIterator implements DB.
func (db *GoLevelDB) ReverseIterator(start, end []byte) (Iterator, error) {
	if (start != nil && len(start) == 0) || (end != nil && len(end) == 0) {
		return nil, errKeyEmpty
	}
	itr := db.db.NewIterator(&util.Range{Start: start, Limit: end}, nil)
	return newGoLevelDBIterator(itr, start, end, true), nil
}
