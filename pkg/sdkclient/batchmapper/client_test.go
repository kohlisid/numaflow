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

package batchmapper

import (
	"fmt"
	"io"
	"reflect"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	mappb "github.com/numaproj/numaflow-go/pkg/apis/proto/map/v1"
	"github.com/numaproj/numaflow-go/pkg/apis/proto/map/v1/mapmock"
	"github.com/stretchr/testify/assert"
	"golang.org/x/net/context"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestClient_IsReady(t *testing.T) {
	var ctx = context.Background()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mapmock.NewMockMapClient(ctrl)
	mockClient.EXPECT().IsReady(gomock.Any(), gomock.Any()).Return(&mappb.ReadyResponse{Ready: true}, nil)
	mockClient.EXPECT().IsReady(gomock.Any(), gomock.Any()).Return(&mappb.ReadyResponse{Ready: false}, fmt.Errorf("mock connection refused"))

	testClient, err := NewFromClient(mockClient)
	assert.NoError(t, err)
	reflect.DeepEqual(testClient, &client{
		grpcClt: mockClient,
	})

	ready, err := testClient.IsReady(ctx, &emptypb.Empty{})
	assert.True(t, ready)
	assert.NoError(t, err)

	ready, err = testClient.IsReady(ctx, &emptypb.Empty{})
	assert.False(t, ready)
	assert.EqualError(t, err, "mock connection refused")
}

func TestClient_BatchMapFn(t *testing.T) {
	var ctx = context.Background()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mapmock.NewMockMapClient(ctrl)
	mockMapclient := mapmock.NewMockMap_MapStreamFnClient(ctrl)

	mockMapclient.EXPECT().Send(gomock.Any()).Return(nil).AnyTimes()
	mockMapclient.EXPECT().CloseSend().Return(nil).AnyTimes()
	mockMapclient.EXPECT().Recv().Return(&mappb.MapResponse{
		Results: []*mappb.MapResponse_Result{
			{
				Keys:  []string{"client_test"},
				Value: []byte(`test1`),
			},
		},
		Id: "test1",
	}, nil)
	mockMapclient.EXPECT().Recv().Return(&mappb.MapResponse{
		Results: []*mappb.MapResponse_Result{
			{
				Keys:  []string{"client_test"},
				Value: []byte(`test2`),
			},
		},
		Id: "test2",
	}, io.EOF)

	mockClient.EXPECT().MapStreamFn(gomock.Any(), gomock.Any()).Return(mockMapclient, nil)

	testClient, err := NewFromClient(mockClient)
	assert.NoError(t, err)
	reflect.DeepEqual(testClient, &client{
		grpcClt: mockClient,
	})

	messageCh := make(chan *mappb.MapRequest)
	close(messageCh)
	responseCh, _ := testClient.BatchMapFn(ctx, messageCh)
	idx := 1
	for response := range responseCh {
		id := fmt.Sprintf("test%d", idx)
		assert.Equal(t, &mappb.MapResponse{
			Results: []*mappb.MapResponse_Result{
				{
					Keys:  []string{"client_test"},
					Value: []byte(id),
				},
			},
			Id: id,
		}, response)
		idx += 1
	}
}

func TestClientContextClosed_BatchMapFn(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mapmock.NewMockMapClient(ctrl)
	mockMapclient := mapmock.NewMockMap_MapStreamFnClient(ctrl)

	mockMapclient.EXPECT().Send(gomock.Any()).Return(nil).AnyTimes()
	mockMapclient.EXPECT().CloseSend().Return(nil).AnyTimes()
	mockMapclient.EXPECT().Recv().Return(&mappb.MapResponse{
		Results: []*mappb.MapResponse_Result{
			{
				Keys:  []string{"client_test"},
				Value: []byte(`test1`),
			},
		},
		Id: "test1",
	}, nil).AnyTimes()
	mockMapclient.EXPECT().Recv().Return(&mappb.MapResponse{
		Results: []*mappb.MapResponse_Result{
			{
				Keys:  []string{"client_test"},
				Value: []byte(`test2`),
			},
		},
		Id: "test2",
	}, io.EOF).AnyTimes()

	mockClient.EXPECT().MapStreamFn(gomock.Any(), gomock.Any()).Return(mockMapclient, nil)

	testClient, err := NewFromClient(mockClient)
	assert.NoError(t, err)
	reflect.DeepEqual(testClient, &client{
		grpcClt: mockClient,
	})

	messageCh := make(chan *mappb.MapRequest)
	responseCh, errCh := testClient.BatchMapFn(ctx, messageCh)
	go func() {
		defer close(messageCh)
		requests := []*mappb.MapRequest{{
			Keys:      []string{"client"},
			Value:     []byte(`test1`),
			EventTime: timestamppb.New(time.Time{}),
			Watermark: timestamppb.New(time.Time{}),
			Id:        "test1",
		}, {
			Keys:      []string{"client"},
			Value:     []byte(`test2`),
			EventTime: timestamppb.New(time.Time{}),
			Watermark: timestamppb.New(time.Time{}),
			Id:        "test2",
		}}
		for _, req := range requests {
			messageCh <- req
		}
	}()

	//idx := 1
	caughtContextError := false
readLoop:
	for {
		select {
		case err := <-errCh:
			if err != nil {
				caughtContextError = true
				assert.Error(t, err, ctx.Err())
				break readLoop
			}
		case _, ok := <-responseCh:
			if !ok {

			}
		}
		// explicitly cancel the context
		cancel()
	}
	assert.True(t, caughtContextError)
}