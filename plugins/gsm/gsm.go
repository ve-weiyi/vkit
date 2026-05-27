package gsm

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/ve-weiyi/vkit/plugins/gsm/service/brand"
	"github.com/ve-weiyi/vkit/plugins/gsm/service/device"
	"github.com/ve-weiyi/vkit/plugins/gsm/service/specification"
)

type GsmPlugin struct{}

func NewGsmPlugin() *GsmPlugin {
	return &GsmPlugin{}
}

func (p *GsmPlugin) Handler(prefix string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		slug := r.URL.Query().Get("slug")
		pageStr := r.URL.Query().Get("page")

		var data interface{}
		var err error

		path := strings.TrimPrefix(r.URL.Path, prefix)
		path = strings.TrimPrefix(path, "/")
		switch path {
		case "brands":
			data, err = brand.GetAllBrands()
		case "devices":
			page := 1
			if pageStr != "" {
				page, _ = strconv.Atoi(pageStr)
			}
			data, err = device.GetDeviceList(slug, page)
		case "specification":
			data, err = specification.GetSpecification(slug)
		default:
			http.NotFound(w, r)
			return
		}

		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		body, err := json.Marshal(data)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(body)
	}
}
