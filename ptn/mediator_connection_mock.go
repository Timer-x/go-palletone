// Code generated by MockGen. DO NOT EDIT.
// Source: ./ptn/mediator_connection.go

// Package ptn is a generated GoMock package.
package ptn

import (
	gomock "github.com/golang/mock/gomock"
	common "github.com/palletone/go-palletone/common"
	event "github.com/palletone/go-palletone/common/event"
	mediatorplugin "github.com/palletone/go-palletone/consensus/mediatorplugin"
	reflect "reflect"
)

// Mockproducer is a mock of producer interface
type Mockproducer struct {
	ctrl     *gomock.Controller
	recorder *MockproducerMockRecorder
}

// MockproducerMockRecorder is the mock recorder for Mockproducer
type MockproducerMockRecorder struct {
	mock *Mockproducer
}

// NewMockproducer creates a new mock instance
func NewMockproducer(ctrl *gomock.Controller) *Mockproducer {
	mock := &Mockproducer{ctrl: ctrl}
	mock.recorder = &MockproducerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *Mockproducer) EXPECT() *MockproducerMockRecorder {
	return m.recorder
}

// SubscribeNewProducedUnitEvent mocks base method
func (m *Mockproducer) SubscribeNewProducedUnitEvent(ch chan<- mediatorplugin.NewProducedUnitEvent) event.Subscription {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SubscribeNewProducedUnitEvent", ch)
	ret0, _ := ret[0].(event.Subscription)
	return ret0
}

// SubscribeNewProducedUnitEvent indicates an expected call of SubscribeNewProducedUnitEvent
func (mr *MockproducerMockRecorder) SubscribeNewProducedUnitEvent(ch interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SubscribeNewProducedUnitEvent", reflect.TypeOf((*Mockproducer)(nil).SubscribeNewProducedUnitEvent), ch)
}

// AddToTBLSSignBufs mocks base method
func (m *Mockproducer) AddToTBLSSignBufs(newHash common.Hash) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "AddToTBLSSignBufs", newHash)
}

// AddToTBLSSignBufs indicates an expected call of AddToTBLSSignBufs
func (mr *MockproducerMockRecorder) AddToTBLSSignBufs(newHash interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AddToTBLSSignBufs", reflect.TypeOf((*Mockproducer)(nil).AddToTBLSSignBufs), newHash)
}

// SubscribeSigShareEvent mocks base method
func (m *Mockproducer) SubscribeSigShareEvent(ch chan<- mediatorplugin.SigShareEvent) event.Subscription {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SubscribeSigShareEvent", ch)
	ret0, _ := ret[0].(event.Subscription)
	return ret0
}

// SubscribeSigShareEvent indicates an expected call of SubscribeSigShareEvent
func (mr *MockproducerMockRecorder) SubscribeSigShareEvent(ch interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SubscribeSigShareEvent", reflect.TypeOf((*Mockproducer)(nil).SubscribeSigShareEvent), ch)
}

// AddToTBLSRecoverBuf mocks base method
func (m *Mockproducer) AddToTBLSRecoverBuf(sigShare *mediatorplugin.SigShareEvent) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "AddToTBLSRecoverBuf", sigShare)
}

// AddToTBLSRecoverBuf indicates an expected call of AddToTBLSRecoverBuf
func (mr *MockproducerMockRecorder) AddToTBLSRecoverBuf(sigShare interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AddToTBLSRecoverBuf", reflect.TypeOf((*Mockproducer)(nil).AddToTBLSRecoverBuf), sigShare)
}

// SubscribeVSSDealEvent mocks base method
func (m *Mockproducer) SubscribeVSSDealEvent(ch chan<- mediatorplugin.VSSDealEvent) event.Subscription {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SubscribeVSSDealEvent", ch)
	ret0, _ := ret[0].(event.Subscription)
	return ret0
}

// SubscribeVSSDealEvent indicates an expected call of SubscribeVSSDealEvent
func (mr *MockproducerMockRecorder) SubscribeVSSDealEvent(ch interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SubscribeVSSDealEvent", reflect.TypeOf((*Mockproducer)(nil).SubscribeVSSDealEvent), ch)
}

