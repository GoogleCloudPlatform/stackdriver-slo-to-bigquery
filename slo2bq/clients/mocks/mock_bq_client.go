// Code generated by MockGen. DO NOT EDIT.
// Source: slo2bq/clients (interfaces: BigQueryClient)

// Package mocks is a generated GoMock package.
package mocks

import (
	context "context"
	gomock "github.com/golang/mock/gomock"
	reflect "reflect"
	clients "slo2bq/clients"
)

// MockBigQueryClient is a mock of BigQueryClient interface
type MockBigQueryClient struct {
	ctrl     *gomock.Controller
	recorder *MockBigQueryClientMockRecorder
}

// MockBigQueryClientMockRecorder is the mock recorder for MockBigQueryClient
type MockBigQueryClientMockRecorder struct {
	mock *MockBigQueryClient
}

// NewMockBigQueryClient creates a new mock instance
func NewMockBigQueryClient(ctrl *gomock.Controller) *MockBigQueryClient {
	mock := &MockBigQueryClient{ctrl: ctrl}
	mock.recorder = &MockBigQueryClientMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockBigQueryClient) EXPECT() *MockBigQueryClientMockRecorder {
	return m.recorder
}

// Close mocks base method
func (m *MockBigQueryClient) Close() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Close")
	ret0, _ := ret[0].(error)
	return ret0
}

// Close indicates an expected call of Close
func (mr *MockBigQueryClientMockRecorder) Close() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Close", reflect.TypeOf((*MockBigQueryClient)(nil).Close))
}

// Put mocks base method
func (m *MockBigQueryClient) Put(arg0 context.Context, arg1, arg2 string, arg3 []*clients.BQRow) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Put", arg0, arg1, arg2, arg3)
	ret0, _ := ret[0].(error)
	return ret0
}

// Put indicates an expected call of Put
func (mr *MockBigQueryClientMockRecorder) Put(arg0, arg1, arg2, arg3 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Put", reflect.TypeOf((*MockBigQueryClient)(nil).Put), arg0, arg1, arg2, arg3)
}

// Query mocks base method
func (m *MockBigQueryClient) Query(arg0 context.Context, arg1 string) ([]*clients.BQRow, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Query", arg0, arg1)
	ret0, _ := ret[0].([]*clients.BQRow)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Query indicates an expected call of Query
func (mr *MockBigQueryClientMockRecorder) Query(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Query", reflect.TypeOf((*MockBigQueryClient)(nil).Query), arg0, arg1)
}

// ReadDatasetMetadataLabel mocks base method
func (m *MockBigQueryClient) ReadDatasetMetadataLabel(arg0 context.Context, arg1, arg2 string) (string, string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ReadDatasetMetadataLabel", arg0, arg1, arg2)
	ret0, _ := ret[0].(string)
	ret1, _ := ret[1].(string)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// ReadDatasetMetadataLabel indicates an expected call of ReadDatasetMetadataLabel
func (mr *MockBigQueryClientMockRecorder) ReadDatasetMetadataLabel(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ReadDatasetMetadataLabel", reflect.TypeOf((*MockBigQueryClient)(nil).ReadDatasetMetadataLabel), arg0, arg1, arg2)
}

// WriteDatasetMetadataLabel mocks base method
func (m *MockBigQueryClient) WriteDatasetMetadataLabel(arg0 context.Context, arg1, arg2, arg3, arg4 string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "WriteDatasetMetadataLabel", arg0, arg1, arg2, arg3, arg4)
	ret0, _ := ret[0].(error)
	return ret0
}

// WriteDatasetMetadataLabel indicates an expected call of WriteDatasetMetadataLabel
func (mr *MockBigQueryClientMockRecorder) WriteDatasetMetadataLabel(arg0, arg1, arg2, arg3, arg4 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "WriteDatasetMetadataLabel", reflect.TypeOf((*MockBigQueryClient)(nil).WriteDatasetMetadataLabel), arg0, arg1, arg2, arg3, arg4)
}
