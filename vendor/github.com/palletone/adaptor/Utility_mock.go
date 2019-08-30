// Code generated by MockGen. DO NOT EDIT.
// Source: ./IUtility.go

// Package adaptor is a generated GoMock package.
package adaptor

import (
	gomock "github.com/golang/mock/gomock"
	reflect "reflect"
)

// MockIUtility is a mock of IUtility interface
type MockIUtility struct {
	ctrl     *gomock.Controller
	recorder *MockIUtilityMockRecorder
}

// MockIUtilityMockRecorder is the mock recorder for MockIUtility
type MockIUtilityMockRecorder struct {
	mock *MockIUtility
}

// NewMockIUtility creates a new mock instance
func NewMockIUtility(ctrl *gomock.Controller) *MockIUtility {
	mock := &MockIUtility{ctrl: ctrl}
	mock.recorder = &MockIUtilityMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockIUtility) EXPECT() *MockIUtilityMockRecorder {
	return m.recorder
}

// NewPrivateKey mocks base method
func (m *MockIUtility) NewPrivateKey(input *NewPrivateKeyInput) (*NewPrivateKeyOutput, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "NewPrivateKey", input)
	ret0, _ := ret[0].(*NewPrivateKeyOutput)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// NewPrivateKey indicates an expected call of NewPrivateKey
func (mr *MockIUtilityMockRecorder) NewPrivateKey(input interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "NewPrivateKey", reflect.TypeOf((*MockIUtility)(nil).NewPrivateKey), input)
}

// GetPublicKey mocks base method
func (m *MockIUtility) GetPublicKey(input *GetPublicKeyInput) (*GetPublicKeyOutput, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetPublicKey", input)
	ret0, _ := ret[0].(*GetPublicKeyOutput)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetPublicKey indicates an expected call of GetPublicKey
func (mr *MockIUtilityMockRecorder) GetPublicKey(input interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetPublicKey", reflect.TypeOf((*MockIUtility)(nil).GetPublicKey), input)
}

// GetAddress mocks base method
func (m *MockIUtility) GetAddress(key *GetAddressInput) (*GetAddressOutput, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetAddress", key)
	ret0, _ := ret[0].(*GetAddressOutput)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetAddress indicates an expected call of GetAddress
func (mr *MockIUtilityMockRecorder) GetAddress(key interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetAddress", reflect.TypeOf((*MockIUtility)(nil).GetAddress), key)
}

// GetPalletOneMappingAddress mocks base method
func (m *MockIUtility) GetPalletOneMappingAddress(addr *GetPalletOneMappingAddressInput) (*GetPalletOneMappingAddressOutput, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetPalletOneMappingAddress", addr)
	ret0, _ := ret[0].(*GetPalletOneMappingAddressOutput)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetPalletOneMappingAddress indicates an expected call of GetPalletOneMappingAddress
func (mr *MockIUtilityMockRecorder) GetPalletOneMappingAddress(addr interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetPalletOneMappingAddress", reflect.TypeOf((*MockIUtility)(nil).GetPalletOneMappingAddress), addr)
}

// SignMessage mocks base method
func (m *MockIUtility) SignMessage(input *SignMessageInput) (*SignMessageOutput, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SignMessage", input)
	ret0, _ := ret[0].(*SignMessageOutput)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// SignMessage indicates an expected call of SignMessage
func (mr *MockIUtilityMockRecorder) SignMessage(input interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SignMessage", reflect.TypeOf((*MockIUtility)(nil).SignMessage), input)
}

// VerifySignature mocks base method
func (m *MockIUtility) VerifySignature(input *VerifySignatureInput) (*VerifySignatureOutput, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "VerifySignature", input)
	ret0, _ := ret[0].(*VerifySignatureOutput)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// VerifySignature indicates an expected call of VerifySignature
func (mr *MockIUtilityMockRecorder) VerifySignature(input interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "VerifySignature", reflect.TypeOf((*MockIUtility)(nil).VerifySignature), input)
}

// SignTransaction mocks base method
func (m *MockIUtility) SignTransaction(input *SignTransactionInput) (*SignTransactionOutput, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SignTransaction", input)
	ret0, _ := ret[0].(*SignTransactionOutput)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// SignTransaction indicates an expected call of SignTransaction
func (mr *MockIUtilityMockRecorder) SignTransaction(input interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SignTransaction", reflect.TypeOf((*MockIUtility)(nil).SignTransaction), input)
}

// BindTxAndSignature mocks base method
func (m *MockIUtility) BindTxAndSignature(input *BindTxAndSignatureInput) (*BindTxAndSignatureOutput, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "BindTxAndSignature", input)
	ret0, _ := ret[0].(*BindTxAndSignatureOutput)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// BindTxAndSignature indicates an expected call of BindTxAndSignature
func (mr *MockIUtilityMockRecorder) BindTxAndSignature(input interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "BindTxAndSignature", reflect.TypeOf((*MockIUtility)(nil).BindTxAndSignature), input)
}

// CalcTxHash mocks base method
func (m *MockIUtility) CalcTxHash(input *CalcTxHashInput) (*CalcTxHashOutput, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CalcTxHash", input)
	ret0, _ := ret[0].(*CalcTxHashOutput)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CalcTxHash indicates an expected call of CalcTxHash
func (mr *MockIUtilityMockRecorder) CalcTxHash(input interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CalcTxHash", reflect.TypeOf((*MockIUtility)(nil).CalcTxHash), input)
}

// SendTransaction mocks base method
func (m *MockIUtility) SendTransaction(input *SendTransactionInput) (*SendTransactionOutput, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SendTransaction", input)
	ret0, _ := ret[0].(*SendTransactionOutput)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// SendTransaction indicates an expected call of SendTransaction
func (mr *MockIUtilityMockRecorder) SendTransaction(input interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SendTransaction", reflect.TypeOf((*MockIUtility)(nil).SendTransaction), input)
}

// GetTxBasicInfo mocks base method
func (m *MockIUtility) GetTxBasicInfo(input *GetTxBasicInfoInput) (*GetTxBasicInfoOutput, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetTxBasicInfo", input)
	ret0, _ := ret[0].(*GetTxBasicInfoOutput)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetTxBasicInfo indicates an expected call of GetTxBasicInfo
func (mr *MockIUtilityMockRecorder) GetTxBasicInfo(input interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetTxBasicInfo", reflect.TypeOf((*MockIUtility)(nil).GetTxBasicInfo), input)
}

// GetBlockInfo mocks base method
func (m *MockIUtility) GetBlockInfo(input *GetBlockInfoInput) (*GetBlockInfoOutput, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetBlockInfo", input)
	ret0, _ := ret[0].(*GetBlockInfoOutput)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetBlockInfo indicates an expected call of GetBlockInfo
func (mr *MockIUtilityMockRecorder) GetBlockInfo(input interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetBlockInfo", reflect.TypeOf((*MockIUtility)(nil).GetBlockInfo), input)
}