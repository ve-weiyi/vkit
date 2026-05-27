package tmplx

import (
	"strings"

	"github.com/ve-weiyi/vkit/x/jsonconv"
)

var StdMapUtils = map[string]any{
	"Case2Camel": jsonconv.Case2Camel,
	"Case2Snake": jsonconv.Case2Snake,
	"ToUpper":    strings.ToUpper,
	"ToLower":    strings.ToLower,
}
