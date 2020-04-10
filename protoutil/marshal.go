package protoutil

import (
	"github.com/golang/protobuf/proto"
)

func MarshalDeterministic(pb proto.Message) ([]byte, error) {
	buffer := proto.NewBuffer(nil)
	buffer.SetDeterministic(true)

	if err := buffer.Marshal(pb); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}
