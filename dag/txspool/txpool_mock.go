// Code generated by MockGen. DO NOT EDIT.
// Source: ./dag/txspool/interface.go

// Package txspool is a generated GoMock package.
package txspool

import (
	gomock "github.com/golang/mock/gomock"
	common "github.com/palletone/go-palletone/common"
	event "github.com/palletone/go-palletone/common/event"
	modules "github.com/palletone/go-palletone/dag/modules"
	reflect "reflect"
)

// MockITxPool is a mock of ITxPool interface
type MockITxPool struct {
	ctrl     *gomock.Controller
	recorder *MockITxPoolMockRecorder
}

// MockITxPoolMockRecorder is the mock recorder for MockITxPool
type MockITxPoolMockRecorder struct {
	mock *MockITxPool
}

// NewMockITxPool creates a new mock instance
func NewMockITxPool(ctrl *gomock.Controller) *MockITxPool {
	mock := &MockITxPool{ctrl: ctrl}
	mock.recorder = &MockITxPoolMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockITxPool) EXPECT() *MockITxPoolMockRecorder {
	return m.recorder
}

// Stop mocks base method
func (m *MockITxPool) Stop() {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "Stop")
}

// Stop indicates an expected call of Stop
func (mr *MockITxPoolMockRecorder) Stop() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Stop", reflect.TypeOf((*MockITxPool)(nil).Stop))
}

// AddLocal mocks base method
func (m *MockITxPool) AddLocal(tx *modules.Transaction) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "AddLocal", tx)
	ret0, _ := ret[0].(error)
	return ret0
}

// AddLocal indicates an expected call of AddLocal
func (mr *MockITxPoolMockRecorder) AddLocal(tx interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AddLocal", reflect.TypeOf((*MockITxPool)(nil).AddLocal), tx)
}

// AddLocals mocks base method
func (m *MockITxPool) AddLocals(txs []*modules.Transaction) []error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "AddLocals", txs)
	ret0, _ := ret[0].([]error)
	return ret0
}

// AddLocals indicates an expected call of AddLocals
func (mr *MockITxPoolMockRecorder) AddLocals(txs interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AddLocals", reflect.TypeOf((*MockITxPool)(nil).AddLocals), txs)
}

// AddSequenTx mocks base method
func (m *MockITxPool) AddSequenTx(tx *modules.Transaction) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "AddSequenTx", tx)
	ret0, _ := ret[0].(error)
	return ret0
}

// AddSequenTx indicates an expected call of AddSequenTx
func (mr *MockITxPoolMockRecorder) AddSequenTx(tx interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AddSequenTx", reflect.TypeOf((*MockITxPool)(nil).AddSequenTx), tx)
}

// AddSequenTxs mocks base method
func (m *MockITxPool) AddSequenTxs(txs []*modules.Transaction) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "AddSequenTxs", txs)
	ret0, _ := ret[0].(error)
	return ret0
}

// AddSequenTxs indicates an expected call of AddSequenTxs
func (mr *MockITxPoolMockRecorder) AddSequenTxs(txs interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AddSequenTxs", reflect.TypeOf((*MockITxPool)(nil).AddSequenTxs), txs)
}

// AllHashs mocks base method
func (m *MockITxPool) AllHashs() []*common.Hash {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "AllHashs")
	ret0, _ := ret[0].([]*common.Hash)
	return ret0
}

// AllHashs indicates an expected call of AllHashs
func (mr *MockITxPoolMockRecorder) AllHashs() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AllHashs", reflect.TypeOf((*MockITxPool)(nil).AllHashs))
}

// AllTxpoolTxs mocks base method
func (m *MockITxPool) AllTxpoolTxs() map[common.Hash]*modules.TxPoolTransaction {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "AllTxpoolTxs")
	ret0, _ := ret[0].(map[common.Hash]*modules.TxPoolTransaction)
	return ret0
}

// AllTxpoolTxs indicates an expected call of AllTxpoolTxs
func (mr *MockITxPoolMockRecorder) AllTxpoolTxs() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AllTxpoolTxs", reflect.TypeOf((*MockITxPool)(nil).AllTxpoolTxs))
}

// AddRemote mocks base method
func (m *MockITxPool) AddRemote(tx *modules.Transaction) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "AddRemote", tx)
	ret0, _ := ret[0].(error)
	return ret0
}

// AddRemote indicates an expected call of AddRemote
func (mr *MockITxPoolMockRecorder) AddRemote(tx interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AddRemote", reflect.TypeOf((*MockITxPool)(nil).AddRemote), tx)
}

