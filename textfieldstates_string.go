// Code generated by "stringer -type=TextFieldStates"; DO NOT EDIT.

package gi

import (
	"fmt"
	"strconv"
)

const _TextFieldStates_name = "TextFieldNormalTextFieldFocusTextFieldDisabledTextFieldStatesN"

var _TextFieldStates_index = [...]uint8{0, 15, 29, 46, 62}

func (i TextFieldStates) String() string {
	if i < 0 || i >= TextFieldStates(len(_TextFieldStates_index)-1) {
		return "TextFieldStates(" + strconv.FormatInt(int64(i), 10) + ")"
	}
	return _TextFieldStates_name[_TextFieldStates_index[i]:_TextFieldStates_index[i+1]]
}

func StringToTextFieldStates(s string) (TextFieldStates, error) {
	for i := 0; i < len(_TextFieldStates_index)-1; i++ {
		if s == _TextFieldStates_name[_TextFieldStates_index[i]:_TextFieldStates_index[i+1]] {
			return TextFieldStates(i), nil
		}
	}
	return 0, fmt.Errorf("String %v is not a valid option for type TextFieldStates", s)
}
