package sszql

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"

	"github.com/erigontech/erigon/cl/beacon/beaconhttp"
	"github.com/erigontech/erigon/rpc"
)

var executionBlockIDPattern = regexp.MustCompile(`^(?:latest|earliest|safe|finalized|pending|0x[0-9a-fA-F]{64}|0|[1-9][0-9]*)$`)
var consensusBlockIDPattern = regexp.MustCompile(`^(?:head|genesis|finalized|0x[0-9a-fA-F]{64}|0|[1-9][0-9]*)$`)
var errInvalidBlockID = errors.New("invalid block_id")
var errInvalidLayer = errors.New("invalid layer")

func parseQueryV1(request SSZQLRequest, version uint, block BlockRef) (SSZQLResponse, error) {
	response := SSZQLResponse{
		Paths:    make([]Path, 0),
		Gindices: make([]Gindex, 0),
		Leaves:   make([]Leaf, 0),
		Results:  make([]Result, 0),
	}
	emptyRes := response
	var temp rpc.BlockNumberOrHash
	aliases, err := parseAliases(request.Aliases, &response, temp)
	if err != nil {
		return emptyRes, err
	}
	err = parseQueries(request, &response, temp, aliases)
	if err != nil {
		return emptyRes, err
	}
	if request.IncludeProofs {
		err = generateProof(&response)
		if err != nil {
			return emptyRes, err
		}
	}

	return response, nil
}

func parseQueries(req SSZQLRequest, res *SSZQLResponse, blockID rpc.BlockNumberOrHash, aliases map[string]string) error {
	for _, query := range req.Queries {
		resolvedPath, err := resolveExecutionPath(query.Path, query.Anchor, blockID)
		if err != nil {
			return err
		}
		res.Paths = append(res.Paths, query.Path)
		res.Results = append(res.Results, resolvedPath.Value)
		res.Gindices = append(res.Gindices, resolvedPath.Gindex)
		res.Leaves = append(res.Leaves, resolvedPath.Leaf)
	}

	return nil
}

func parseAliases(aliases []Alias, res *SSZQLResponse, blockID rpc.BlockNumberOrHash) (map[string]string, error) {
	m := make(map[string]string)

	for _, alias := range aliases {
		if _, dup := m[alias.Alias]; dup {
			return nil, fmt.Errorf("%w: %q", errors.New("duplicate alias"), alias.Alias)
		}

		resolvedPath, err := resolveExecutionPath(alias.Path, alias.Anchor, blockID)
		if err != nil {
			return nil, err
		}
		m[alias.Alias] = string(resolvedPath.Value)
		res.Aliases = append(res.Aliases, AliasResponse{Alias: alias.Alias, Value: string(resolvedPath.Value)})
	}

	return m, nil
}

func resolveExecutionPath(path Path, anchor Anchor, blockID rpc.BlockNumberOrHash) (ResolvedPath, error) {
	response := ResolvedPath{
		Gindex: Gindex(99),
		Leaf:   Leaf("0xabcdef"),
		Value:  Result("0xabcdef"),
	}
	return response, nil
}

func resolveConsensusPath(path Path, anchor Anchor, blockID beaconhttp.SegmentID) (ResolvedPath, error) {
	response := ResolvedPath{
		Gindex: Gindex(99),
		Leaf:   Leaf("0xabcdef"),
		Value:  Result("0xabcdef"),
	}
	return response, nil
}

func generateProof(res *SSZQLResponse) error {
	proofs := make([]Proof, 0, len(res.Results))
	for i := range res.Results {
		proof := Proof("proof of query" + strconv.Itoa(i))
		proofs = append(proofs, proof)
		res.Proofs = append(res.Proofs, proof)
	}
	return nil
}
