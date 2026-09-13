package sszql

import (
	"errors"
	"fmt"
	"strconv"
)

func parseQueryV1(request SSZQLRequest, version uint, block Block) (SSZQLResponse, error) {
	response := SSZQLResponse{
		Paths:    make([]Path, 0),
		Gindices: make([]Gindex, 0),
		Leaves:   make([]Leaf, 0),
		Results:  make([]Result, 0),
	}
	emptyRes := response
	aliases, err := parseAliases(request.Aliases, &response, block)
	if err != nil {
		return emptyRes, err
	}
	err = parseQueries(request, &response, block, aliases)
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

func parseQueries(req SSZQLRequest, res *SSZQLResponse, block Block, aliases map[string]string) error {
	for _, query := range req.Queries {
		resolvedPath, err := resolvePath(query.Path, query.Anchor, block)
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

func parseAliases(aliases []Alias, res *SSZQLResponse, block Block) (map[string]string, error) {
	m := make(map[string]string)

	for _, alias := range aliases {
		if _, dup := m[alias.Alias]; dup {
			return nil, fmt.Errorf("%w: %q", errors.New("duplicate alias"), alias.Alias)
		}

		resolvedPath, err := resolvePath(alias.Path, alias.Anchor, block)
		if err != nil {
			return nil, err
		}
		m[alias.Alias] = string(resolvedPath.Value)
		res.Aliases = append(res.Aliases, AliasResponse{Alias: alias.Alias, Value: string(resolvedPath.Value)})
	}

	return m, nil
}

func resolvePath(path Path, anchor Anchor, block Block) (ResolvedPath, error) {
	if path == "/parent_hash" && block.execution != nil {
		parentHash := block.execution.ParentHash().Hex()
		return ResolvedPath{
			Gindex: Gindex(4),
			Leaf:   Leaf(parentHash),
			Value:  Result(parentHash),
		}, nil
	}

	response := ResolvedPath{
		Gindex: Gindex(99),
		Leaf:   Leaf("0xabcdef"),
		Value:  Result("0xabcdef"),
	}
	return response, nil
}

// func resolveConsensusPath(path Path, anchor Anchor, blockID beaconhttp.SegmentID) (ResolvedPath, error) {
// 	response := ResolvedPath{
// 		Gindex: Gindex(99),
// 		Leaf:   Leaf("0xabcdef"),
// 		Value:  Result("0xabcdef"),
// 	}
// 	return response, nil
// }

func generateProof(res *SSZQLResponse) error {
	proofs := make([]Proof, 0, len(res.Results))
	for i := range res.Results {
		proof := Proof("proof of query" + strconv.Itoa(i))
		proofs = append(proofs, proof)
		res.Proofs = append(res.Proofs, proof)
	}
	return nil
}
