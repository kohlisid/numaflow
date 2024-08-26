package v1alpha1

import (
	"encoding/binary"
	"math"
	"strconv"
)

/**
This inspired by intstr.IntOrStr and json.Number.
*/

// Amount represent a numeric amount.
type Amount struct {
	Value []byte `json:"value" protobuf:"bytes,1,opt,name=value"`
}

func NewAmount(s string) Amount {
	return Amount{Value: []byte(s)}
}

func NewAmountFromFloat64(s float64) Amount {
	bytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(bytes, math.Float64bits(s))
	return Amount{Value: bytes}
}

func (a *Amount) UnmarshalJSON(value []byte) error {
	a.Value = value
	return nil
}

func (n Amount) MarshalJSON() ([]byte, error) {
	return n.Value, nil
}

func (n Amount) OpenAPISchemaType() []string {
	return []string{"number"}
}

func (n Amount) OpenAPISchemaFormat() string { return "" }

// OpenAPIV3Types is used by the kube-openapi generator when constructing
// the OpenAPI v3 spec of this type.
func (n Amount) OpenAPIV3Types() []string { return []string{"number"} }

func (n *Amount) Float64() float64 {
	val, _ := strconv.ParseFloat(string(n.Value), 64)
	return val
}
