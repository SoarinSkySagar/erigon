package sszql

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func doRequest(t *testing.T, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	rec := httptest.NewRecorder()
	SSZQueryHandler().ServeHTTP(rec, req)
	return rec
}

const validQueryBody = `{"queries":[{"anchor":"execution_block","path":".transactions[0].to"}]}`

func TestRouteMethodNotAllowed(t *testing.T) {
	for _, method := range []string{http.MethodGet, http.MethodPut, http.MethodDelete, http.MethodPatch} {
		rec := doRequest(t, method, "/eth/v1/execution/123/query", validQueryBody)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s: got status %d, want %d", method, rec.Code, http.StatusMethodNotAllowed)
		}
	}
}

func TestRoutePathMatching(t *testing.T) {
	tests := []struct {
		name   string
		path   string
		status int
	}{
		{"valid v1", "/eth/v1/execution/123/query", http.StatusOK},
		{"valid v6", "/eth/v6/execution/head/query", http.StatusOK},
		{"trailing slash tolerated", "/eth/v1/execution/123/query/", http.StatusOK},
		{"too few segments", "/eth/v1/execution/query", http.StatusNotFound},
		{"too many segments", "/eth/v1/execution/123/extra/query", http.StatusNotFound},
		{"wrong root", "/beacon/v1/execution/123/query", http.StatusNotFound},
		{"missing v prefix", "/eth/1/execution/123/query", http.StatusNotFound},
		{"wrong domain", "/eth/v1/consensus/123/query", http.StatusNotFound},
		{"wrong suffix", "/eth/v1/execution/123/prove", http.StatusNotFound},
		{"non-numeric version", "/eth/vx/execution/123/query", http.StatusNotFound},
		{"version too low", "/eth/v0/execution/123/query", http.StatusNotFound},
		{"version too high", "/eth/v7/execution/123/query", http.StatusNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := doRequest(t, http.MethodPost, tt.path, validQueryBody)
			if rec.Code != tt.status {
				t.Errorf("path %q: got status %d, want %d", tt.path, rec.Code, tt.status)
			}
		})
	}
}

func TestRouteInvalidJSON(t *testing.T) {
	for _, body := range []string{"", "not json", `{"queries":`} {
		rec := doRequest(t, http.MethodPost, "/eth/v1/execution/123/query", body)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("body %q: got status %d, want %d", body, rec.Code, http.StatusBadRequest)
		}
	}
}

func TestRouteValidResponse(t *testing.T) {
	rec := doRequest(t, http.MethodPost, "/eth/v1/execution/123/query", validQueryBody)

	if rec.Code != http.StatusOK {
		t.Fatalf("got status %d, want %d", rec.Code, http.StatusOK)
	}
	if ct := rec.Header().Get("Content-Type"); ct != sszQLContentType {
		t.Errorf("Content-Type: got %q, want %q", ct, sszQLContentType)
	}

	var res SSZQLResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}
	if len(res.Paths) != 1 || res.Paths[0] != ".transactions[0].to" {
		t.Errorf("Paths: got %v, want [.transactions[0].to]", res.Paths)
	}
	if len(res.Results) != 1 {
		t.Errorf("Results: got %d entries, want 1", len(res.Results))
	}
}

var (
	tru = Raw{0x01}
	fal = Raw{0x00}
)

func testAliases() map[string]string {
	return map[string]string{
		"five": "5",
		"ten":  "0x0a",
	}
}

func rawsEqual(a, b []Raw) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !bytes.Equal(a[i], b[i]) {
			return false
		}
	}
	return true
}

