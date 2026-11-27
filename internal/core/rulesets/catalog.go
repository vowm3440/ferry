// Package rulesets manages the core's fixed catalog of offline SRS resources.
// Its directory and HTTP transport are trusted startup configuration, never IPC
// input. Only catalog IDs may cross the client boundary.
package rulesets

import "errors"

// Info describes one resource without exposing any local filesystem path.
type Info struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	URL       string `json:"url"`
	UpdatedAt int64  `json:"updatedAt"` // Unix milliseconds; successful file publication.
	SizeBytes int64  `json:"sizeBytes"`
	Status    string `json:"status"` // missing, ready, updating, error, or invalid.
}

const upstream = "https://raw.githubusercontent.com/SagerNet/"

var catalog = [...]Info{
	{ID: "geosite-cn", Name: "China domains", URL: upstream + "sing-geosite/rule-set/geosite-cn.srs", Status: "missing"},
	{ID: "geosite-non-cn", Name: "Non-China domains", URL: upstream + "sing-geosite/rule-set/geosite-geolocation-!cn.srs", Status: "missing"},
	{ID: "geoip-cn", Name: "China IP addresses", URL: upstream + "sing-geoip/rule-set/geoip-cn.srs", Status: "missing"},
	{ID: "geoip-private", Name: "Private IP addresses", URL: "https://raw.githubusercontent.com/MetaCubeX/meta-rules-dat/sing/geo/geoip/private.srs", Status: "missing"},
}

// Catalog returns a copy of the built-in definitions. Sources are not editable.
func Catalog() []Info { return append([]Info(nil), catalog[:]...) }

func definition(id string) (Info, error) {
	for _, item := range catalog {
		if item.ID == id {
			return item, nil
		}
	}
	return Info{}, errors.New("unknown rule-set ID")
}
