package store

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
	costsBucketName        = "costs"               // bucket with operation costs, operation -> cost
	balanceBucketName      = "balance"             // userId -> balance
	usersBucketName        = "users"               // userId -> user struct
	currentTransactionName = "current_transaction" // userId -> transaction struct

	defaultWalkDogCost     = 10
	defaultFreeDish        = 5
	defaultDirtyDish       = 7
	defaultGoToShop        = 5
	defaultWashFloorInFlat = 30

	TSNano = "2006-01-02T15:04:05.000000000Z07:00"
)

const (
	OpNewTask           = "new_task"
	OpWalkDog           = "walk_dog"
	OpBalance           = "balance"
	OpHistory           = "history"
	OpGetMoney          = "get_money"
	OpFinishTask        = "finish_task"
	OpFreeDish          = "free_dish"
	OpDirtyDish         = "dirty_dish"
	OpGoToShop          = "go_to_shop"
	OpWashFloorInFlat   = "wash_floor_in_flat"
	OpChild             = "child"
	OpParent            = "parent"
	OpCancelCurrentTask = "cancel_current_task"
	OpFinishCurrentTask = "finish_current_task"
)

// transaction list is stored in separate bucket for each user, bucketname = userId, timestamp -> transaction
// parent_<userId> - bucket with a list of parent children
// "child_@" + childNickName - contains list of parents, parentId->nil
type BoltDB struct {
	db *bbolt.DB
}

// NewBoltDB creates or opens DB, and creates buckets
func NewBoltDB(fileName string) (*BoltDB, error) {
	log.Printf("[INFO] creating bolt store")
	db, err := bbolt.Open(fileName, 0o600, nil)
	buckets := []string{costsBucketName, balanceBucketName, usersBucketName, currentTransactionName}

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

func (b *BoltDB) CreateTransaction(op string, userId int64) (transaction *Transaction, err error) {
	cost, err := b.GetOperationCost(op)
	if err != nil {
		return nil, err
	}

	t := Transaction{
		Timestamp: time.Now(),
		Operation: op,
		Cost:      cost,
		UserId:    userId,
		Status:    OpenStatus,
	}

	err = b.db.Update(func(tx *bbolt.Tx) error {
		// create bucket for a user if not exists, bucket keeps list of user transactions
		_, e := tx.CreateBucketIfNotExists(itob64(userId))
		if e != nil {
			return fmt.Errorf("failed to create user bucket %d: %w", userId, e)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	transactionTimeStamp := []byte(t.Timestamp.Format(TSNano))

	err = b.db.Update(func(tx *bbolt.Tx) error {
		// Retrieve the users bucket.
		// This should be created when the DB is first opened.
		userBkt := tx.Bucket(itob64(userId))

		id, _ := userBkt.NextSequence()
		t.ID = fmt.Sprint(id)

		// Marshal user data into bytes.
		buf, err := json.Marshal(t)
		if err != nil {
			return err
		}

		// Persist bytes to users bucket.
		return userBkt.Put(transactionTimeStamp, buf)
	})

	if err != nil {
		return nil, err
	}

	err = b.db.Update(func(tx *bbolt.Tx) error {
		// Retrieve the users bucket.
		// This should be created when the DB is first opened.
		bkt := tx.Bucket([]byte(currentTransactionName))
		buf, err := json.Marshal(t)
		if err != nil {
			return err
		}

		// Persist bytes to users bucket.
		return bkt.Put(itob64(userId), buf)
	})

	return &t, err
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
		if v != nil {
			result = int(binary.BigEndian.Uint64(v))
		}
		return nil
	})

	return result, err
}

func (b *BoltDB) ChangeBalance(userId int64, delta int) (result int, err error) {
	result = delta
	err = b.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(balanceBucketName))
		v := b.Get(itob64(userId))

		if v == nil {
			b.Put(itob64(userId), itob(delta))
		} else {
			val := int(binary.BigEndian.Uint64(v)) + delta
			result = val
			b.Put(itob64(userId), itob(val))
		}

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

	if user.Type == PARENT {
		err = b.CreateParentBucket(user.ID)
	}

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
			return fmt.Errorf("failed to create parent bucket %d: %w", parentId, e)
		}

		return nil
	})

	return err
}

