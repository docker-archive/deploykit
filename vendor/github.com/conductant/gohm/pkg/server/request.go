package server

import (
	"github.com/gorilla/mux"
	"net/http"
	"reflect"
	"strconv"
	"strings"
)

func GetUrlParameter(req *http.Request, key string) string {
	vars := mux.Vars(req)
	if val, has := vars[key]; has {
		return val
	} else if err := req.ParseForm(); err == nil {
		if _, has := req.Form[key]; has {
			return req.Form[key][0]
		}
	}
	return ""
}

func GetHttpHeaders(req *http.Request, m HttpHeaders) (map[string][]string, error) {
	if m == nil {
		return nil, ErrNoHttpHeaderSpec
	}
	q := make(map[string][]string)
	for k, h := range m {
		if l, ok := req.Header[h]; ok {
			// Really strange -- you can have a 1 element list with value that's actually comma-delimited.
			if len(l) == 1 {
				q[k] = strings.Split(l[0], ", ")
			} else {
				q[k] = l
			}
		}
	}
	return q, nil
}

func GetPostForm(req *http.Request, m FormParams) (FormParams, error) {
	q, err := GetUrlQueries(req, UrlQueries(m))
	return FormParams(q), err
}

func GetUrlQueries(req *http.Request, m UrlQueries) (UrlQueries, error) {
	result := make(UrlQueries)
	for key, default_value := range m {
		actual := GetUrlParameter(req, key)
		if actual != "" {
			// Check the type and do conversion
			switch reflect.TypeOf(default_value).Kind() {
			case reflect.Bool:
				if v, err := strconv.ParseBool(actual); err != nil {
					return nil, err
				} else {
					result[key] = v
				}
			case reflect.String:
				result[key] = actual
			case reflect.Int:
				if v, err := strconv.Atoi(actual); err != nil {
					return nil, err
				} else {
					result[key] = v
				}
			case reflect.Float32:
				if v, err := strconv.ParseFloat(actual, 32); err != nil {
					return nil, err
				} else {
					result[key] = v
				}
			case reflect.Float64:
				if v, err := strconv.ParseFloat(actual, 64); err != nil {
					return nil, err
				} else {
					result[key] = v
				}
			default:
				return nil, ErrNotSupportedUrlParameterType
			}

		} else {
			result[key] = default_value
		}
	}
	return result, nil
}

func JSONContentType(req *http.Request) bool {
	return "application/json" == content_type_for_request(req)
}

func GetContentType(req *http.Request) *string {
	if req == nil {
		return nil
	} else {
		t := content_type_for_request(req)
		return &t
	}
}
