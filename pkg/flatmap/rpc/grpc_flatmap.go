package rpc

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	flatmappb "github.com/numaproj/numaflow-go/pkg/apis/proto/flatmap/v1"
	"golang.org/x/net/context"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/numaproj/numaflow/pkg/flatmap/tracker"
	"github.com/numaproj/numaflow/pkg/flatmap/types"
	"github.com/numaproj/numaflow/pkg/isb"
	sdkerr "github.com/numaproj/numaflow/pkg/sdkclient/error"
	"github.com/numaproj/numaflow/pkg/sdkclient/flatmapper"
	"github.com/numaproj/numaflow/pkg/shared/logging"
)

// GRPCBasedFlatmap is a flat map applier that uses gRPC client to invoke the flat map UDF.
// It implements the applier.FlatmapApplier interface.
type GRPCBasedFlatmap struct {
	client     flatmapper.Client
	tracker    *tracker.Tracker
	vertexName string
}

func NewUDSgRPCBasedFlatmap(client flatmapper.Client, vertexName string) *GRPCBasedFlatmap {
	return &GRPCBasedFlatmap{client: client, tracker: tracker.NewTracker(), vertexName: vertexName}
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
	logger := logging.FromContext(ctx)
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("failed on readiness check: %w", ctx.Err())
		default:
			if _, err := u.client.IsReady(ctx, &emptypb.Empty{}); err == nil {
				return nil
			} else {
				logger.Infof("waiting for reduce udf to be ready: %v", err)
				time.Sleep(1 * time.Second)
			}
		}
	}
}

// ApplyMap applies the map udf on the stream of read messages and streams the responses back on the responseCh
// Internally, it spawns two go-routines, one for sending the requests to the client and the other to listen to the
// responses back from it.
func (u *GRPCBasedFlatmap) ApplyMap(ctx context.Context, messageStream []*isb.ReadMessage, responseCh chan<- *types.ReadWriteMessagePair) (chan struct{}, <-chan error) {
	// errCh is used to propagate any errors received from the grpc client upstream so that they can be handled
	// accordingly.
	doneChan := make(chan struct{})
	errCh := make(chan error)
	respCh := make(chan *types.ResponseFlatmap)

	go func() {
		defer close(doneChan)
		for resp := range respCh {
			parsedResp := u.parseFlatmapResponse(resp)
			//log.Print("MYDEBUG GRPC GOT RESPONSE ", *parsedResp.WriteMessages)
			responseCh <- parsedResp
		}
	}()

	go func() {
		defer close(respCh)
		wg := sync.WaitGroup{}
		for _, req := range messageStream {
			wg.Add(1)
			d := u.createMapRequest(req)
			go u.client.MapFn(ctx, d, &wg, respCh, errCh, req)
		}
		wg.Wait()
	}()
	return doneChan, errCh
}

// createMapRequest takes an isb.ReadMessage and returns proto MapRequest
func (u *GRPCBasedFlatmap) createMapRequest(msg *isb.ReadMessage) *flatmappb.MapRequest {
	keys := msg.Keys
	payload := msg.Body.Payload
	//// Add the request to the tracker, and get the unique UUID corresponding to it
	//uid := u.tracker.AddRequest(msg)
	// Create the MapRequest, with the required fields.
	var d = &flatmappb.MapRequest{
		Keys:      keys,
		Value:     payload,
		EventTime: timestamppb.New(msg.EventTime),
		Watermark: timestamppb.New(msg.Watermark),
		Headers:   msg.Headers,
	}
	return d
}

func (u *GRPCBasedFlatmap) parseFlatmapResponse(resp *types.ResponseFlatmap) *types.ReadWriteMessagePair {
	results := resp.RespMessage.GetResults()
	writeMessages := make([]*isb.WriteMessage, 0)
	parentRequest := resp.ParentMessage
	for idx, result := range results {
		keys := result.GetKeys()
		taggedMessage := &isb.WriteMessage{
			Message: isb.Message{
				Header: isb.Header{
					MessageInfo: parentRequest.MessageInfo,
					// We need this to be unique so that the ISB can execute its Dedup logic
					// this ID should be such that even when the same response is processed and received
					// again from the UDF, we still assign it the same ID.
					// The ID here will be a concat of the three values
					// parentRequest.ReadOffset - vertexName - result.Index
					//
					// ReadOffset - Will be the read offset of the request which corresponds to this response.
					// We have this stored in our tracker.
					//
					// VertexName - the name of the vertex from which this response is generated, this is
					// important to ensure that we can differentiate between messages emitted from 2 map vertices
					//
					// Result Index - This parameter is added on the SDK side.
					// We add the index of the message from the messages slice to the individual response.
					// TODO(stream): explore if there can be more robust ways to do this
					ID:   getMessageId(parentRequest.ReadOffset.String(), u.vertexName, strconv.Itoa(idx)),
					Keys: keys,
				},
				Body: isb.Body{
					Payload: result.GetValue(),
				},
			},
			Tags: result.GetTags(),
		}
		writeMessages = append(writeMessages, taggedMessage)
	}
	return &types.ReadWriteMessagePair{
		ReadMessage:   parentRequest,
		WriteMessages: &writeMessages,
	}
}

// convertToUdfError converts the error returned by the MapFn to ApplyUDFErr
func convertToUdfError(err error) error {
	udfErr, _ := sdkerr.FromError(err)
	switch udfErr.ErrorKind() {
	case sdkerr.Retryable:
		// TODO: currently we don't handle retryable errors yet
		return &ApplyUDFErr{
			UserUDFErr: false,
			Message:    fmt.Sprintf("gRPC client.MapFn failed, %s", err),
			InternalErr: InternalErr{
				Flag:        true,
				MainCarDown: false,
			},
		}
	case sdkerr.NonRetryable:
		return &ApplyUDFErr{
			UserUDFErr: false,
			Message:    fmt.Sprintf("gRPC client.MapFn failed, %s", err),
			InternalErr: InternalErr{
				Flag:        true,
				MainCarDown: false,
			},
		}
	case sdkerr.Canceled:
		return &ApplyUDFErr{
			UserUDFErr: false,
			Message:    context.Canceled.Error(),
			InternalErr: InternalErr{
				Flag:        true,
				MainCarDown: false,
			},
		}
	default:
		return &ApplyUDFErr{
			UserUDFErr: false,
			Message:    fmt.Sprintf("gRPC client.MapFn failed, %s", err),
			InternalErr: InternalErr{
				Flag:        true,
				MainCarDown: false,
			},
		}
	}
}

func getMessageId(offset string, vertexName string, index string) string {
	var idString strings.Builder
	idString.WriteString(offset)
	idString.WriteString("-")
	idString.WriteString(vertexName)
	idString.WriteString("-")
	idString.WriteString(index)
	return idString.String()
}
