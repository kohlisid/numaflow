package rpc

import (
	"fmt"
	"log"
	"time"

	flatmappb "github.com/numaproj/numaflow-go/pkg/apis/proto/flatmap/v1"
	"golang.org/x/net/context"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/numaproj/numaflow/pkg/flatmap/tracker"
	"github.com/numaproj/numaflow/pkg/flatmap/types"
	"github.com/numaproj/numaflow/pkg/isb"
	"github.com/numaproj/numaflow/pkg/sdkclient/flatmapper"
	"github.com/numaproj/numaflow/pkg/shared/logging"
)

// GRPCBasedFlatmap is a flat map applier that uses gRPC client to invoke the flat map UDF.
// It implements the applier.FlatmapApplier interface.
type GRPCBasedFlatmap struct {
	client  flatmapper.Client
	tracker *tracker.Tracker
}

func NewUDSgRPCBasedFlatmap(client flatmapper.Client) *GRPCBasedFlatmap {
	return &GRPCBasedFlatmap{client: client, tracker: tracker.NewTracker()}
}

// IsHealthy checks if the map udf is healthy.
func (u *GRPCBasedFlatmap) IsHealthy(ctx context.Context) error {
	return u.WaitUntilReady(ctx)
}

// CloseConn closes the gRPC client connection.
func (u *GRPCBasedFlatmap) CloseConn(ctx context.Context) error {
	return u.client.CloseConn(ctx)
}

// WaitUntilReady waits until the reduce udf is connected.
func (u *GRPCBasedFlatmap) WaitUntilReady(ctx context.Context) error {
	log := logging.FromContext(ctx)
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("failed on readiness check: %w", ctx.Err())
		default:
			if _, err := u.client.IsReady(ctx, &emptypb.Empty{}); err == nil {
				return nil
			} else {
				log.Infof("waiting for reduce udf to be ready: %v", err)
				time.Sleep(1 * time.Second)
			}
		}
	}
}

func (u *GRPCBasedFlatmap) ApplyMap(ctx context.Context, messageStream <-chan *isb.ReadMessage) (<-chan *types.ResponseFlatmap, <-chan error) {
	var (
		errCh        = make(chan error)
		responseCh   = make(chan *types.ResponseFlatmap)
		mapRequestCh = make(chan *flatmappb.MapRequest)
	)

	// invoke the MapFn method with mapRequestCh channel and send the result to responseCh channel
	// and any error to errCh channel
	go func() {
		log.Println("MYDEBUG: I'm processing here")
		index := 0
		resultCh, mapErrCh := u.client.MapFn(ctx, mapRequestCh)
		for {
			select {
			case result, ok := <-resultCh:
				// TODO(stream): Check error handling here
				if !ok || result == nil {
					errCh = nil
					// if the resultCh channel is closed, close the responseCh and return
					close(responseCh)
					return
				}
				// create a unique message id for each response message which will be used for deduplication
				index++
				responseCh <- u.parseMapResponse(result)
				// TODO(stream): We need to remove the request message from the tracker once this is completed.
				// As we are streaming messages, we need to have some control field to indicate that this is completed
				// now, we can do that in the SDK itself.

			case err := <-mapErrCh:
				// TODO(stream): Check error handling here
				// ctx.Done() event will be handled by the AsyncReduceFn method
				// so we don't need a separate case for ctx.Done() here
				if err == ctx.Err() {
					errCh <- err
					return
				}
				//if err != nil {
				//	errCh <- convertToUdfError(err)
				//}
			}
		}
	}()

	// create ReduceRequest from TimedWindowRequest and send it to reduceRequests channel for AsyncReduceFn
	go func() {
		// after reading all the messages from the requestsStream or if ctx was canceled close the reduceRequests channel
		defer func() {
			close(mapRequestCh)
		}()
		for {
			select {
			case msg, ok := <-messageStream:
				log.Println("MYDEBUG: reading for messages here")
				// if the requestsStream is closed or if the message is nil, return
				if !ok || msg == nil {
					//return
				}

				d := u.createFlatmapRequest(msg)
				// send the datum to reduceRequests channel, handle the case when the context is canceled
				select {
				// TODO(stream): Check the context end here
				case mapRequestCh <- d:
					log.Println("MYDEBUG: send the message here", d.Uuid)
					//case <-ctx.Done():
					//	return
				}
				// TODO(stream): Check the context end here, need to invoke shutdown
				//case <-ctx.Done(): // if the context is done, don't send any more datum to reduceRequests channel
				//	return
			}
		}
	}()

	return responseCh, errCh

}

func (u *GRPCBasedFlatmap) createFlatmapRequest(msg *isb.ReadMessage) *flatmappb.MapRequest {
	keys := msg.Keys
	payload := msg.Body.Payload

	uid := u.tracker.AddRequest(msg)

	var d = &flatmappb.MapRequest{
		Keys:      keys,
		Value:     payload,
		EventTime: timestamppb.New(msg.EventTime),
		Watermark: timestamppb.New(msg.Watermark),
		Headers:   msg.Headers,
		Uuid:      uid,
	}
	return d
}

func (u *GRPCBasedFlatmap) parseMapResponse(resp *flatmappb.MapResponse) *types.ResponseFlatmap {
	result := resp.Result
	uid := result.GetUuid()
	parentRequest, ok := u.tracker.GetRequest(uid)
	// TODO(stream): check what should be path for !ok
	if !ok {

	}
	idx, present := u.tracker.GetIdx(uid)
	if !present {
		u.tracker.NewResponse(uid)
		idx = 1
	}
	keys := result.GetKeys()
	taggedMessage := &isb.WriteMessage{
		Message: isb.Message{
			Header: isb.Header{
				MessageInfo: parentRequest.MessageInfo,
				// TODO(stream): Check what will be the unique ID to use here
				//msgId := fmt.Sprintf("%s-%d-%s-%d", u.vertexName, u.vertexReplica, partitionID.String(), index)
				ID:   fmt.Sprintf("%s-%d", parentRequest.ReadOffset.String(), idx),
				Keys: keys,
			},
			Body: isb.Body{
				Payload: result.GetValue(),
			},
		},
		Tags: result.GetTags(),
	}
	return &types.ResponseFlatmap{
		ParentMessage: parentRequest,
		Uid:           uid,
		RespMessage:   taggedMessage,
	}
}
