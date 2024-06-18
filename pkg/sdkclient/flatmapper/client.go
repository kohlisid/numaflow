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

package flatmapper

import (
	"context"
	"sync"

	flatmappb "github.com/numaproj/numaflow-go/pkg/apis/proto/flatmap/v1"
	"github.com/numaproj/numaflow-go/pkg/info"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/numaproj/numaflow/pkg/flatmap/types"
	"github.com/numaproj/numaflow/pkg/isb"
	"github.com/numaproj/numaflow/pkg/sdkclient"
	sdkerror "github.com/numaproj/numaflow/pkg/sdkclient/error"
	grpcutil "github.com/numaproj/numaflow/pkg/sdkclient/grpc"
)

// client contains the grpc connection and the grpc client.
type client struct {
	conn    *grpc.ClientConn
	grpcClt flatmappb.FlatmapClient
}

// MapFn is the RPC handler for the gRPC client (Numa container)
// It takes in a stream of input Requests, sends them to the gRPC server(UDF) and then streams the
// responses received back on a channel asynchronously.
// We spawn 2 goroutines here, one for sending the requests over the stream
// and the other one for receiving the responses
func (c client) MapFn(ctx context.Context, datum *flatmappb.MapRequest, wg *sync.WaitGroup, responseCh chan<- *types.ResponseFlatmap, errCh chan<- error, req *isb.ReadMessage) {
	defer wg.Done()
	resp, err := c.grpcClt.MapFn(ctx, datum)
	if err != nil {
		err = sdkerror.ToUDFErr("c.grpcClt.MapFn", err)
		errCh <- err
		return
	}
	//log.Print("MYDEBUG MapFn GOT RESPONSE ", resp.GetResults())
	responseCh <- &types.ResponseFlatmap{
		ParentMessage: req,
		RespMessage:   resp,
	}
}

func (c client) CloseConn(ctx context.Context) error {
	return c.conn.Close()
}

func (c client) IsReady(ctx context.Context, in *emptypb.Empty) (bool, error) {
	resp, err := c.grpcClt.IsReady(ctx, in)
	if err != nil {
		return false, err
	}
	return resp.GetReady(), nil
}

// New creates a new client object.
func New(serverInfo *info.ServerInfo, inputOptions ...sdkclient.Option) (Client, error) {
	var opts = sdkclient.DefaultOptions(sdkclient.FlatmapAddr)

	for _, inputOption := range inputOptions {
		inputOption(opts)
	}

	// Connect to the server
	conn, err := grpcutil.ConnectToServer(opts.UdsSockAddr(), serverInfo, opts.MaxMessageSize())
	if err != nil {
		return nil, err
	}

	c := new(client)
	c.conn = conn
	c.grpcClt = flatmappb.NewFlatmapClient(conn)
	return c, nil
}

//
//func TestMap(ctx context.Context, msg *flatmappb.MapRequest) []*flatmappb.MapResponse {
//	val := msg.GetValue()
//	var elements []*flatmappb.MapResponse
//	strs := strings.Split(string(val), ",")
//	for idx, x := range strs {
//		elements = append(elements, &flatmappb.MapResponse{
//			Result: &flatmappb.MapResponse_Result{
//				Keys:  msg.GetKeys(),
//				Value: []byte(x),
//				Tags:  msg.GetKeys(),
//				EOR:   false,
//				Uuid:  msg.GetUuid(),
//				Index: strconv.Itoa(idx),
//				Total: int32(len(strs)),
//			},
//		})
//		//element := &flatmappb.MapResponse{
//		//	Result: &flatmappb.MapResponse_Result{
//		//		Keys:  msg.GetKeys(),
//		//		Value: []byte(x),
//		//		Tags:  msg.GetKeys(),
//		//		EOR:   false,
//		//		Uuid:  msg.GetUuid(),
//		//		Index: strconv.Itoa(idx),
//		//		Total: int32(len(strs)),
//		//	},
//		//}
//		//responseChan <- element
//	}
//	if len(strs) == 0 {
//		// Append the EOR to indicate that the processing for the given request has completed
//		elements = append(elements, &flatmappb.MapResponse{
//			Result: &flatmappb.MapResponse_Result{
//				EOR:   true,
//				Uuid:  msg.GetUuid(),
//				Total: int32(len(strs)),
//			},
//		})
//		//element := &flatmappb.MapResponse{
//		//	Result: &flatmappb.MapResponse_Result{
//		//		EOR:   true,
//		//		Uuid:  msg.GetUuid(),
//		//		Total: int32(len(strs)),
//		//	},
//		//}
//		//responseChan <- element
//	}
//	return elements
//}
