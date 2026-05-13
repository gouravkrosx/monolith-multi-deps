// Package taskpb provides hand-written gRPC types and service descriptors.
//
// Rather than requiring `protoc` as a build-time dependency, this package
// uses a JSON-based gRPC codec ("subtype json"). The `proto/task.proto` file
// remains the canonical schema; the Go types here mirror it 1:1.
package taskpb

import (
	"encoding/json"

	"google.golang.org/grpc/encoding"
)

const CodecName = "json"

type jsonCodec struct{}

func (jsonCodec) Marshal(v any) ([]byte, error)      { return json.Marshal(v) }
func (jsonCodec) Unmarshal(data []byte, v any) error { return json.Unmarshal(data, v) }
func (jsonCodec) Name() string                       { return CodecName }

func init() { encoding.RegisterCodec(jsonCodec{}) }
