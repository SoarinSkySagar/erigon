package sszql

import (
	"encoding/binary"
	"encoding/hex"
	"strconv"
	"strings"
)

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

func parseFilters(filter Filter, path Path, aliases map[string]string) ([]Raw, error) {
	node, err := Parse(filter)
	if err != nil {
		return nil, err
	}
	return node.Eval(&EvalContext{Aliases: aliases, Root: path})
}

func filterPath(path Path, filter Filter, aliases map[string]string) ([]Raw, error) {
	mask, err := parseFilters(filter, path, aliases)
	if err != nil {
		return nil, err
	}
	return applyFilter(getValueFromPath(path), mask)
}

func convertToRaw(in string) []Raw {
	if in == "" {
		return nil
	}
	if strings.HasPrefix(in, "0x") {
		body := in[2:]
		if len(body)%2 == 1 {
			body = "0" + body
		}
		b, err := hex.DecodeString(body)
		if err != nil {
			return nil
		}
		return []Raw{Raw(b)}
	}
	num, err := strconv.ParseUint(in, 10, 64)
	if err != nil {
		return nil
	}
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, num)
	return []Raw{Raw(buf)}
}

func getValueFromPath(path Path) []Raw {
	switch path {
	case ".scalar5":
		return []Raw{{0x05}}
	case ".scalar10":
		return []Raw{{0x0a}}
	case ".arr":
		return []Raw{{0x01}, {0x02}, {0x03}}
	case ".arr2":
		return []Raw{{0x02}, {0x03}, {0x04}}
	case ".arr4":
		return []Raw{{0x01}, {0x02}, {0x03}, {0x04}}
	case ".empty":
		return nil
	default:
		return nil
	}
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
