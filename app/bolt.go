package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	bbolt "go.etcd.io/bbolt"
	"log"
	"time"
)

const (
	transactionsBucketName = "transactions"
	costsBucketName        = "costs"
	balanceBucketName      = "balance"
	currentTransactionName = "current_transaction"

	defaultWalkDogCost     = 10
	defaultFreeDish        = 5
	defaultDirtyDish       = 7
	defaultGoToShop        = 5
	defaultWashFloorInFlat = 30
)

type BoltDB struct {
	db *bbolt.DB
}

func NewBoltDB(fileName string) (*BoltDB, error) {
	log.Printf("[INFO] creating bolt store")
	db, err := bbolt.Open(fileName, 0o600, nil)
	buckets := []string{transactionsBucketName, costsBucketName}

	err = db.Update(func(tx *bbolt.Tx) error {
		for _, bktName := range buckets {
			bkt, e := tx.CreateBucketIfNotExists([]byte(bktName))
			if e != nil {
				return fmt.Errorf("failed to create top level bucket %s: %w", bktName, e)
			}

			if bktName == costsBucketName {
				stats := bkt.Stats()
				if stats.KeyN == 0 {
					bkt.Put([]byte(OpWalkDog), itob(defaultWalkDogCost))
					bkt.Put([]byte(OpDirtyDish), itob(defaultDirtyDish))
					bkt.Put([]byte(OpFreeDish), itob(defaultFreeDish))
					bkt.Put([]byte(OpGoToShop), itob(defaultGoToShop))
					bkt.Put([]byte(OpWashFloorInFlat), itob(defaultWashFloorInFlat))
				}
			}
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to create top level bucket): %w", err)
	}

	return &BoltDB{db: db}, nil
}

func (b *BoltDB) CreateTransaction(op string, userId int64) (transactionId string, err error) {
	cost, err := b.GetOperationCost(op)
	if err != nil {
		return "", err
	}

	t := Transaction{
		Timestamp: time.Now(),
		Operation: op,
		Cost:      cost,
		UserId:    userId,
	}

	err = b.db.Update(func(tx *bbolt.Tx) error {
		// Retrieve the users bucket.
		// This should be created when the DB is first opened.
		b := tx.Bucket([]byte(transactionsBucketName))

		// Generate ID for the user.
		// This returns an error only if the Tx is closed or not writeable.
		// That can't happen in an Update() call so I ignore the error check.
		id, _ := b.NextSequence()
		t.ID = string(id)

		// Marshal user data into bytes.
		buf, err := json.Marshal(t)
		if err != nil {
			return err
		}

		// Persist bytes to users bucket.
		return b.Put([]byte(t.ID), buf)
	})

	return t.ID, err
}

func (b *BoltDB) StoreCurrentTransactionId(transactionId string, userId int64) error {
	err := b.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(currentTransactionName))
		return b.Put(itob64(userId), []byte(transactionId))
	})

	return err
}

func (b *BoltDB) GetOperationCost(op string) (int, error) {
	var cost int
	err := b.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(transactionsBucketName))
		v := b.Get([]byte(op))
		cost = int(binary.BigEndian.Uint64(v))
		return nil
	})

	return cost, err
}

func (b *BoltDB) Balance(id int64) (int, error) {
	result := 0
	err := b.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(balanceBucketName))

		v := b.Get(itob64(id))
		result = int(binary.BigEndian.Uint64(v))
		return nil
	})

	return result, err
}

// itob returns an 8-byte big endian representation of v.
func itob(v int) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, uint64(v))
	return b
}

func itob64(v int64) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, uint64(v))
	return b
}
