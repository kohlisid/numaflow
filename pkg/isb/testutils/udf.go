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

package testutils

import (
	"context"

	"github.com/numaproj/numaflow/pkg/isb"
)

// CopyUDFTestApply applies a copy UDF that simply copies the input to output.
func CopyUDFTestApply(ctx context.Context, vertexName string, readMessages []*isb.ReadMessage) ([]isb.ReadWriteMessagePair, error) {
	udfResults := make([]isb.ReadWriteMessagePair, len(readMessages))
	for i, readMessage := range readMessages {
		offset := readMessage.ReadOffset
		payload := readMessage.Body.Payload
		parentPaneInfo := readMessage.MessageInfo
		// copy the payload
		result := payload
		var keys []string

		writeMessage := isb.Message{
			Header: isb.Header{
				MessageInfo: parentPaneInfo,
				ID: isb.MessageID{
					VertexName: vertexName,
					Offset:     offset.String(),
				},
				Keys: keys,
			},
			Body: isb.Body{
				Payload: result,
			},
		}
		udfResults[i] = isb.ReadWriteMessagePair{
			ReadMessage:   readMessage,
			WriteMessages: []*isb.WriteMessage{{Message: writeMessage}},
		}
	}
	return udfResults, nil
}

func CopyUDFTestApplyStream(ctx context.Context, vertexName string, writeMessageCh chan<- isb.WriteMessage, readMessage *isb.ReadMessage) error {
	defer close(writeMessageCh)
	_ = ctx
	offset := readMessage.ReadOffset
	payload := readMessage.Body.Payload
	parentPaneInfo := readMessage.MessageInfo

	// apply UDF
	_ = payload
	// copy the payload
	result := payload
	var keys []string

	writeMessage := isb.Message{
		Header: isb.Header{
			MessageInfo: parentPaneInfo,
			ID:          isb.MessageID{VertexName: vertexName, Offset: offset.String()},
			Keys:        keys,
		},
		Body: isb.Body{
			Payload: result,
		},
	}

	writeMessageCh <- isb.WriteMessage{Message: writeMessage}
	return nil
}

func CopyUDFTestApplyBatchMap(ctx context.Context, vertexName string, readMessages []*isb.ReadMessage) ([]isb.ReadWriteMessagePair, error) {
	udfResults := make([]isb.ReadWriteMessagePair, len(readMessages))

	for idx, readMessage := range readMessages {
		offset := readMessage.ReadOffset
		payload := readMessage.Body.Payload
		parentPaneInfo := readMessage.MessageInfo
		// copy the payload
		result := payload

		writeMessage := isb.Message{
			Header: isb.Header{
				MessageInfo: parentPaneInfo,
				ID: isb.MessageID{
					VertexName: vertexName,
					Offset:     offset.String(),
					Index:      0,
				},
				Keys: readMessage.Keys,
			},
			Body: isb.Body{
				Payload: result,
			},
		}
		writeMessage.Headers = readMessage.Headers
		taggedMessage := &isb.WriteMessage{
			Message: writeMessage,
			Tags:    nil,
		}
		udfResults[idx].WriteMessages = []*isb.WriteMessage{taggedMessage}
		udfResults[idx].ReadMessage = readMessage
	}

	return udfResults, nil
}