// AddToDealBuf mocks base method
func (m *Mockproducer) AddToDealBuf(deal *mediatorplugin.VSSDealEvent) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "AddToDealBuf", deal)
}

// AddToDealBuf indicates an expected call of AddToDealBuf
func (mr *MockproducerMockRecorder) AddToDealBuf(deal interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AddToDealBuf", reflect.TypeOf((*Mockproducer)(nil).AddToDealBuf), deal)
}

// SubscribeVSSResponseEvent mocks base method
func (m *Mockproducer) SubscribeVSSResponseEvent(ch chan<- mediatorplugin.VSSResponseEvent) event.Subscription {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SubscribeVSSResponseEvent", ch)
	ret0, _ := ret[0].(event.Subscription)
	return ret0
}

// SubscribeVSSResponseEvent indicates an expected call of SubscribeVSSResponseEvent
func (mr *MockproducerMockRecorder) SubscribeVSSResponseEvent(ch interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SubscribeVSSResponseEvent", reflect.TypeOf((*Mockproducer)(nil).SubscribeVSSResponseEvent), ch)
}

// AddToResponseBuf mocks base method
func (m *Mockproducer) AddToResponseBuf(resp *mediatorplugin.VSSResponseEvent) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "AddToResponseBuf", resp)
}

// AddToResponseBuf indicates an expected call of AddToResponseBuf
func (mr *MockproducerMockRecorder) AddToResponseBuf(resp interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AddToResponseBuf", reflect.TypeOf((*Mockproducer)(nil).AddToResponseBuf), resp)
}

// LocalHaveActiveMediator mocks base method
func (m *Mockproducer) LocalHaveActiveMediator() bool {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "LocalHaveActiveMediator")
	ret0, _ := ret[0].(bool)
	return ret0
}

// LocalHaveActiveMediator indicates an expected call of LocalHaveActiveMediator
func (mr *MockproducerMockRecorder) LocalHaveActiveMediator() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "LocalHaveActiveMediator", reflect.TypeOf((*Mockproducer)(nil).LocalHaveActiveMediator))
}

// LocalHavePrecedingMediator mocks base method
func (m *Mockproducer) LocalHavePrecedingMediator() bool {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "LocalHavePrecedingMediator")
	ret0, _ := ret[0].(bool)
	return ret0
}

// LocalHavePrecedingMediator indicates an expected call of LocalHavePrecedingMediator
func (mr *MockproducerMockRecorder) LocalHavePrecedingMediator() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "LocalHavePrecedingMediator", reflect.TypeOf((*Mockproducer)(nil).LocalHavePrecedingMediator))
}

// SubscribeGroupSigEvent mocks base method
func (m *Mockproducer) SubscribeGroupSigEvent(ch chan<- mediatorplugin.GroupSigEvent) event.Subscription {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SubscribeGroupSigEvent", ch)
	ret0, _ := ret[0].(event.Subscription)
	return ret0
}

// SubscribeGroupSigEvent indicates an expected call of SubscribeGroupSigEvent
func (mr *MockproducerMockRecorder) SubscribeGroupSigEvent(ch interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SubscribeGroupSigEvent", reflect.TypeOf((*Mockproducer)(nil).SubscribeGroupSigEvent), ch)
}

// UpdateMediatorsDKG mocks base method
func (m *Mockproducer) UpdateMediatorsDKG(isRenew bool) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "UpdateMediatorsDKG", isRenew)
}

// UpdateMediatorsDKG indicates an expected call of UpdateMediatorsDKG
func (mr *MockproducerMockRecorder) UpdateMediatorsDKG(isRenew interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UpdateMediatorsDKG", reflect.TypeOf((*Mockproducer)(nil).UpdateMediatorsDKG), isRenew)
}

// IsLocalMediator mocks base method
func (m *Mockproducer) IsLocalMediator(add common.Address) bool {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "IsLocalMediator", add)
	ret0, _ := ret[0].(bool)
	return ret0
}

// IsLocalMediator indicates an expected call of IsLocalMediator
func (mr *MockproducerMockRecorder) IsLocalMediator(add interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "IsLocalMediator", reflect.TypeOf((*Mockproducer)(nil).IsLocalMediator), add)
}