func TestParseFiltersSuccess(t *testing.T) {
	tests := []struct {
		name   string
		filter Filter
		want   []Raw
	}{
		{"scalar eq true", `5 == 5`, []Raw{tru}},
		{"scalar eq false", `5 == 6`, []Raw{fal}},
		{"scalar neq true", `5 != 6`, []Raw{tru}},
		{"scalar neq false", `5 != 5`, []Raw{fal}},
		{"scalar gt", `6 > 5`, []Raw{tru}},
		{"scalar lt", `5 < 6`, []Raw{tru}},
		{"scalar gte equal", `5 >= 5`, []Raw{tru}},
		{"scalar gte greater", `6 >= 5`, []Raw{tru}},
		{"scalar lte false", `6 <= 5`, []Raw{fal}},

		{"hex eq decimal", `0x0a == 10`, []Raw{tru}},
		{"hex eq hex-padded decimal", `0x05 == 5`, []Raw{tru}},

		{"path scalar eq", `.scalar5 == 5`, []Raw{tru}},
		{"path scalar gt", `.scalar5 > 3`, []Raw{tru}},
		{"path scalar vs path scalar", `.scalar10 > .scalar5`, []Raw{tru}},

		{"array vs scalar eq", `.arr == 2`, []Raw{fal, tru, fal}},
		{"array gt scalar", `.arr > 1`, []Raw{fal, tru, tru}},
		{"scalar-left array-right", `2 == .arr`, []Raw{fal, tru, fal}},
		{"array vs array eq elementwise", `.arr == .arr2`, []Raw{fal, fal, fal}},

		{"in scalar present", `2 in [1,2,3]`, []Raw{tru}},
		{"in scalar absent", `9 in [1,2,3]`, []Raw{fal}},
		{"in array elementwise", `.arr in [2,3]`, []Raw{fal, tru, tru}},
		{"in with mixed literal widths", `0x02 in [1,2,3]`, []Raw{tru}},

		{"logical and true", `(5 == 5) && (3 < 4)`, []Raw{tru}},
		{"logical and false", `(5 == 5) && (3 > 4)`, []Raw{fal}},
		{"logical or true", `(5 == 6) || (3 < 4)`, []Raw{tru}},
		{"logical or false", `(5 == 6) || (3 > 4)`, []Raw{fal}},
		{"precedence without parens", `5 == 5 && 3 < 4`, []Raw{tru}},
		{"logical array and scalar", `.arr && 1`, []Raw{tru, tru, tru}},
		{"logical array or array", `.arr || .arr2`, []Raw{tru, tru, tru}},

		{"not scalar true", `!(5 == 6)`, []Raw{tru}},
		{"not scalar false", `!(5 == 5)`, []Raw{fal}},
		{"not array", `!.arr`, []Raw{fal, fal, fal}},
		{"double not", `!!(5 == 5)`, []Raw{tru}},
		{"not of empty path yields empty", `!.empty`, []Raw{}},

		{"alias eq literal", `$five == 5`, []Raw{tru}},
		{"alias hex eq decimal", `$ten == 10`, []Raw{tru}},
		{"alias in list", `$five in [3,5,7]`, []Raw{tru}},
		{"alias in list element", `5 in [$five, 7]`, []Raw{tru}},
	}

	aliases := testAliases()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseFilters(tt.filter, ".root", aliases)
			if err != nil {
				t.Fatalf("parseFilters(%q) unexpected error: %v", tt.filter, err)
			}
			if !rawsEqual(got, tt.want) {
				t.Errorf("parseFilters(%q) = %v, want %v", tt.filter, got, tt.want)
			}
		})
	}
}

func TestParseFiltersError(t *testing.T) {
	tests := []struct {
		name    string
		filter  Filter
		wantSub string
	}{
		{"empty filter", ``, "empty or unrecognizable"},
		{"whitespace only", `   `, "empty or unrecognizable"},
		{"unmatched open paren", `(5 == 5`, "missing closing"},
		{"unexpected close paren", `)`, "unexpected"},
		{"empty list literal", `1 in []`, "empty list literal"},
		{"unterminated list", `[1,2`, "unterminated list literal"},
		{"bad list separator", `[1 2]`, "expected ',' or ']'"},
		{"chained comparison", `1 == 2 == 3`, "chained comparison"},
		{"leftover token", `1 2`, "unexpected token"},
		{"numeric overflow", `99999999999999999999`, "bad numeric literal"},
		{"dangling operator", `5 ==`, "unexpected end"},
		{"dangling not", `!`, "unexpected end"},
		{"unknown alias", `$nope == 5`, "unknown alias"},

		{"empty operand comparison", `.empty == 5`, "empty operand"},
		{"empty operand in", `.empty in [1]`, "empty operand"},
		{"empty operand logical", `.empty && 1`, "empty operand"},

		{"ordering between two arrays", `.arr > .arr4`, "not supported between two arrays"},
		{"comparison array length mismatch", `.arr == .arr4`, "array length mismatch"},
		{"logical array length mismatch", `.arr && .arr4`, "array length mismatch"},
	}

	aliases := testAliases()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseFilters(tt.filter, ".root", aliases)
			if err == nil {
				t.Fatalf("parseFilters(%q) = %v, want error containing %q", tt.filter, got, tt.wantSub)
			}
			if !strings.Contains(err.Error(), tt.wantSub) {
				t.Errorf("parseFilters(%q) error = %q, want substring %q", tt.filter, err.Error(), tt.wantSub)
			}
		})
	}
}

func TestParseFiltersNilAliases(t *testing.T) {
	if _, err := parseFilters(`$five == 5`, ".root", nil); err == nil {
		t.Fatal("parseFilters with nil aliases and alias reference: want error, got nil")
	}
	got, err := parseFilters(`5 == 5`, ".root", nil)
	if err != nil {
		t.Fatalf("parseFilters with nil aliases and no alias reference: unexpected error: %v", err)
	}
	if !rawsEqual(got, []Raw{tru}) {
		t.Errorf("parseFilters(`5 == 5`) = %v, want %v", got, []Raw{tru})
	}
}
