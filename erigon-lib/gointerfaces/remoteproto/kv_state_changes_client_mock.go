// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/ledgerwatch/erigon-lib/gointerfaces/remoteproto (interfaces: KV_StateChangesClient)
//
// Generated by this command:
//
//	mockgen -typed=true -destination=./kv_state_changes_client_mock.go -package=remoteproto . KV_StateChangesClient
//

// Package remoteproto is a generated GoMock package.
package remoteproto

import (
	context "context"
	reflect "reflect"

	gomock "go.uber.org/mock/gomock"
	metadata "google.golang.org/grpc/metadata"
)

// MockKV_StateChangesClient is a mock of KV_StateChangesClient interface.
type MockKV_StateChangesClient struct {
	ctrl     *gomock.Controller
	recorder *MockKV_StateChangesClientMockRecorder
}

// MockKV_StateChangesClientMockRecorder is the mock recorder for MockKV_StateChangesClient.
type MockKV_StateChangesClientMockRecorder struct {
	mock *MockKV_StateChangesClient
}

// NewMockKV_StateChangesClient creates a new mock instance.
func NewMockKV_StateChangesClient(ctrl *gomock.Controller) *MockKV_StateChangesClient {
	mock := &MockKV_StateChangesClient{ctrl: ctrl}
	mock.recorder = &MockKV_StateChangesClientMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockKV_StateChangesClient) EXPECT() *MockKV_StateChangesClientMockRecorder {
	return m.recorder
}

// CloseSend mocks base method.
func (m *MockKV_StateChangesClient) CloseSend() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CloseSend")
	ret0, _ := ret[0].(error)
	return ret0
}

// CloseSend indicates an expected call of CloseSend.
func (mr *MockKV_StateChangesClientMockRecorder) CloseSend() *MockKV_StateChangesClientCloseSendCall {
	mr.mock.ctrl.T.Helper()
	call := mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CloseSend", reflect.TypeOf((*MockKV_StateChangesClient)(nil).CloseSend))
	return &MockKV_StateChangesClientCloseSendCall{Call: call}
}

// MockKV_StateChangesClientCloseSendCall wrap *gomock.Call
type MockKV_StateChangesClientCloseSendCall struct {
	*gomock.Call
}

// Return rewrite *gomock.Call.Return
func (c *MockKV_StateChangesClientCloseSendCall) Return(arg0 error) *MockKV_StateChangesClientCloseSendCall {
	c.Call = c.Call.Return(arg0)
	return c
}

// Do rewrite *gomock.Call.Do
func (c *MockKV_StateChangesClientCloseSendCall) Do(f func() error) *MockKV_StateChangesClientCloseSendCall {
	c.Call = c.Call.Do(f)
	return c
}

// DoAndReturn rewrite *gomock.Call.DoAndReturn
func (c *MockKV_StateChangesClientCloseSendCall) DoAndReturn(f func() error) *MockKV_StateChangesClientCloseSendCall {
	c.Call = c.Call.DoAndReturn(f)
	return c
}

// Context mocks base method.
func (m *MockKV_StateChangesClient) Context() context.Context {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Context")
	ret0, _ := ret[0].(context.Context)
	return ret0
}

// Context indicates an expected call of Context.
func (mr *MockKV_StateChangesClientMockRecorder) Context() *MockKV_StateChangesClientContextCall {
	mr.mock.ctrl.T.Helper()
	call := mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Context", reflect.TypeOf((*MockKV_StateChangesClient)(nil).Context))
	return &MockKV_StateChangesClientContextCall{Call: call}
}

// MockKV_StateChangesClientContextCall wrap *gomock.Call
type MockKV_StateChangesClientContextCall struct {
	*gomock.Call
}

// Return rewrite *gomock.Call.Return
func (c *MockKV_StateChangesClientContextCall) Return(arg0 context.Context) *MockKV_StateChangesClientContextCall {
	c.Call = c.Call.Return(arg0)
	return c
}

// Do rewrite *gomock.Call.Do
func (c *MockKV_StateChangesClientContextCall) Do(f func() context.Context) *MockKV_StateChangesClientContextCall {
	c.Call = c.Call.Do(f)
	return c
}

