// Code generated by counterfeiter. DO NOT EDIT.
package k8sfakes

import (
	"sync"

	"code.cloudfoundry.org/eirini/k8s"
	v1 "k8s.io/api/batch/v1"
)

type FakeJobCreatingClient struct {
	CreateStub        func(string, *v1.Job) (*v1.Job, error)
	createMutex       sync.RWMutex
	createArgsForCall []struct {
		arg1 string
		arg2 *v1.Job
	}
	createReturns struct {
		result1 *v1.Job
		result2 error
	}
	createReturnsOnCall map[int]struct {
		result1 *v1.Job
		result2 error
	}
	GetByGUIDStub        func(string) ([]v1.Job, error)
	getByGUIDMutex       sync.RWMutex
	getByGUIDArgsForCall []struct {
		arg1 string
	}
	getByGUIDReturns struct {
		result1 []v1.Job
		result2 error
	}
	getByGUIDReturnsOnCall map[int]struct {
		result1 []v1.Job
		result2 error
	}
	ListStub        func() ([]v1.Job, error)
	listMutex       sync.RWMutex
	listArgsForCall []struct {
	}
	listReturns struct {
		result1 []v1.Job
		result2 error
	}
	listReturnsOnCall map[int]struct {
		result1 []v1.Job
		result2 error
	}
	invocations      map[string][][]interface{}
	invocationsMutex sync.RWMutex
}

