package protoutil

import (
	"google.golang.org/protobuf/proto"
)

func MarshalDeterministic(m proto.Message) ([]byte, error) {
	return proto.MarshalOptions{
		Deterministic: true,
	}.Marshal(m)
}
