package v1alpha1

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
)

type Float64OrString struct {
	Type       Type   `json:"type" protobuf:"varint,1,opt,name=type,casttype=Type"`
	Float64Val []byte `json:"float64Val,omitempty" protobuf:"bytes,2,opt,name=float64Val"`
	StrVal     string `json:"strVal,omitempty" protobuf:"bytes,3,opt,name=strVal"`
}

// Type represents the stored type of Float64OrString.
type Type int64

const (
	Float64 Type = iota // The Float64OrString holds a float64 value in byte slice.
	String              // The Float64OrString holds a string.
)

// FromString creates a Float64OrString object with a string value.
func FromString(val string) Float64OrString {
	return Float64OrString{Type: String, StrVal: val}
}

// FromFloat64 creates a Float64OrString object with a float64 value using byte slice.
func FromFloat64(val float64) Float64OrString {
	bytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(bytes, math.Float64bits(val))
	return Float64OrString{Type: Float64, Float64Val: bytes}
}

// Parse the given string and try to convert it to a float64 before
// setting it as a byte slice value.
func Parse(val string) Float64OrString {
	f, err := strconv.ParseFloat(val, 64)
	if err != nil {
		return FromString(val)
	}
	return FromFloat64(f)
}

// UnmarshalJSON implements the json.Unmarshaller interface.
func (float64str *Float64OrString) UnmarshalJSON(value []byte) error {
	if value[0] == '"' {
		float64str.Type = String
		return json.Unmarshal(value, &float64str.StrVal)
	}
	float64str.Type = Float64
	return json.Unmarshal(value, &float64str.Float64Val)
}

// Float64Value returns the float64 value by converting from byte slice if type Float64,
// or if it is a String, will attempt a conversion to float64,
// returning 0 if a parsing error occurs.
func (float64str *Float64OrString) Float64Value() float64 {
	if float64str.Type == String {
		f, _ := strconv.ParseFloat(float64str.StrVal, 64)
		return f
	}
	float, err := strconv.ParseFloat(string(float64str.Float64Val), 64)
	if err != nil {
		return 0
	}
	return float
}

// MarshalJSON implements the json.Marshaller interface.
func (float64str Float64OrString) MarshalJSON() ([]byte, error) {
	switch float64str.Type {
	case Float64:
		return json.Marshal(float64str.Float64Val)
	case String:
		return json.Marshal(float64str.StrVal)
	default:
		return []byte{}, fmt.Errorf("impossible Float64OrString.Type")
	}
}

// OpenAPISchemaType is used by the kube-openapi generator when constructing
// the OpenAPI spec of this type.
func (Float64OrString) OpenAPISchemaType() []string { return []string{"string"} }

// OpenAPISchemaFormat is used by the kube-openapi generator when constructing
// the OpenAPI spec of this type.
func (Float64OrString) OpenAPISchemaFormat() string { return "float64-or-string" }