// DoAndReturn rewrite *gomock.Call.DoAndReturn
func (c *MockKV_StateChangesClientContextCall) DoAndReturn(f func() context.Context) *MockKV_StateChangesClientContextCall {
	c.Call = c.Call.DoAndReturn(f)
	return c
}

// Header mocks base method.
func (m *MockKV_StateChangesClient) Header() (metadata.MD, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Header")
	ret0, _ := ret[0].(metadata.MD)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Header indicates an expected call of Header.
func (mr *MockKV_StateChangesClientMockRecorder) Header() *MockKV_StateChangesClientHeaderCall {
	mr.mock.ctrl.T.Helper()
	call := mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Header", reflect.TypeOf((*MockKV_StateChangesClient)(nil).Header))
	return &MockKV_StateChangesClientHeaderCall{Call: call}
}

// MockKV_StateChangesClientHeaderCall wrap *gomock.Call
type MockKV_StateChangesClientHeaderCall struct {
	*gomock.Call
}

// Return rewrite *gomock.Call.Return
func (c *MockKV_StateChangesClientHeaderCall) Return(arg0 metadata.MD, arg1 error) *MockKV_StateChangesClientHeaderCall {
	c.Call = c.Call.Return(arg0, arg1)
	return c
}

// Do rewrite *gomock.Call.Do
func (c *MockKV_StateChangesClientHeaderCall) Do(f func() (metadata.MD, error)) *MockKV_StateChangesClientHeaderCall {
	c.Call = c.Call.Do(f)
	return c
}

// DoAndReturn rewrite *gomock.Call.DoAndReturn
func (c *MockKV_StateChangesClientHeaderCall) DoAndReturn(f func() (metadata.MD, error)) *MockKV_StateChangesClientHeaderCall {
	c.Call = c.Call.DoAndReturn(f)
	return c
}

// Recv mocks base method.
func (m *MockKV_StateChangesClient) Recv() (*StateChangeBatch, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Recv")
	ret0, _ := ret[0].(*StateChangeBatch)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Recv indicates an expected call of Recv.
func (mr *MockKV_StateChangesClientMockRecorder) Recv() *MockKV_StateChangesClientRecvCall {
	mr.mock.ctrl.T.Helper()
	call := mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Recv", reflect.TypeOf((*MockKV_StateChangesClient)(nil).Recv))
	return &MockKV_StateChangesClientRecvCall{Call: call}
}

// MockKV_StateChangesClientRecvCall wrap *gomock.Call
type MockKV_StateChangesClientRecvCall struct {
	*gomock.Call
}

// Return rewrite *gomock.Call.Return
func (c *MockKV_StateChangesClientRecvCall) Return(arg0 *StateChangeBatch, arg1 error) *MockKV_StateChangesClientRecvCall {
	c.Call = c.Call.Return(arg0, arg1)
	return c
}

// Do rewrite *gomock.Call.Do
func (c *MockKV_StateChangesClientRecvCall) Do(f func() (*StateChangeBatch, error)) *MockKV_StateChangesClientRecvCall {
	c.Call = c.Call.Do(f)
	return c
}

// DoAndReturn rewrite *gomock.Call.DoAndReturn
func (c *MockKV_StateChangesClientRecvCall) DoAndReturn(f func() (*StateChangeBatch, error)) *MockKV_StateChangesClientRecvCall {
	c.Call = c.Call.DoAndReturn(f)
	return c
}

// RecvMsg mocks base method.
func (m *MockKV_StateChangesClient) RecvMsg(arg0 any) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RecvMsg", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// RecvMsg indicates an expected call of RecvMsg.
func (mr *MockKV_StateChangesClientMockRecorder) RecvMsg(arg0 any) *MockKV_StateChangesClientRecvMsgCall {
	mr.mock.ctrl.T.Helper()
	call := mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RecvMsg", reflect.TypeOf((*MockKV_StateChangesClient)(nil).RecvMsg), arg0)
	return &MockKV_StateChangesClientRecvMsgCall{Call: call}
}

// MockKV_StateChangesClientRecvMsgCall wrap *gomock.Call
type MockKV_StateChangesClientRecvMsgCall struct {
	*gomock.Call
}

// Return rewrite *gomock.Call.Return
func (c *MockKV_StateChangesClientRecvMsgCall) Return(arg0 error) *MockKV_StateChangesClientRecvMsgCall {
	c.Call = c.Call.Return(arg0)
	return c
}

// Do rewrite *gomock.Call.Do
func (c *MockKV_StateChangesClientRecvMsgCall) Do(f func(any) error) *MockKV_StateChangesClientRecvMsgCall {
	c.Call = c.Call.Do(f)
	return c
}

// DoAndReturn rewrite *gomock.Call.DoAndReturn
func (c *MockKV_StateChangesClientRecvMsgCall) DoAndReturn(f func(any) error) *MockKV_StateChangesClientRecvMsgCall {
	c.Call = c.Call.DoAndReturn(f)
	return c
}

// SendMsg mocks base method.
func (m *MockKV_StateChangesClient) SendMsg(arg0 any) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SendMsg", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// SendMsg indicates an expected call of SendMsg.
func (mr *MockKV_StateChangesClientMockRecorder) SendMsg(arg0 any) *MockKV_StateChangesClientSendMsgCall {
	mr.mock.ctrl.T.Helper()
	call := mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SendMsg", reflect.TypeOf((*MockKV_StateChangesClient)(nil).SendMsg), arg0)
	return &MockKV_StateChangesClientSendMsgCall{Call: call}
}

// MockKV_StateChangesClientSendMsgCall wrap *gomock.Call
type MockKV_StateChangesClientSendMsgCall struct {
	*gomock.Call
}

// Return rewrite *gomock.Call.Return
func (c *MockKV_StateChangesClientSendMsgCall) Return(arg0 error) *MockKV_StateChangesClientSendMsgCall {
	c.Call = c.Call.Return(arg0)
	return c
}

// Do rewrite *gomock.Call.Do
func (c *MockKV_StateChangesClientSendMsgCall) Do(f func(any) error) *MockKV_StateChangesClientSendMsgCall {
	c.Call = c.Call.Do(f)
	return c
}

// DoAndReturn rewrite *gomock.Call.DoAndReturn
func (c *MockKV_StateChangesClientSendMsgCall) DoAndReturn(f func(any) error) *MockKV_StateChangesClientSendMsgCall {
	c.Call = c.Call.DoAndReturn(f)
	return c
}

// Trailer mocks base method.
func (m *MockKV_StateChangesClient) Trailer() metadata.MD {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Trailer")
	ret0, _ := ret[0].(metadata.MD)
	return ret0
}

// Trailer indicates an expected call of Trailer.
func (mr *MockKV_StateChangesClientMockRecorder) Trailer() *MockKV_StateChangesClientTrailerCall {
	mr.mock.ctrl.T.Helper()
	call := mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Trailer", reflect.TypeOf((*MockKV_StateChangesClient)(nil).Trailer))
	return &MockKV_StateChangesClientTrailerCall{Call: call}
}

// MockKV_StateChangesClientTrailerCall wrap *gomock.Call
type MockKV_StateChangesClientTrailerCall struct {
	*gomock.Call
}

// Return rewrite *gomock.Call.Return
func (c *MockKV_StateChangesClientTrailerCall) Return(arg0 metadata.MD) *MockKV_StateChangesClientTrailerCall {
	c.Call = c.Call.Return(arg0)
	return c
}

// Do rewrite *gomock.Call.Do
func (c *MockKV_StateChangesClientTrailerCall) Do(f func() metadata.MD) *MockKV_StateChangesClientTrailerCall {
	c.Call = c.Call.Do(f)
	return c
}

// DoAndReturn rewrite *gomock.Call.DoAndReturn
func (c *MockKV_StateChangesClientTrailerCall) DoAndReturn(f func() metadata.MD) *MockKV_StateChangesClientTrailerCall {
	c.Call = c.Call.DoAndReturn(f)
	return c
}
