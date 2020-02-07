package dispatcher

import (
	"context"
	"fmt"
	"reflect"

	"github.com/renproject/darknode/jsonrpc"
	"github.com/renproject/lightnode/http"
	"github.com/sirupsen/logrus"
)

// Iterator reads response from the given channel, collects response and
// combines them to a single response depending on different strategies. It
// tries to cancel the ctx when it has the response.
type Iterator interface {
	Collect(id interface{}, cancel context.CancelFunc, responses <-chan jsonrpc.Response) jsonrpc.Response
}

// firstSuccessfulResponseIterator returns the first successful response it gets
// and stop waiting for responses from the rest darknodes.
type firstSuccessfulResponseIterator struct {
}

// NewFirstResponseIterator creates a new firstSuccessfulResponseIterator.
func NewFirstResponseIterator() Iterator {
	return firstSuccessfulResponseIterator{}
}

// Collect implements the Iterator interface.
func (iter firstSuccessfulResponseIterator) Collect(id interface{}, cancel context.CancelFunc, responses <-chan jsonrpc.Response) jsonrpc.Response {
	defer cancel()

	errMsg := ""
	for response := range responses {
		if response.Error == nil {
			return response
		}
		errMsg += fmt.Sprintf("%v, ", response.Error.Message)
	}

	errMsg = fmt.Sprintf("lightnode could not forward request to darknode: [ %v ]", errMsg)
	jsonErr := jsonrpc.NewError(http.ErrorCodeForwardingError, errMsg, nil)
	return jsonrpc.NewResponse(id, nil, &jsonErr)
}

// majorityResponseIterator select and returns the response returned by majority
// darknodes.
type majorityResponseIterator struct {
	logger    logrus.FieldLogger
	responses *interfaceMap
}

// NewMajorityResponseIterator returns a new majorityResponseIterator.
func NewMajorityResponseIterator(logger logrus.FieldLogger) Iterator {
	return majorityResponseIterator{
		logger: logger,
	}
}

// Collect implements the `Iterator` interface.
func (iter majorityResponseIterator) Collect(id interface{}, cancel context.CancelFunc, responses <-chan jsonrpc.Response) jsonrpc.Response {
	iter.responses = newInterfaceMap(cap(responses))
	defer cancel()

	for response := range responses {
		if ok := iter.responses.store(response); ok {
			return response
		}
	}

	return iter.responses.most().(jsonrpc.Response)
}

// interfaceMap use to is a customized map for storing interface{}. It uses
// reflect.Deepequal function to compare interface{}.
type interfaceMap struct {
	threshold int
	counter   []int
	data      []interface{}
}

// newInterfaceMap creates a new interfaceMap.
func newInterfaceMap(total int) *interfaceMap {
	return &interfaceMap{
		threshold: (total - 1) / 3 * 2,
		counter:   make([]int, 0, total),
		data:      make([]interface{}, 0, total),
	}
}

// store increments the counter by 1 if we already have the same interface,
// otherwise it store the new key with a counter starting from 1.
// todo : bool doesn't mean whether the store operation succeed which can be confusing.
func (m *interfaceMap) store(key interface{}) bool {
	for i := range m.data {
		if reflect.DeepEqual(key, m.data[i]) {
			m.counter[i]++
			return m.counter[i] > m.threshold
		}
	}
	m.data = append(m.data, key)
	m.counter = append(m.counter, 1)
	return 1 > m.threshold
}

func (m *interfaceMap) most() interface{} {
	max, index := 0, 0
	for i := range m.counter {
		if m.counter[i] > max {
			max = m.counter[i]
			index = i
		}
	}
	return m.data[index]
}
