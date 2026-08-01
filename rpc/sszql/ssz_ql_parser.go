package sszql

import (
	"encoding/binary"
	"fmt"
	"regexp"
	"strconv"
)

type Raw []byte

func parseQuery(request SSZQLRequest, version int, block_id string) SSZQLResponse {
	response := SSZQLResponse{
		Paths:    make([]Path, 0),
		Gindices: make([]Gindex, 0),
		Leaves:   make([]Leaf, 0),
		Results:  make([]Result, 0),
	}
	parseQueries(request, &response)
	parseAliases(request.Aliases, &response)
	if request.IncludeProofs {
		generateProof(&response)
	}

	return response
}

func parseQueries(req SSZQLRequest, res *SSZQLResponse) []SSZQuery {
	for i, query := range req.Queries {
		res.Paths = append(res.Paths, query.Path)
		res.Results = append(res.Results, Result("query "+strconv.Itoa(i)+" result"))
	}

	return req.Queries
}

func parseFilters(filter Filter, path Path, aliases map[string]string) Raw {
	ops := map[string]bool{
		"==": true, "!=": true, ">": true, "<": true,
		">=": true, "<=": true, "&&": true, "||": true,
	}

	var tokenPattern = regexp.MustCompile(
		`==|!=|>=|<=|&&|\|\||>|<|\.[a-zA-Z_][a-zA-Z0-9_]*|0x[a-fA-F0-9]+|[0-9]+(\.[0-9]+)?|[a-zA-Z_][a-zA-Z0-9_]*`,
	)

	tokens := tokenPattern.FindAllString(string(filter), -1)

	values := make(map[int][]Raw)

	for i, token := range tokens {
		runes := []rune(token)
		if ops[token] {
			continue
		}
		var bytes []Raw
		switch runes[0] {
		case '$':
			bytes = convertToRaw(aliases[string(runes[1:])])
		case '.':
			bytes = getValueFromPath(Path(runes[1:]))
		default:
			num, err := strconv.Atoi(string(runes))
			if err != nil {
				fmt.Println("Conversion error:", err)
				return nil
			}
			buf := make([]byte, 64)
			binary.BigEndian.PutUint64(buf, uint64(num))
			bytes = append(bytes, buf)
		}
		values[i] = bytes
	}
	var ret Raw
	return ret
}

func convertToRaw(in string) []Raw {
	var ret []Raw
	return ret
}

func getValueFromPath(path Path) []Raw {
	var str string
	ret := convertToRaw(str)
	return ret
}

func parseAliases(aliases []Alias, res *SSZQLResponse) map[string]string {
	m := make(map[string]string)

	for _, alias := range aliases {
		value := parseQueryWithPathAndFilter(alias.Path, alias.Filter)
		m[alias.Alias] = value
		res.Aliases = append(res.Aliases, AliasResponse{Alias: alias.Alias, Value: value})
	}

	return m
}

// todo: implement actual logic
func parseQueryWithPathAndFilter(path Path, filter Filter) string {
	return "dummy"
}

func generateProof(res *SSZQLResponse) []Proof {
	proofs := make([]Proof, 0, len(res.Results))
	for i := range res.Results {
		proof := Proof("proof of query" + strconv.Itoa(i))
		proofs = append(proofs, proof)
		res.Proofs = append(res.Proofs, proof)
	}
	return proofs
}
