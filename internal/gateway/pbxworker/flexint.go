package pbxworker

import (
	"bytes"
	"fmt"
	"strconv"
)

// FlexInt unmarshals a JSON number OR a JSON string of digits into an int.
//
// Confirmed live against the real pbx-worker (sbc.wevetel.com): fields like
// `ring_time` on action=answered-calls come back as a JSON string on some
// rows and a JSON number on others — the same PHP loose-typing behaviour
// internal/gateway/pbxcontrol.FlexString exists for on the live-control
// surface, hitting the historical-reports surface's numeric fields instead
// of its string ones. A plain `int` field decodes some rows and hard-fails
// json.Unmarshal on others, which is exactly what happened here: the whole
// answered-calls response for a real date range failed to decode over one
// row's ring_time being `"12"` instead of `12`.
//
// Always marshals back out as a plain JSON number — callers (this
// service's own Angular client) see a clean numeric contract regardless of
// what pbx-worker sent.
type FlexInt int

func (f *FlexInt) UnmarshalJSON(b []byte) error {
	if bytes.Equal(b, []byte("null")) {
		*f = 0
		return nil
	}
	trimmed := bytes.Trim(b, `"`)
	n, err := strconv.Atoi(string(trimmed))
	if err != nil {
		return fmt.Errorf("pbxworker: FlexInt: %q is not an integer: %w", b, err)
	}
	*f = FlexInt(n)
	return nil
}

func (f FlexInt) MarshalJSON() ([]byte, error) {
	return []byte(strconv.Itoa(int(f))), nil
}

func (f FlexInt) Int() int { return int(f) }
