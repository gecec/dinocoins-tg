package store

import (
	"encoding/json"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.etcd.io/bbolt"
	"os"
	"testing"
)

var testDB = "/tmp/test-dinocoins.db"

func TestBoltDB_CreateTransaction(t *testing.T) {
	var db, teardown = prepare(t)
	defer teardown()

	userID := int64(1)

	tr, err := db.CreateTransaction(OpWalkDog, 1)
	assert.NoError(t, err)
	assert.NotNil(t, tr)

	assert.Equal(t, OpWalkDog, tr.Operation)
	assert.Equal(t, 10, tr.Cost)
	assert.NotEmpty(t, tr.ID)
	assert.Equal(t, OpenStatus, tr.Status)
	assert.Equal(t, userID, tr.UserId)
	assert.NotEmpty(t, tr.Timestamp)

	err = db.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(itob64(userID))
		assert.NotNil(t, b)
		v := b.Get([]byte(tr.Timestamp.Format(TSNano)))
		assert.NotNil(t, v)

		transaction := Transaction{}
		err = json.Unmarshal(v, &transaction)
		assert.NoError(t, err)

		assert.Equal(t, tr.ID, transaction.ID)
		assert.Equal(t, tr.UserId, transaction.UserId)
		assert.Equal(t, tr.Status, transaction.Status)
		assert.Equal(t, tr.Operation, transaction.Operation)
		assert.Equal(t, tr.Cost, transaction.Cost)
		assert.Equal(t, tr.Timestamp.Format(TSNano), transaction.Timestamp.Format(TSNano))

		return nil
	})

	assert.NoError(t, err)

	err = db.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(currentTransactionName))
		assert.NotNil(t, b)

		v := b.Get(itob64(userID))
		assert.NotNil(t, v)

		return nil
	})

	assert.NoError(t, err)
}

func prepare(t *testing.T) (db *BoltDB, teardown func()) {
	_ = os.Remove(testDB)
	db, err := NewBoltDB(testDB)
	assert.NoError(t, err)

	teardown = func() {
		require.NoError(t, db.Close())
		_ = os.Remove(testDB)
	}

	return db, teardown
}
