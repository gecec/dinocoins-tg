package store

import (
	"fmt"
)

type ActionManager struct {
	db *BoltDB
}

func NewActionManager(db *BoltDB) (*ActionManager, error) {
	return &ActionManager{db: db}, nil
}

func (am *ActionManager) ConfirmTransaction(childId int64) error {
	curTransaction, err := am.db.GetCurrentTransaction(childId)
	if err != nil {
		return fmt.Errorf("unable to find current transaction: %w", err)
	}
	err = am.db.UpdateTransactionStatus(curTransaction, CompletedStatus, childId)
	if err != nil {
		return fmt.Errorf("unable to change transaction status %w", err)
	}
	_, err = am.db.ChangeBalance(childId, curTransaction.Cost)
	if err != nil {
		return fmt.Errorf("unable to change balance %w", err)
	}

	return nil
}

func (am *ActionManager) SendRequestToCompleteCurrentTask(childId int64, childName string) (string, int64, error) {
	curTrans, err := am.db.GetCurrentTransaction(childId)
	if err != nil {
		return "", -1, fmt.Errorf("unable to find current transaction %w", err)
	}

	parentId, err := am.db.FindParentIdByChildNickName(childName)
	if err != nil {
		return "", -1, fmt.Errorf("unable to find parent by child name @%s %w", childName, err)
	}

	parUser, err := am.db.FindUser(parentId)
	if err != nil {
		return "", -1, fmt.Errorf("unable to find parent user %w", err)
	}

	return curTrans.Operation, parUser.ChatID, nil
}
