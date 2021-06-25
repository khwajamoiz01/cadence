// Copyright (c) 2021 Uber Technologies Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package thrifttests

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/common/types/mapper/thrift"
	"github.com/uber/cadence/common/types/testdata"
)

// TODO: this package is create to avoid the cycle dependency where
// "github.com/uber/cadence/common/types/mapper/thrift" imports
// "github.com/uber/cadence/common/types/mapper/testdata" imports
// "github.com/uber/cadence/common/persistence" imports
// "github.com/uber/cadence/common/types/mapper/thrift"

func TestCrossClusterTaskInfo(t *testing.T) {
	for _, item := range []*types.CrossClusterTaskInfo{nil, {}, &testdata.CrossClusterTaskInfo} {
		assert.Equal(t, item, thrift.ToCrossClusterTaskInfo(thrift.FromCrossClusterTaskInfo(item)))
	}
}

func TestCrossClusterTaskRequest(t *testing.T) {
	for _, item := range []*types.CrossClusterTaskRequest{
		nil,
		{},
		&testdata.CrossClusterTaskRequestStartChildExecution,
		&testdata.CrossClusterTaskRequestCancelExecution,
		&testdata.CrossClusterTaskRequestSignalExecution,
	} {
		assert.Equal(t, item, thrift.ToCrossClusterTaskRequest(thrift.FromCrossClusterTaskRequest(item)))
	}
}

func TestCrossClusterTaskResponse(t *testing.T) {
	for _, item := range []*types.CrossClusterTaskResponse{
		nil,
		{},
		&testdata.CrossClusterTaskResponseStartChildExecution,
		&testdata.CrossClusterTaskResponseCancelExecution,
		&testdata.CrossClusterTaskResponseSignalExecution,
	} {
		assert.Equal(t, item, thrift.ToCrossClusterTaskResponse(thrift.FromCrossClusterTaskResponse(item)))
	}
}

func TestCrossClusterTaskRequestArray(t *testing.T) {
	for _, item := range [][]*types.CrossClusterTaskRequest{nil, {}, testdata.CrossClusterTaskRequestArray} {
		assert.Equal(t, item, thrift.ToCrossClusterTaskRequestArray(thrift.FromCrossClusterTaskRequestArray(item)))
	}
}

func TestCrossClusterTaskResponseArray(t *testing.T) {
	for _, item := range [][]*types.CrossClusterTaskResponse{nil, {}, testdata.CrossClusterTaskResponseArray} {
		assert.Equal(t, item, thrift.ToCrossClusterTaskResponseArray(thrift.FromCrossClusterTaskResponseArray(item)))
	}
}

func TestCrossClusterTaskRequestMap(t *testing.T) {
	for _, item := range []map[int32][]*types.CrossClusterTaskRequest{nil, {}, testdata.CrossClusterTaskRequestMap} {
		assert.Equal(t, item, thrift.ToCrossClusterTaskRequestMap(thrift.FromCrossClusterTaskRequestMap(item)))
	}
}

func TestGetCrossClusterTasksRequest(t *testing.T) {
	for _, item := range []*types.GetCrossClusterTasksRequest{nil, {}, &testdata.GetCrossClusterTasksRequest} {
		assert.Equal(t, item, thrift.ToGetCrossClusterTasksRequest(thrift.FromGetCrossClusterTasksRequest(item)))
	}
}

func TestGetCrossClusterTasksResponse(t *testing.T) {
	for _, item := range []*types.GetCrossClusterTasksResponse{nil, {}, &testdata.GetCrossClusterTasksResponse} {
		assert.Equal(t, item, thrift.ToGetCrossClusterTasksResponse(thrift.FromGetCrossClusterTasksResponse(item)))
	}
}

func TestRespondCrossClusterTasksCompletedRequest(t *testing.T) {
	for _, item := range []*types.RespondCrossClusterTasksCompletedRequest{nil, {}, &testdata.RespondCrossClusterTasksCompletedRequest} {
		assert.Equal(t, item, thrift.ToRespondCrossClusterTasksCompletedRequest(thrift.FromRespondCrossClusterTasksCompletedRequest(item)))
	}
}

func TestRespondCrossClusterTasksCompletedResponse(t *testing.T) {
	for _, item := range []*types.RespondCrossClusterTasksCompletedResponse{nil, {}, &testdata.RespondCrossClusterTasksCompletedResponse} {
		assert.Equal(t, item, thrift.ToRespondCrossClusterTasksCompletedResponse(thrift.FromRespondCrossClusterTasksCompletedResponse(item)))
	}
}
