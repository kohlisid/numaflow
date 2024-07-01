/*
Copyright 2022 The Numaproj Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package rpc

import (
	"context"
	"errors"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	mappb "github.com/numaproj/numaflow-go/pkg/apis/proto/map/v1"
	"github.com/numaproj/numaflow-go/pkg/apis/proto/map/v1/mapmock"
	"github.com/stretchr/testify/assert"

	"github.com/numaproj/numaflow/pkg/isb"
	"github.com/numaproj/numaflow/pkg/isb/testutils"
	"github.com/numaproj/numaflow/pkg/sdkclient/batchmapper"
)

func NewMockUDSGRPCBasedBatchMap(mockClient *mapmock.MockMapClient) *GRPCBasedBatchMap {
	c, _ := batchmapper.NewFromClient(mockClient)
	return NewUDSgRPCBasedBatchMap("test-vertex", c)
}

func TestGRPCBasedBatchMap_WaitUntilReady(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mapmock.NewMockMapClient(ctrl)
	mockClient.EXPECT().IsReady(gomock.Any(), gomock.Any()).Return(&mappb.ReadyResponse{Ready: true}, nil)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	go func() {
		<-ctx.Done()
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			t.Log(t.Name(), "test timeout")
		}
	}()

	u := NewMockUDSGRPCBasedBatchMap(mockClient)
	err := u.WaitUntilReady(ctx)
	assert.NoError(t, err)
}

func TestGRPCBasedBatchMap_BasicMapStreamFnWithMockClient(t *testing.T) {
	mapResponses := []mappb.MapResponse{{
		Results: []*mappb.MapResponse_Result{
			{
				Keys:  []string{"client_test"},
				Value: []byte(`test1`),
			},
		},
		Id: "0-0",
	}, {
		Results: []*mappb.MapResponse_Result{
			{
				Keys:  []string{"client_test"},
				Value: []byte(`test2`),
			},
		},
		Id: "1-0",
	}}
	t.Run("test success", func(t *testing.T) {

		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockClient := mapmock.NewMockMapClient(ctrl)
		mockMapclient := mapmock.NewMockMap_MapStreamFnClient(ctrl)

		mockMapclient.EXPECT().Send(gomock.Any()).Return(nil).AnyTimes()
		mockMapclient.EXPECT().CloseSend().Return(nil).AnyTimes()

		mockMapclient.EXPECT().Recv().Return(&mapResponses[0], nil).Times(1)
		mockMapclient.EXPECT().Recv().Return(&mapResponses[1], nil).Times(1)
		mockMapclient.EXPECT().Recv().Return(nil, io.EOF).Times(1)

		//requestsCh := make(chan *mappb.MapRequest)
		mockClient.EXPECT().MapStreamFn(gomock.Any(), gomock.Any()).Return(mockMapclient, nil)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		go func() {
			<-ctx.Done()
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				t.Log(t.Name(), "test timeout")
			}
		}()

		u := NewMockUDSGRPCBasedBatchMap(mockClient)
		//requests := []*mappb.MapRequest{{
		//	Keys:      []string{"client"},
		//	Value:     []byte(`test1`),
		//	EventTime: timestamppb.New(time.Time{}),
		//	Watermark: timestamppb.New(time.Time{}),
		//	Id:        "test1",
		//}, {
		//	Keys:      []string{"client"},
		//	Value:     []byte(`test2`),
		//	EventTime: timestamppb.New(time.Time{}),
		//	Watermark: timestamppb.New(time.Time{}),
		//	Id:        "test2",
		//}}
		//
		//var wg sync.WaitGroup
		//wg.Add(1)
		//go func() {
		//	defer wg.Done()
		//	for index := range requests {
		//		requestsCh <- requests[index]
		//	}
		//	close(requestsCh)
		//}()

		readMessages := testutils.BuildTestReadMessages(2, time.Unix(1661169600, 0), nil)

		dataMessages := make([]*isb.ReadMessage, 0)
		for _, x := range readMessages {
			u.requestTracker.AddRequest(&x)
		}

		responseCh, _ := u.ApplyBatchMap(ctx, dataMessages)
		idx := 1
		for _, response := range responseCh {
			for _, writeMessage := range response.WriteMessages {
				val := fmt.Sprintf("test%d", idx)
				assert.Equal(t, writeMessage.Payload, []byte(val))
				assert.Equal(t, writeMessage.Headers, readMessages[0].Headers)
				idx += 1
			}

		}
	})

	t.Run("test error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockClient := mapmock.NewMockMapClient(ctrl)
		mockBatchMapClient := mapmock.NewMockMap_MapStreamFnClient(ctrl)
		mockBatchMapClient.EXPECT().Send(gomock.Any()).Return(nil).AnyTimes()
		mockBatchMapClient.EXPECT().CloseSend().Return(nil).AnyTimes()
		mockBatchMapClient.EXPECT().Recv().Return(nil, errors.New("mock error for map")).Times(1)

		mockClient.EXPECT().MapStreamFn(gomock.Any(), gomock.Any()).Return(mockBatchMapClient, nil)

		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		go func() {
			<-ctx.Done()
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				t.Log(t.Name(), "test timeout")
			}
		}()

		u := NewMockUDSGRPCBasedBatchMap(mockClient)
		readMessages := testutils.BuildTestReadMessages(2, time.Unix(1661169600, 0), nil)

		dataMessages := make([]*isb.ReadMessage, 0)
		for _, x := range readMessages {
			u.requestTracker.AddRequest(&x)
		}
		_, err := u.ApplyBatchMap(ctx, dataMessages)
		assert.ErrorIs(t, err, &ApplyUDFErr{
			UserUDFErr: false,
			Message:    err.Error(),
			InternalErr: InternalErr{
				Flag:        true,
				MainCarDown: false,
			},
		})
	})

	t.Run("test context close", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockClient := mapmock.NewMockMapClient(ctrl)
		mockBatchMapClient := mapmock.NewMockMap_MapStreamFnClient(ctrl)
		mockBatchMapClient.EXPECT().Send(gomock.Any()).Return(nil).AnyTimes()
		mockBatchMapClient.EXPECT().CloseSend().Return(nil).AnyTimes()
		mockClient.EXPECT().MapStreamFn(gomock.Any(), gomock.Any()).Return(mockBatchMapClient, nil)

		ctx, cancel := context.WithTimeout(context.Background(), time.Second)

		u := NewMockUDSGRPCBasedBatchMap(mockClient)
		readMessages := testutils.BuildTestReadMessages(2, time.Unix(1661169600, 0), nil)

		dataMessages := make([]*isb.ReadMessage, 0)
		for _, x := range readMessages {
			u.requestTracker.AddRequest(&x)
		}
		// explicit cancel the context, we should see that error
		cancel()
		_, err := u.ApplyBatchMap(ctx, dataMessages)
		assert.Error(t, err, ctx.Err())
	})
}
