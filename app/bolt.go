package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	bbolt "go.etcd.io/bbolt"
	"log"
	"strconv"
	"strings"
	"time"
)

const (
	transactionsBucketName = "transactions"
	costsBucketName        = "costs"
	balanceBucketName      = "balance"
	usersBucketName        = "users"
	currentTransactionName = "current_transaction"

	defaultWalkDogCost     = 10
	defaultFreeDish        = 5
	defaultDirtyDish       = 7
	defaultGoToShop        = 5
	defaultWashFloorInFlat = 30

	TSNano = "2006-01-02T15:04:05.000000000Z07:00"
)

type BoltDB struct {
	db *bbolt.DB
}

func NewBoltDB(fileName string) (*BoltDB, error) {
	log.Printf("[INFO] creating bolt store")
	db, err := bbolt.Open(fileName, 0o600, nil)
	buckets := []string{transactionsBucketName, costsBucketName, balanceBucketName, usersBucketName}

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
		Status:    OpenStatus,
	}

	err = b.db.Update(func(tx *bbolt.Tx) error {
		// create bucket for a user if not exists
		_, e := tx.CreateBucketIfNotExists(itob64(userId))
		if e != nil {
			return fmt.Errorf("failed to create user bucket %s: %w", userId, e)
		}

		return nil
	})

	transactionTimeStamp := []byte(t.Timestamp.Format(TSNano))

	err = b.db.Update(func(tx *bbolt.Tx) error {
		// Retrieve the users bucket.
		// This should be created when the DB is first opened.
		userBkt := tx.Bucket(itob64(userId))

		id, _ := userBkt.NextSequence()
		t.ID = string(id)

		// Marshal user data into bytes.
		buf, err := json.Marshal(t)
		if err != nil {
			return err
		}

		// Persist bytes to users bucket.
		return userBkt.Put(transactionTimeStamp, buf)
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
		b := tx.Bucket([]byte(costsBucketName))
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

func (b *BoltDB) ShowLastNTransactions(id int64, limit int) (transactions []Transaction, err error) {
	transactions = []Transaction{}

	err = b.db.View(func(tx *bbolt.Tx) error {
		// Assume bucket exists and has keys
		userBkt := tx.Bucket(itob64(id))

		bktStats := userBkt.Stats()

		log.Println("[DEBUG] number of keys ", bktStats.KeyN)
		c := userBkt.Cursor()

		for k, v := c.Last(); k != nil; k, v = c.Prev() {
			transaction := Transaction{}
			if err := json.Unmarshal(v, &transaction); err != nil {
				return fmt.Errorf("failed to unmarshal: %w", err)
			}

			transactions = append(transactions, transaction)
			if len(transactions) >= limit {
				break
			}
		}

		return nil
	})

	return transactions, err
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

func (b *BoltDB) load(bkt *bbolt.Bucket, key string, res interface{}) error {
	value := bkt.Get([]byte(key))
	if value == nil {
		return fmt.Errorf("no value for %s", key)
	}

	if err := json.Unmarshal(value, &res); err != nil {
		return fmt.Errorf("failed to unmarshal: %w", err)
	}
	return nil
}

func (b *BoltDB) HasParent(childNickName string) (bool, error) {
	result := false
	err := b.db.View(func(tx *bbolt.Tx) error {
		bktName := "child_@" + childNickName
		bkt := tx.Bucket([]byte(bktName))

		if bkt != nil {
			result = true
		}

		return nil
	})

	return result, err
}

func (b *BoltDB) RegisterUser(user User) error {
	log.Printf("[INFO] user %s registration", user.Nickname)
	err := b.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(usersBucketName))

		user.RegistrationTS = time.Now()
		buf, err := json.Marshal(user)
		if err != nil {
			return err
		}

		return b.Put(itob64(user.ID), buf)
	})

	return err
}

func (b *BoltDB) CheckRegistered(id int64) (bool, error) {
	_, err := b.FindUser(id)
	if err != nil && strings.Contains(err.Error(), "not found") {
		return false, nil
	}

	if err != nil {
		return false, err
	}

	return true, nil
}

func (b *BoltDB) FindUser(id int64) (User, error) {
	var user User

	err := b.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(usersBucketName))

		v := b.Get(itob64(id))
		if v != nil {
			if err := json.Unmarshal(v, &user); err != nil {
				return fmt.Errorf("failed to unmarshal: %w", err)
			}
		} else {
			return fmt.Errorf("user not found")
		}

		return nil
	})

	return user, err
}

func (b *BoltDB) CreateParentBucket(parentId int64) error {
	err := b.db.Update(func(tx *bbolt.Tx) error {
		// create bucket for a user if not exists
		bktName := "parent_" + strconv.FormatInt(parentId, 10)
		_, e := tx.CreateBucketIfNotExists([]byte(bktName))
		if e != nil {
			return fmt.Errorf("failed to create parent bucket %s: %w", parentId, e)
		}

		return nil
	})

	return err
}

func (b *BoltDB) BindChildToParent(parentId int64, childNickName string) error {
	err := b.db.Update(func(tx *bbolt.Tx) error {
		// create bucket for a user if not exists
		bktName := "child_" + childNickName
		bkt, e := tx.CreateBucketIfNotExists([]byte(bktName))
		if e != nil {
			return fmt.Errorf("failed to create child bucket %s: %w", parentId, e)
		}

		e = bkt.Put(itob64(parentId), nil)
		if e != nil {
			return fmt.Errorf("failed to add parent %d to child bucket %s: %w", parentId, bktName, e)
		}

		return nil
	})

	return err
}