func (fake *FakeJobCreatingClient) Create(arg1 string, arg2 *v1.Job) (*v1.Job, error) {
	fake.createMutex.Lock()
	ret, specificReturn := fake.createReturnsOnCall[len(fake.createArgsForCall)]
	fake.createArgsForCall = append(fake.createArgsForCall, struct {
		arg1 string
		arg2 *v1.Job
	}{arg1, arg2})
	fake.recordInvocation("Create", []interface{}{arg1, arg2})
	fake.createMutex.Unlock()
	if fake.CreateStub != nil {
		return fake.CreateStub(arg1, arg2)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	fakeReturns := fake.createReturns
	return fakeReturns.result1, fakeReturns.result2
}

func (fake *FakeJobCreatingClient) CreateCallCount() int {
	fake.createMutex.RLock()
	defer fake.createMutex.RUnlock()
	return len(fake.createArgsForCall)
}

func (fake *FakeJobCreatingClient) CreateCalls(stub func(string, *v1.Job) (*v1.Job, error)) {
	fake.createMutex.Lock()
	defer fake.createMutex.Unlock()
	fake.CreateStub = stub
}

func (fake *FakeJobCreatingClient) CreateArgsForCall(i int) (string, *v1.Job) {
	fake.createMutex.RLock()
	defer fake.createMutex.RUnlock()
	argsForCall := fake.createArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2
}

func (fake *FakeJobCreatingClient) CreateReturns(result1 *v1.Job, result2 error) {
	fake.createMutex.Lock()
	defer fake.createMutex.Unlock()
	fake.CreateStub = nil
	fake.createReturns = struct {
		result1 *v1.Job
		result2 error
	}{result1, result2}
}

func (fake *FakeJobCreatingClient) CreateReturnsOnCall(i int, result1 *v1.Job, result2 error) {
	fake.createMutex.Lock()
	defer fake.createMutex.Unlock()
	fake.CreateStub = nil
	if fake.createReturnsOnCall == nil {
		fake.createReturnsOnCall = make(map[int]struct {
			result1 *v1.Job
			result2 error
		})
	}
	fake.createReturnsOnCall[i] = struct {
		result1 *v1.Job
		result2 error
	}{result1, result2}
}

func (fake *FakeJobCreatingClient) GetByGUID(arg1 string) ([]v1.Job, error) {
	fake.getByGUIDMutex.Lock()
	ret, specificReturn := fake.getByGUIDReturnsOnCall[len(fake.getByGUIDArgsForCall)]
	fake.getByGUIDArgsForCall = append(fake.getByGUIDArgsForCall, struct {
		arg1 string
	}{arg1})
	fake.recordInvocation("GetByGUID", []interface{}{arg1})
	fake.getByGUIDMutex.Unlock()
	if fake.GetByGUIDStub != nil {
		return fake.GetByGUIDStub(arg1)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	fakeReturns := fake.getByGUIDReturns
	return fakeReturns.result1, fakeReturns.result2
}

func (fake *FakeJobCreatingClient) GetByGUIDCallCount() int {
	fake.getByGUIDMutex.RLock()
	defer fake.getByGUIDMutex.RUnlock()
	return len(fake.getByGUIDArgsForCall)
}

func (fake *FakeJobCreatingClient) GetByGUIDCalls(stub func(string) ([]v1.Job, error)) {
	fake.getByGUIDMutex.Lock()
	defer fake.getByGUIDMutex.Unlock()
	fake.GetByGUIDStub = stub
}

func (fake *FakeJobCreatingClient) GetByGUIDArgsForCall(i int) string {
	fake.getByGUIDMutex.RLock()
	defer fake.getByGUIDMutex.RUnlock()
	argsForCall := fake.getByGUIDArgsForCall[i]
	return argsForCall.arg1
}

func (fake *FakeJobCreatingClient) GetByGUIDReturns(result1 []v1.Job, result2 error) {
	fake.getByGUIDMutex.Lock()
	defer fake.getByGUIDMutex.Unlock()
	fake.GetByGUIDStub = nil
	fake.getByGUIDReturns = struct {
		result1 []v1.Job
		result2 error
	}{result1, result2}
}

func (fake *FakeJobCreatingClient) GetByGUIDReturnsOnCall(i int, result1 []v1.Job, result2 error) {
	fake.getByGUIDMutex.Lock()
	defer fake.getByGUIDMutex.Unlock()
	fake.GetByGUIDStub = nil
	if fake.getByGUIDReturnsOnCall == nil {
		fake.getByGUIDReturnsOnCall = make(map[int]struct {
			result1 []v1.Job
			result2 error
		})
	}
	fake.getByGUIDReturnsOnCall[i] = struct {
		result1 []v1.Job
		result2 error
	}{result1, result2}
}

func (fake *FakeJobCreatingClient) List() ([]v1.Job, error) {
	fake.listMutex.Lock()
	ret, specificReturn := fake.listReturnsOnCall[len(fake.listArgsForCall)]
	fake.listArgsForCall = append(fake.listArgsForCall, struct {
	}{})
	fake.recordInvocation("List", []interface{}{})
	fake.listMutex.Unlock()
	if fake.ListStub != nil {
		return fake.ListStub()
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	fakeReturns := fake.listReturns
	return fakeReturns.result1, fakeReturns.result2
}

func (fake *FakeJobCreatingClient) ListCallCount() int {
	fake.listMutex.RLock()
	defer fake.listMutex.RUnlock()
	return len(fake.listArgsForCall)
}

func (fake *FakeJobCreatingClient) ListCalls(stub func() ([]v1.Job, error)) {
	fake.listMutex.Lock()
	defer fake.listMutex.Unlock()
	fake.ListStub = stub
}

func (fake *FakeJobCreatingClient) ListReturns(result1 []v1.Job, result2 error) {
	fake.listMutex.Lock()
	defer fake.listMutex.Unlock()
	fake.ListStub = nil
	fake.listReturns = struct {
		result1 []v1.Job
		result2 error
	}{result1, result2}
}

func (fake *FakeJobCreatingClient) ListReturnsOnCall(i int, result1 []v1.Job, result2 error) {
	fake.listMutex.Lock()
	defer fake.listMutex.Unlock()
	fake.ListStub = nil
	if fake.listReturnsOnCall == nil {
		fake.listReturnsOnCall = make(map[int]struct {
			result1 []v1.Job
			result2 error
		})
	}
	fake.listReturnsOnCall[i] = struct {
		result1 []v1.Job
		result2 error
	}{result1, result2}
}

func (fake *FakeJobCreatingClient) Invocations() map[string][][]interface{} {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.createMutex.RLock()
	defer fake.createMutex.RUnlock()
	fake.getByGUIDMutex.RLock()
	defer fake.getByGUIDMutex.RUnlock()
	fake.listMutex.RLock()
	defer fake.listMutex.RUnlock()
	copiedInvocations := map[string][][]interface{}{}
	for key, value := range fake.invocations {
		copiedInvocations[key] = value
	}
	return copiedInvocations
}

func (fake *FakeJobCreatingClient) recordInvocation(key string, args []interface{}) {
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

var _ k8s.JobCreatingClient = new(FakeJobCreatingClient)
