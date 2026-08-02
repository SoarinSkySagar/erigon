package sszql

type Raw []byte

type Path string

type Filter string

type Anchor string

type Gindex int64

type Proof string

type Leaf string

type Result string

type Alias struct {
	Path   Path   `json:"path"`
	Filter Filter `json:"filter,omitempty"`
	Alias  string `json:"alias"`
}

type SSZQuery struct {
	Anchor    Anchor `json:"anchor"`
	Path      Path   `json:"path"`
	Filter    Filter `json:"filter,omitempty"`
	Summaries bool   `json:"summaries,omitempty"`
}

type SSZQLRequest struct {
	Aliases       []Alias    `json:"aliases,omitempty"`
	Queries       []SSZQuery `json:"queries"`
	IncludeProofs bool       `json:"include_proof,omitempty"`
	Multiproof    bool       `json:"multiproof,omitempty"`
}

type AliasResponse struct {
	Alias string `json:"alias"`
	Value string `json:"value"`
}

type SSZQLResponse struct {
	Aliases  []AliasResponse `json:"aliases,omitempty"`
	Paths    []Path          `json:"paths"`
	Gindices []Gindex        `json:"gindices"`
	Leaves   []Leaf          `json:"leaves"`
	Results  []Result        `json:"results"`
	Proofs   []Proof         `json:"proofs,omitempty"`
}

type Node interface {
	Eval(ctx *EvalContext) ([]Raw, error)
}

type EvalContext struct {
	Aliases map[string]string
	Root    Path
}

type BinaryNode struct {
	Op          string
	Left, Right Node
}

type NotNode struct{ Operand Node }

type ListNode struct{ Elems []Node }

type LiteralNode struct{ Value Raw }

type PathNode struct{ Path Path }

type AliasNode struct{ Name string }

type Parser struct {
	tokens []string
	pos    int
}
