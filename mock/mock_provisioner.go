// Automatically generated by MockGen. DO NOT EDIT!
// Source: github.com/docker/libmachete/provisioners/api (interfaces: Provisioner)

package mock

import (
	api "github.com/docker/libmachete/provisioners/api"
	gomock "github.com/golang/mock/gomock"
)

// Mock of Provisioner interface
type MockProvisioner struct {
	ctrl     *gomock.Controller
	recorder *_MockProvisionerRecorder
}

// Recorder for MockProvisioner (not exported)
type _MockProvisionerRecorder struct {
	mock *MockProvisioner
}

func NewMockProvisioner(ctrl *gomock.Controller) *MockProvisioner {
	mock := &MockProvisioner{ctrl: ctrl}
	mock.recorder = &_MockProvisionerRecorder{mock}
	return mock
}

func (_m *MockProvisioner) EXPECT() *_MockProvisionerRecorder {
	return _m.recorder
}

func (_m *MockProvisioner) CreateInstance(_param0 interface{}) (<-chan api.CreateInstanceEvent, error) {
	ret := _m.ctrl.Call(_m, "CreateInstance", _param0)
	ret0, _ := ret[0].(<-chan api.CreateInstanceEvent)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

func (_mr *_MockProvisionerRecorder) CreateInstance(arg0 interface{}) *gomock.Call {
	return _mr.mock.ctrl.RecordCall(_mr.mock, "CreateInstance", arg0)
}

func (_m *MockProvisioner) DestroyInstance(_param0 string) (<-chan api.DestroyInstanceEvent, error) {
	ret := _m.ctrl.Call(_m, "DestroyInstance", _param0)
	ret0, _ := ret[0].(<-chan api.DestroyInstanceEvent)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

func (_mr *_MockProvisionerRecorder) DestroyInstance(arg0 interface{}) *gomock.Call {
	return _mr.mock.ctrl.RecordCall(_mr.mock, "DestroyInstance", arg0)
}