// AddRemotes mocks base method
func (m *MockITxPool) AddRemotes(arg0 []*modules.Transaction) []error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "AddRemotes", arg0)
	ret0, _ := ret[0].([]error)
	return ret0
}

// AddRemotes indicates an expected call of AddRemotes
func (mr *MockITxPoolMockRecorder) AddRemotes(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AddRemotes", reflect.TypeOf((*MockITxPool)(nil).AddRemotes), arg0)
}

// ProcessTransaction mocks base method
func (m *MockITxPool) ProcessTransaction(tx *modules.Transaction, allowOrphan, rateLimit bool, tag Tag) ([]*TxDesc, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ProcessTransaction", tx, allowOrphan, rateLimit, tag)
	ret0, _ := ret[0].([]*TxDesc)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ProcessTransaction indicates an expected call of ProcessTransaction
func (mr *MockITxPoolMockRecorder) ProcessTransaction(tx, allowOrphan, rateLimit, tag interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ProcessTransaction", reflect.TypeOf((*MockITxPool)(nil).ProcessTransaction), tx, allowOrphan, rateLimit, tag)
}

// Pending mocks base method
func (m *MockITxPool) Pending() (map[common.Hash][]*modules.TxPoolTransaction, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Pending")
	ret0, _ := ret[0].(map[common.Hash][]*modules.TxPoolTransaction)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Pending indicates an expected call of Pending
func (mr *MockITxPoolMockRecorder) Pending() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Pending", reflect.TypeOf((*MockITxPool)(nil).Pending))
}

// Queued mocks base method
func (m *MockITxPool) Queued() ([]*modules.TxPoolTransaction, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Queued")
	ret0, _ := ret[0].([]*modules.TxPoolTransaction)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Queued indicates an expected call of Queued
func (mr *MockITxPoolMockRecorder) Queued() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Queued", reflect.TypeOf((*MockITxPool)(nil).Queued))
}

// SetPendingTxs mocks base method
func (m *MockITxPool) SetPendingTxs(unit_hash common.Hash, txs []*modules.Transaction) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SetPendingTxs", unit_hash, txs)
	ret0, _ := ret[0].(error)
	return ret0
}

// SetPendingTxs indicates an expected call of SetPendingTxs
func (mr *MockITxPoolMockRecorder) SetPendingTxs(unit_hash, txs interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetPendingTxs", reflect.TypeOf((*MockITxPool)(nil).SetPendingTxs), unit_hash, txs)
}

// ResetPendingTxs mocks base method
func (m *MockITxPool) ResetPendingTxs(txs []*modules.Transaction) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ResetPendingTxs", txs)
	ret0, _ := ret[0].(error)
	return ret0
}

// ResetPendingTxs indicates an expected call of ResetPendingTxs
func (mr *MockITxPoolMockRecorder) ResetPendingTxs(txs interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ResetPendingTxs", reflect.TypeOf((*MockITxPool)(nil).ResetPendingTxs), txs)
}

// SendStoredTxs mocks base method
func (m *MockITxPool) SendStoredTxs(hashs []common.Hash) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SendStoredTxs", hashs)
	ret0, _ := ret[0].(error)
	return ret0
}

// SendStoredTxs indicates an expected call of SendStoredTxs
func (mr *MockITxPoolMockRecorder) SendStoredTxs(hashs interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SendStoredTxs", reflect.TypeOf((*MockITxPool)(nil).SendStoredTxs), hashs)
}

// DiscardTxs mocks base method
func (m *MockITxPool) DiscardTxs(hashs []common.Hash) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DiscardTxs", hashs)
	ret0, _ := ret[0].(error)
	return ret0
}

// DiscardTxs indicates an expected call of DiscardTxs
func (mr *MockITxPoolMockRecorder) DiscardTxs(hashs interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DiscardTxs", reflect.TypeOf((*MockITxPool)(nil).DiscardTxs), hashs)
}

