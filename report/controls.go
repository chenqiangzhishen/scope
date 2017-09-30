package report

import (
	"time"
	"unsafe"

	"github.com/ugorji/go/codec"
	"github.com/weaveworks/common/mtime"
)

// Controls describe the control tags within the Nodes
type Controls map[string]Control

// A Control basically describes an RPC
type Control struct {
	ID    string `json:"id"`
	Human string `json:"human"`
	Icon  string `json:"icon"` // from https://fortawesome.github.io/Font-Awesome/cheatsheet/ please
	Rank  int    `json:"rank"`
}

// Merge merges other with cs, returning a fresh Controls.
func (cs Controls) Merge(other Controls) Controls {
	result := cs.Copy()
	for k, v := range other {
		result[k] = v
	}
	return result
}

// Copy produces a copy of cs.
func (cs Controls) Copy() Controls {
	result := Controls{}
	for k, v := range cs {
		result[k] = v
	}
	return result
}

// AddControl adds c added to cs.
func (cs Controls) AddControl(c Control) {
	cs[c.ID] = c
}

// AddControls adds a collection of controls to cs.
func (cs Controls) AddControls(controls []Control) {
	for _, c := range controls {
		cs[c.ID] = c
	}
}

// NodeControls represent the individual controls that are valid for a given
// node at a given point in time.  It's immutable. A zero-value for Timestamp
// indicated this NodeControls is 'not set'.
type NodeControls struct {
	Timestamp time.Time
	Controls  StringSet
}

var emptyNodeControls = NodeControls{Controls: MakeStringSet()}

// MakeNodeControls makes a new NodeControls
func MakeNodeControls() NodeControls {
	return emptyNodeControls
}

// Merge returns the newest of the two NodeControls; it does not take the union
// of the valid Controls.
func (nc NodeControls) Merge(other NodeControls) NodeControls {
	if nc.Timestamp.Before(other.Timestamp) {
		return other
	}
	return nc
}

// Add the new control IDs to this NodeControls, producing a fresh NodeControls.
func (nc NodeControls) Add(ids ...string) NodeControls {
	return NodeControls{
		Timestamp: mtime.Now(),
		Controls:  nc.Controls.Add(ids...),
	}
}

// WireNodeControls is the intermediate type for encoding/decoding.
// Only needed for backwards compatibility with probes
// (time.Time is encoded in binary in MsgPack)
type wireNodeControls struct {
	Timestamp string    `json:"timestamp,omitempty"`
	Controls  StringSet `json:"controls,omitempty"`
	dummySelfer
}

// CodecEncodeSelf implements codec.Selfer
func (nc *NodeControls) CodecEncodeSelf(encoder *codec.Encoder) {
	encoder.Encode(wireNodeControls{
		Timestamp: renderTime(nc.Timestamp),
		Controls:  nc.Controls,
	})
}

// CodecDecodeSelf implements codec.Selfer
// Re-coded using codec internals to avoid creating an intermediate object
// - said object escapes to the heap
func (nc *NodeControls) CodecDecodeSelf(decoder *codec.Decoder) {
	z, r := codec.GenHelperDecoder(decoder)
	if r.TryDecodeAsNil() {
		return
	}
	length := r.ReadMapStart()
	for i := 0; length < 0 || i < length; i++ {
		if length < 0 && r.CheckBreak() {
			break
		}
		z.DecSendContainerState(containerMapKey)
		// replace down to switch with DecodeStringAsBytes from newer version of codec
		buf := z.DecScratchBuffer()
		slc := r.DecodeBytes(buf[:], true, true)
		type unsafeString struct {
			Data uintptr
			Len  int
		}
		str := unsafeString{uintptr(unsafe.Pointer(&slc[0])), len(slc)}
		k := *(*string)(unsafe.Pointer(&str))
		switch string(k) {
		case "timestamp":
			// can't avoid the allocation to string without re-implementing time.Parse
			nc.Timestamp = parseTime(r.DecodeString())
		case "controls":
			if !r.TryDecodeAsNil() {
				nc.Controls.CodecDecodeSelf(decoder)
			}
		}
	}
}

// MarshalJSON shouldn't be used, use CodecEncodeSelf instead
func (NodeControls) MarshalJSON() ([]byte, error) {
	panic("MarshalJSON shouldn't be used, use CodecEncodeSelf instead")
}

// UnmarshalJSON shouldn't be used, use CodecDecodeSelf instead
func (*NodeControls) UnmarshalJSON(b []byte) error {
	panic("UnmarshalJSON shouldn't be used, use CodecDecodeSelf instead")
}

// NodeControlData contains specific information about the control. It
// is used as a Value field of LatestEntry in NodeControlDataLatestMap.
type NodeControlData struct {
	Dead bool `json:"dead"`
}
