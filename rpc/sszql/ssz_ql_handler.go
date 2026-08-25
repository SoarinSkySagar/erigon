package sszql

import (
	"encoding/json"
	"mime"
	"net/http"
	"strconv"
	"strings"
)

const sszQLContentType = "application/json"

func SSZQueryHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleSSZQuery(w, r)
	})
}

func handleSSZQuery(w http.ResponseWriter, r *http.Request) {

	mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mt != sszQLContentType {
		writeQueryError(w, http.StatusUnsupportedMediaType, "unsupported media type, only "+sszQLContentType+" is supported")
		return
	}

	versionValue := r.PathValue("version")
	if !strings.HasPrefix(versionValue, "v") {
		writeQueryError(w, http.StatusNotFound, "invalid version")
		return
	}

	v := strings.TrimPrefix(versionValue, "v")
	if len(v) > 1 && v[0] == '0' {
		writeQueryError(w, http.StatusNotFound, "invalid version")
		return
	}
	parsed, err := strconv.ParseUint(v, 10, 8)
	if err != nil {
		writeQueryError(w, http.StatusNotFound, "invalid version")
		return
	}
	version := uint(parsed)

	blockID := r.PathValue("blockID")

	layer := r.PathValue("layer")

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	var req SSZQLRequest

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		writeQueryError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if dec.More() {
		writeQueryError(w, http.StatusBadRequest, "invalid JSON: unexpected data after request body")
		return
	}
	if len(req.Queries) == 0 {
		writeQueryError(w, http.StatusBadRequest, "invalid JSON: queries must not be empty")
		return
	}

	var res SSZQLResponse

	switch version {
	case 1:
		res, err = parseQueryV1(req, version, layer, blockID)
	default:
		writeQueryError(w, http.StatusNotFound, "unsupported API version")
		return
	}

	if err != nil {
		writeQueryError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeQueryResponse(w, res)
}

type queryError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func writeQueryError(w http.ResponseWriter, code int, message string) {
	b, err := json.Marshal(queryError{Code: code, Message: message})
	if err != nil {
		b = []byte(`{"code":500,"message":"internal error"}`)
		code = http.StatusInternalServerError
	}

	w.Header().Set("Content-Type", sszQLContentType)
	w.WriteHeader(code)
	_, _ = w.Write(append(b, '\n'))
}

func writeQueryResponse(w http.ResponseWriter, res SSZQLResponse) {
	b, err := json.Marshal(res)
	if err != nil {
		writeQueryError(w, http.StatusInternalServerError, "invalid response: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", sszQLContentType)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(append(b, '\n'))
}