// GetUtxoEntry mocks base method
func (m *MockITxPool) GetUtxoEntry(outpoint *modules.OutPoint) (*modules.Utxo, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetUtxoEntry", outpoint)
	ret0, _ := ret[0].(*modules.Utxo)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetUtxoEntry indicates an expected call of GetUtxoEntry
func (mr *MockITxPoolMockRecorder) GetUtxoEntry(outpoint interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetUtxoEntry", reflect.TypeOf((*MockITxPool)(nil).GetUtxoEntry), outpoint)
}

// SubscribeTxPreEvent mocks base method
func (m *MockITxPool) SubscribeTxPreEvent(arg0 chan<- modules.TxPreEvent) event.Subscription {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SubscribeTxPreEvent", arg0)
	ret0, _ := ret[0].(event.Subscription)
	return ret0
}

// SubscribeTxPreEvent indicates an expected call of SubscribeTxPreEvent
func (mr *MockITxPoolMockRecorder) SubscribeTxPreEvent(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SubscribeTxPreEvent", reflect.TypeOf((*MockITxPool)(nil).SubscribeTxPreEvent), arg0)
}

// GetSortedTxs mocks base method
func (m *MockITxPool) GetSortedTxs(hash common.Hash) ([]*modules.TxPoolTransaction, common.StorageSize) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetSortedTxs", hash)
	ret0, _ := ret[0].([]*modules.TxPoolTransaction)
	ret1, _ := ret[1].(common.StorageSize)
	return ret0, ret1
}

// GetSortedTxs indicates an expected call of GetSortedTxs
func (mr *MockITxPoolMockRecorder) GetSortedTxs(hash interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetSortedTxs", reflect.TypeOf((*MockITxPool)(nil).GetSortedTxs), hash)
}

// Get mocks base method
func (m *MockITxPool) Get(hash common.Hash) (*modules.TxPoolTransaction, common.Hash) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Get", hash)
	ret0, _ := ret[0].(*modules.TxPoolTransaction)
	ret1, _ := ret[1].(common.Hash)
	return ret0, ret1
}

// Get indicates an expected call of Get
func (mr *MockITxPoolMockRecorder) Get(hash interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Get", reflect.TypeOf((*MockITxPool)(nil).Get), hash)
}

// GetPoolTxsByAddr mocks base method
func (m *MockITxPool) GetPoolTxsByAddr(addr string) ([]*modules.TxPoolTransaction, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetPoolTxsByAddr", addr)
	ret0, _ := ret[0].([]*modules.TxPoolTransaction)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetPoolTxsByAddr indicates an expected call of GetPoolTxsByAddr
func (mr *MockITxPoolMockRecorder) GetPoolTxsByAddr(addr interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetPoolTxsByAddr", reflect.TypeOf((*MockITxPool)(nil).GetPoolTxsByAddr), addr)
}

// Stats mocks base method
func (m *MockITxPool) Stats() (int, int, int) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Stats")
	ret0, _ := ret[0].(int)
	ret1, _ := ret[1].(int)
	ret2, _ := ret[2].(int)
	return ret0, ret1, ret2
}

// Stats indicates an expected call of Stats
func (mr *MockITxPoolMockRecorder) Stats() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Stats", reflect.TypeOf((*MockITxPool)(nil).Stats))
}

// Content mocks base method
func (m *MockITxPool) Content() (map[common.Hash]*modules.TxPoolTransaction, map[common.Hash]*modules.TxPoolTransaction) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Content")
	ret0, _ := ret[0].(map[common.Hash]*modules.TxPoolTransaction)
	ret1, _ := ret[1].(map[common.Hash]*modules.TxPoolTransaction)
	return ret0, ret1
}

// Content indicates an expected call of Content
func (mr *MockITxPoolMockRecorder) Content() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Content", reflect.TypeOf((*MockITxPool)(nil).Content))
}

// GetTxFee mocks base method
func (m *MockITxPool) GetTxFee(tx *modules.Transaction) (*modules.AmountAsset, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetTxFee", tx)
	ret0, _ := ret[0].(*modules.AmountAsset)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetTxFee indicates an expected call of GetTxFee
func (mr *MockITxPoolMockRecorder) GetTxFee(tx interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetTxFee", reflect.TypeOf((*MockITxPool)(nil).GetTxFee), tx)
}

// OutPointIsSpend mocks base method
func (m *MockITxPool) OutPointIsSpend(outPoint *modules.OutPoint) (bool, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "OutPointIsSpend", outPoint)
	ret0, _ := ret[0].(bool)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// OutPointIsSpend indicates an expected call of OutPointIsSpend
func (mr *MockITxPoolMockRecorder) OutPointIsSpend(outPoint interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "OutPointIsSpend", reflect.TypeOf((*MockITxPool)(nil).OutPointIsSpend), outPoint)
}

// ValidateOrphanTx mocks base method
func (m *MockITxPool) ValidateOrphanTx(tx *modules.Transaction) (bool, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ValidateOrphanTx", tx)
	ret0, _ := ret[0].(bool)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ValidateOrphanTx indicates an expected call of ValidateOrphanTx
func (mr *MockITxPoolMockRecorder) ValidateOrphanTx(tx interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ValidateOrphanTx", reflect.TypeOf((*MockITxPool)(nil).ValidateOrphanTx), tx)
}
