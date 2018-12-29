// Code generated by MockGen. DO NOT EDIT.
// Source: ./ptn/unit_sign.go

// Package ptn is a generated GoMock package.
package ptn

import (
	gomock "github.com/golang/mock/gomock"
	event "github.com/palletone/go-palletone/common/event"
	mediatorplugin "github.com/palletone/go-palletone/consensus/mediatorplugin"
	modules "github.com/palletone/go-palletone/dag/modules"
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

// SubscribeNewUnitEvent mocks base method
func (m *Mockproducer) SubscribeNewUnitEvent(ch chan<- mediatorplugin.NewProducedUnitEvent) event.Subscription {
	ret := m.ctrl.Call(m, "SubscribeNewUnitEvent", ch)
	ret0, _ := ret[0].(event.Subscription)
	return ret0
}

// SubscribeNewUnitEvent indicates an expected call of SubscribeNewUnitEvent
func (mr *MockproducerMockRecorder) SubscribeNewUnitEvent(ch interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SubscribeNewUnitEvent", reflect.TypeOf((*Mockproducer)(nil).SubscribeNewUnitEvent), ch)
}

// ToUnitTBLSSign mocks base method
func (m *Mockproducer) ToUnitTBLSSign(newUnit *modules.Unit) error {
	ret := m.ctrl.Call(m, "ToUnitTBLSSign", newUnit)
	ret0, _ := ret[0].(error)
	return ret0
}

// ToUnitTBLSSign indicates an expected call of ToUnitTBLSSign
func (mr *MockproducerMockRecorder) ToUnitTBLSSign(newUnit interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ToUnitTBLSSign", reflect.TypeOf((*Mockproducer)(nil).ToUnitTBLSSign), newUnit)
}

// SubscribeSigShareEvent mocks base method
func (m *Mockproducer) SubscribeSigShareEvent(ch chan<- mediatorplugin.SigShareEvent) event.Subscription {
	ret := m.ctrl.Call(m, "SubscribeSigShareEvent", ch)
	ret0, _ := ret[0].(event.Subscription)
	return ret0
}

// SubscribeSigShareEvent indicates an expected call of SubscribeSigShareEvent
func (mr *MockproducerMockRecorder) SubscribeSigShareEvent(ch interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SubscribeSigShareEvent", reflect.TypeOf((*Mockproducer)(nil).SubscribeSigShareEvent), ch)
}

// ToTBLSRecover mocks base method
func (m *Mockproducer) ToTBLSRecover(sigShare *mediatorplugin.SigShareEvent) error {
	ret := m.ctrl.Call(m, "ToTBLSRecover", sigShare)
	ret0, _ := ret[0].(error)
	return ret0
}

// ToTBLSRecover indicates an expected call of ToTBLSRecover
func (mr *MockproducerMockRecorder) ToTBLSRecover(sigShare interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ToTBLSRecover", reflect.TypeOf((*Mockproducer)(nil).ToTBLSRecover), sigShare)
}

// SubscribeVSSDealEvent mocks base method
func (m *Mockproducer) SubscribeVSSDealEvent(ch chan<- mediatorplugin.VSSDealEvent) event.Subscription {
	ret := m.ctrl.Call(m, "SubscribeVSSDealEvent", ch)
	ret0, _ := ret[0].(event.Subscription)
	return ret0
}

// SubscribeVSSDealEvent indicates an expected call of SubscribeVSSDealEvent
func (mr *MockproducerMockRecorder) SubscribeVSSDealEvent(ch interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SubscribeVSSDealEvent", reflect.TypeOf((*Mockproducer)(nil).SubscribeVSSDealEvent), ch)
}

// ToProcessDeal mocks base method
func (m *Mockproducer) ToProcessDeal(deal *mediatorplugin.VSSDealEvent) error {
	ret := m.ctrl.Call(m, "ToProcessDeal", deal)
	ret0, _ := ret[0].(error)
	return ret0
}

// ToProcessDeal indicates an expected call of ToProcessDeal
func (mr *MockproducerMockRecorder) ToProcessDeal(deal interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ToProcessDeal", reflect.TypeOf((*Mockproducer)(nil).ToProcessDeal), deal)
}

// SubscribeVSSResponseEvent mocks base method
func (m *Mockproducer) SubscribeVSSResponseEvent(ch chan<- mediatorplugin.VSSResponseEvent) event.Subscription {
	ret := m.ctrl.Call(m, "SubscribeVSSResponseEvent", ch)
	ret0, _ := ret[0].(event.Subscription)
	return ret0
}

// SubscribeVSSResponseEvent indicates an expected call of SubscribeVSSResponseEvent
func (mr *MockproducerMockRecorder) SubscribeVSSResponseEvent(ch interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SubscribeVSSResponseEvent", reflect.TypeOf((*Mockproducer)(nil).SubscribeVSSResponseEvent), ch)
}

// ToProcessResponse mocks base method
func (m *Mockproducer) ToProcessResponse(resp *mediatorplugin.VSSResponseEvent) error {
	ret := m.ctrl.Call(m, "ToProcessResponse", resp)
	ret0, _ := ret[0].(error)
	return ret0
}

// ToProcessResponse indicates an expected call of ToProcessResponse
func (mr *MockproducerMockRecorder) ToProcessResponse(resp interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ToProcessResponse", reflect.TypeOf((*Mockproducer)(nil).ToProcessResponse), resp)
}

// LocalHaveActiveMediator mocks base method
func (m *Mockproducer) LocalHaveActiveMediator() bool {
	ret := m.ctrl.Call(m, "LocalHaveActiveMediator")
	ret0, _ := ret[0].(bool)
	return ret0
}

// LocalHaveActiveMediator indicates an expected call of LocalHaveActiveMediator
func (mr *MockproducerMockRecorder) LocalHaveActiveMediator() *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "LocalHaveActiveMediator", reflect.TypeOf((*Mockproducer)(nil).LocalHaveActiveMediator))
}

// StartVSSProtocol mocks base method
func (m *Mockproducer) StartVSSProtocol() {
	m.ctrl.Call(m, "StartVSSProtocol")
}

// StartVSSProtocol indicates an expected call of StartVSSProtocol
func (mr *MockproducerMockRecorder) StartVSSProtocol() *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "StartVSSProtocol", reflect.TypeOf((*Mockproducer)(nil).StartVSSProtocol))
}

// SubscribeGroupSigEvent mocks base method
func (m *Mockproducer) SubscribeGroupSigEvent(ch chan<- mediatorplugin.GroupSigEvent) event.Subscription {
	ret := m.ctrl.Call(m, "SubscribeGroupSigEvent", ch)
	ret0, _ := ret[0].(event.Subscription)
	return ret0
}

// SubscribeGroupSigEvent indicates an expected call of SubscribeGroupSigEvent
func (mr *MockproducerMockRecorder) SubscribeGroupSigEvent(ch interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SubscribeGroupSigEvent", reflect.TypeOf((*Mockproducer)(nil).SubscribeGroupSigEvent), ch)
}
