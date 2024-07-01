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
	"context"
	"errors"
	"io"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"

	batchmappb "github.com/numaproj/numaflow-go/pkg/apis/proto/map/v1"
	"github.com/numaproj/numaflow-go/pkg/info"

	"github.com/numaproj/numaflow/pkg/sdkclient"
	sdkerr "github.com/numaproj/numaflow/pkg/sdkclient/error"
	grpcutil "github.com/numaproj/numaflow/pkg/sdkclient/grpc"
)

// client contains the grpc connection and the grpc client.
type client struct {
	conn    *grpc.ClientConn
	grpcClt batchmappb.MapClient
}

// New creates a new client object.
func New(serverInfo *info.ServerInfo, inputOptions ...sdkclient.Option) (Client, error) {
	var opts = sdkclient.DefaultOptions(sdkclient.MapAddr)

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
	c.grpcClt = batchmappb.NewMapClient(conn)
	return c, nil
}

func NewFromClient(c batchmappb.MapClient) (Client, error) {
	return &client{
		grpcClt: c,
	}, nil
}

// CloseConn closes the grpc client connection.
func (c *client) CloseConn(ctx context.Context) error {
	return c.conn.Close()
}

// IsReady returns true if the grpc connection is ready to use.
func (c *client) IsReady(ctx context.Context, in *emptypb.Empty) (bool, error) {
	resp, err := c.grpcClt.IsReady(ctx, in)
	if err != nil {
		return false, err
	}
	return resp.GetReady(), nil
}

// BatchMapFn is the handler for the gRPC client (Numa container)
// It takes in a stream of input Requests, sends them to the gRPC server(UDF) and then streams the
// responses received back on a channel asynchronously.
// We spawn 2 goroutines here, one for sending the requests over the stream
// and the other one for receiving the responses
func (c *client) BatchMapFn(ctx context.Context, inputCh <-chan *batchmappb.MapRequest) (<-chan *batchmappb.MapResponse, <-chan error) {
	// errCh is used to track and propagate any errors that might occur during the rpc lifecyle, these could include
	// errors in sending, UDF errors etc
	// These are propagated to the applier for further handling
	errCh := make(chan error)

	// response channel for streaming back the results received from the gRPC server
	// TODO(map-batch): Should we keep try to keep this buffered?
	responseCh := make(chan *batchmappb.MapResponse)

	// BatchMapFn is a bidirectional streaming RPC
	// We get a Map_BatchMapFnClient object over which we can send the requests,
	// receive the responses asynchronously.
	// TODO(map-batch): this creates a new gRPC stream for every batch,
	// it might be useful to see the performance difference between this approach
	// and a long-running RPC
	stream, err := c.grpcClt.MapStreamFn(ctx)
	if err != nil {
		go func() {
			errCh <- sdkerr.ToUDFErr("c.grpcClt.BatchMapFn stream", err)
		}()
		return responseCh, errCh
	}

	// read the response from the server stream and send it to responseCh channel
	// any error is sent to errCh channel
	go func() {
		// close this channel to indicate that no more elements left to receive from grpc
		// We do defer here on the whole go-routine as even during a error scenario, we
		// want to close the channel and stop forwarding any more responses from the UDF
		// as we would be replaying the current ones.
		defer close(responseCh)
		var resp *batchmappb.MapResponse
		var recvErr error
		index := 0
		for {
			select {
			case <-ctx.Done():
				errCh <- ctx.Err()
				return
			default:
				resp, recvErr = stream.Recv()
				// check if this is EOF error, which indicates that no more responses left to process on the
				// stream from the UDF, in such a case we return without any error to indicate this
				if errors.Is(recvErr, io.EOF) {
					return
				}
				// If this is some other error, propagate it to error channel,
				// also close the response channel(done using the defer close) to indicate no more messages being read
				errSDK := sdkerr.ToUDFErr("c.grpcClt.BatchMapFn", recvErr)
				if errSDK != nil {
					errCh <- errSDK
					return
				}
				// send the response upstream
				responseCh <- resp
				index += 1
			}
		}
	}()

	// Read from the read messages and send them individually to the bi-di stream for processing
	// in case there is an error in sending, send it to the error channel for handling
	go func() {
		// TODO(map-batch): Should we check for ctx.Done here as well?
		for inputMsg := range inputCh {
			err = stream.Send(inputMsg)
			if err != nil {
				go func(sErr error) {
					errCh <- sdkerr.ToUDFErr("c.grpcClt.BatchMapFn", sErr)
				}(err)
				// TODO(map-batch): Check if we should still do a CloseSend on the stream even if we have
				// received an error. Ideally on an error we would be stopping any
				// further processing and go for a replay so this should not be required.
				// but would be good to verify.
				// Otherwise this would be a break
				return
			}
		}
		// CloseSend closes the send direction of the stream. This indicates to the
		// UDF that we have sent all requests from the client, and it can safely
		// stop listening on the stream
		sendErr := stream.CloseSend()
		if sendErr != nil && !errors.Is(sendErr, io.EOF) {
			go func(sErr error) {
				errCh <- sdkerr.ToUDFErr("c.grpcClt.BatchMapFn stream.CloseSend()", sErr)
			}(sendErr)
		}
	}()
	return responseCh, errCh

}