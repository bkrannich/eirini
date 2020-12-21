// Code generated by counterfeiter. DO NOT EDIT.
package stsetfakes

import (
	"sync"

	"code.cloudfoundry.org/eirini/k8s/stset"
	"code.cloudfoundry.org/eirini/opi"
	v1 "k8s.io/api/apps/v1"
)

type FakeLRPToStatefulSet struct {
	Stub        func(string, *opi.LRP) (*v1.StatefulSet, error)
	mutex       sync.RWMutex
	argsForCall []struct {
		arg1 string
		arg2 *opi.LRP
	}
	returns struct {
		result1 *v1.StatefulSet
		result2 error
	}
	returnsOnCall map[int]struct {
		result1 *v1.StatefulSet
		result2 error
	}
	invocations      map[string][][]interface{}
	invocationsMutex sync.RWMutex
}

func (fake *FakeLRPToStatefulSet) Spy(arg1 string, arg2 *opi.LRP) (*v1.StatefulSet, error) {
	fake.mutex.Lock()
	ret, specificReturn := fake.returnsOnCall[len(fake.argsForCall)]
	fake.argsForCall = append(fake.argsForCall, struct {
		arg1 string
		arg2 *opi.LRP
	}{arg1, arg2})
	stub := fake.Stub
	returns := fake.returns
	fake.recordInvocation("LRPToStatefulSet", []interface{}{arg1, arg2})
	fake.mutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	return returns.result1, returns.result2
}

func (fake *FakeLRPToStatefulSet) CallCount() int {
	fake.mutex.RLock()
	defer fake.mutex.RUnlock()
	return len(fake.argsForCall)
}

func (fake *FakeLRPToStatefulSet) Calls(stub func(string, *opi.LRP) (*v1.StatefulSet, error)) {
	fake.mutex.Lock()
	defer fake.mutex.Unlock()
	fake.Stub = stub
}

func (fake *FakeLRPToStatefulSet) ArgsForCall(i int) (string, *opi.LRP) {
	fake.mutex.RLock()
	defer fake.mutex.RUnlock()
	return fake.argsForCall[i].arg1, fake.argsForCall[i].arg2
}

func (fake *FakeLRPToStatefulSet) Returns(result1 *v1.StatefulSet, result2 error) {
	fake.mutex.Lock()
	defer fake.mutex.Unlock()
	fake.Stub = nil
	fake.returns = struct {
		result1 *v1.StatefulSet
		result2 error
	}{result1, result2}
}

func (fake *FakeLRPToStatefulSet) ReturnsOnCall(i int, result1 *v1.StatefulSet, result2 error) {
	fake.mutex.Lock()
	defer fake.mutex.Unlock()
	fake.Stub = nil
	if fake.returnsOnCall == nil {
		fake.returnsOnCall = make(map[int]struct {
			result1 *v1.StatefulSet
			result2 error
		})
	}
	fake.returnsOnCall[i] = struct {
		result1 *v1.StatefulSet
		result2 error
	}{result1, result2}
}

func (fake *FakeLRPToStatefulSet) Invocations() map[string][][]interface{} {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.mutex.RLock()
	defer fake.mutex.RUnlock()
	copiedInvocations := map[string][][]interface{}{}
	for key, value := range fake.invocations {
		copiedInvocations[key] = value
	}
	return copiedInvocations
}

func (fake *FakeLRPToStatefulSet) recordInvocation(key string, args []interface{}) {
	fake.invocationsMutex.Lock()
	defer fake.invocationsMutex.Unlock()
	if fake.invocations == nil {
		fake.invocations = map[string][][]interface{}{}
	}
	if fake.invocations[key] == nil {
		fake.invocations[key] = [][]interface{}{}
	}
	fake.invocations[key] = append(fake.invocations[key], args)
}

var _ stset.LRPToStatefulSet = new(FakeLRPToStatefulSet).Spy
