package main

import (
	"google.golang.org/grpc/encoding"

	// register JSON codec for gRPC
	_ "github.com/keploy/taskmanager/proto/taskpb"
)

func encodingCodec() encoding.Codec {
	return encoding.GetCodec("json")
}
