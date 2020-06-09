package pbutil

import (
	"encoding/binary"
	"io"

	"github.com/golang/protobuf/proto"
)

// WriteDelimited encodes and dumps a message to the provided writer prefixed
// with a 32-bit varint indicating the length of the encoded message, producing
// a length-delimited record stream, which can be used to chain together
// encoded messages of the same type together in a file.  It returns the total
// number of bytes written and any applicable error.  This is roughly
// equivalent to the companion Java API's MessageLite#writeDelimitedTo.
func WriteDelimited(w io.Writer, m proto.Message) (n int, err error) {
	buffer, err := proto.Marshal(m)
	if err != nil {
		return 0, err
	}

	buf := make([]byte, binary.MaxVarintLen32)
	encodedLength := binary.PutUvarint(buf, uint64(len(buffer)))

	sync, err := w.Write(buf[:encodedLength])
	if err != nil {
		return sync, err
	}

	n, err = w.Write(buffer)
	return n + sync, err
}