func (b *BoltDB) BindChildToParent(parentId int64, childNickName string) error {
	// create and put child -> parent relation
	err := b.db.Update(func(tx *bbolt.Tx) error {
		// create bucket for a user if not exists
		bktName := "child_" + childNickName
		bkt, e := tx.CreateBucketIfNotExists([]byte(bktName))
		if e != nil {
			return fmt.Errorf("failed to create child bucket %d: %w", parentId, e)
		}

		e = bkt.Put(itob64(parentId), nil)
		if e != nil {
			return fmt.Errorf("failed to add parent %d to child bucket %s: %w", parentId, bktName, e)
		}

		return nil
	})

	// add child to parent list
	err = b.db.Update(func(tx *bbolt.Tx) error {
		prtBktName := "parent_" + strconv.FormatInt(parentId, 10)
		prtBkt := tx.Bucket([]byte(prtBktName))
		prtBkt.Put([]byte(childNickName), nil)

		return nil
	})

	return err
}

func (b *BoltDB) GetCurrentTransaction(userId int64) (Transaction, error) {
	var t Transaction
	err := b.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(currentTransactionName))

		v := b.Get(itob64(userId))
		if v != nil {
			if err := json.Unmarshal(v, &t); err != nil {
				return fmt.Errorf("failed to unmarshal: %w", err)
			}
		} else {
			return fmt.Errorf("current transaction not found")
		}

		return nil
	})

	return t, err
}

func (b *BoltDB) HasOpenTask(userId int64) (bool, error) {
	_, err := b.GetCurrentTransaction(userId)
	if err != nil && strings.Contains(err.Error(), "not found") {
		return false, nil
	}

	if err != nil {
		return false, err
	}

	return true, nil
}

func (b *BoltDB) CancelCurrentTask(userId int64) error {
	err := b.db.Update(func(tx *bbolt.Tx) error {
		bkt := tx.Bucket([]byte(currentTransactionName))

		var curTransaction Transaction

		v := bkt.Get(itob64(userId))
		if v != nil {
			if err := json.Unmarshal(v, &curTransaction); err != nil {
				return fmt.Errorf("failed to unmarshal: %w", err)
			}
		}

		e := bkt.Delete(itob64(userId))
		if e != nil {
			return fmt.Errorf("failed to remove current transaction: %w", e)
		}

		if err := b.UpdateTransactionStatus(curTransaction, CanceledStatus, userId); err != nil {
			return fmt.Errorf("failed to update transaction status: %w", err)
		}

		return nil
	})

	return err
}

func (b *BoltDB) UpdateTransactionStatus(t Transaction, newStatus string, userId int64) error {
	err := b.db.Update(func(tx *bbolt.Tx) error {
		trsBkt := tx.Bucket(itob64(userId))
		ts := t.Timestamp.Format(TSNano)
		val := trsBkt.Get([]byte(ts))

		if val != nil {
			var t Transaction
			if err := json.Unmarshal(val, &t); err != nil {
				return fmt.Errorf("failed to unmarshal: %w", err)
			}

			t.Status = newStatus

			buf, err := json.Marshal(t)
			if err != nil {
				return err
			}

			e := trsBkt.Put([]byte(ts), buf)

			if e != nil {
				return fmt.Errorf("failed to update transaction status: %w", e)
			}
		} else {
			return fmt.Errorf("transaction with timestamp %s not found", ts)
		}

		return nil
	})

	return err
}

func (b *BoltDB) FindParentIdByChildNickName(childNickName string) (parentId int64, err error) {
	err = b.db.View(func(tx *bbolt.Tx) error {
		bktName := "child_@" + childNickName
		bkt := tx.Bucket([]byte(bktName))

		k, _ := bkt.Cursor().First()

		if (k == nil) || (len(k) == 0) {
			return fmt.Errorf("parent for a child [%s] not found", childNickName)
		}

		parentId = int64(binary.BigEndian.Uint64(k))

		return nil
	})

	return parentId, err
}

func (b *BoltDB) FindChildren(parentId int64) (children []string, err error) {
	err = b.db.View(func(tx *bbolt.Tx) error {
		prtBktName := "parent_" + strconv.FormatInt(parentId, 10)
		prtBkt := tx.Bucket([]byte(prtBktName))

		prtBkt.ForEach(func(k, v []byte) error {
			children = append(children, string(k))
			return nil
		})

		return nil
	})
	return children, err
}

func (b *BoltDB) FindUserByNickname(nickname string) (User, error) {
	var user User

	err := b.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(usersBucketName))

		c := b.Cursor()
		for k, v := c.Last(); k != nil; k, v = c.Prev() {
			var tmpUser User

			if err := json.Unmarshal(v, &tmpUser); err != nil {
				return fmt.Errorf("failed to unmarshal: %w", err)
			}

			if tmpUser.Nickname == nickname {
				user = tmpUser
				break
			}
		}

		return nil
	})

	return user, err
}

// Close boltdb store
func (b *BoltDB) Close() error {
	return b.db.Close()
}
