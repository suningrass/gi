// Code generated by "stringer -type=NodeFlags"; DO NOT EDIT.

package gi

import (
	"fmt"
	"strconv"
)

const _NodeFlags_name = "NodeFlagsNilNoLayoutEventsConnectedCanFocusHasFocusFullReRenderReRenderAnchorInactiveSelectedMouseHasEnteredDNDHasEnteredNodeDraggingOverlayButtonFlagCheckableButtonFlagCheckedButtonFlagMenuButtonFlagsNVpFlagPopupDestroyAllVpFlagSVG"

var _NodeFlags_index = [...]uint8{0, 12, 20, 35, 43, 51, 63, 77, 85, 93, 108, 121, 133, 140, 159, 176, 190, 202, 223, 232}

func (i NodeFlags) String() string {
	i -= 14
	if i < 0 || i >= NodeFlags(len(_NodeFlags_index)-1) {
		return "NodeFlags(" + strconv.FormatInt(int64(i+14), 10) + ")"
	}
	return _NodeFlags_name[_NodeFlags_index[i]:_NodeFlags_index[i+1]]
}

func StringToNodeFlags(s string) (NodeFlags, error) {
	for i := 0; i < len(_NodeFlags_index)-1; i++ {
		if s == _NodeFlags_name[_NodeFlags_index[i]:_NodeFlags_index[i+1]] {
			return NodeFlags(i + 14), nil
		}
	}
	return 0, fmt.Errorf("String %v is not a valid option for type NodeFlags", s)
}
