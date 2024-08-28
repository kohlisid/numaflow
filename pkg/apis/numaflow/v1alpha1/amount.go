package v1alpha1

import (
	"strconv"
)

/**
This inspired by intstr.IntOrStr and json.Number.
*/

// Amount represents a numeric amount stored as a string to preserve precision.
// +k8s:openapi-gen=true
type Amount string

// NewAmount creates a new Amount instance from a string.
func NewAmount(s string) Amount {
	return Amount(s)
}

// NewAmountFromFloat64 creates a new Amount from a float64.
// It converts the float64 to a string to avoid precision issues.
func NewAmountFromFloat64(f float64) Amount {
	return Amount(strconv.FormatFloat(f, 'f', -1, 64))
}

// UnmarshalJSON is a custom unmarshaler for Amount, handling JSON numbers represented as strings.
func (a *Amount) UnmarshalJSON(data []byte) error {
	// Trim quotes for string JSON values.
	if len(data) > 0 && data[0] == '"' {
		data = data[1 : len(data)-1]
	}
	*a = Amount(data)
	return nil
}

// MarshalJSON converts the Amount back to JSON byte slice.
func (a Amount) MarshalJSON() ([]byte, error) {
	return []byte(`"` + a + `"`), nil
}

// Float64 converts the value of the amount to a float64.
func (a Amount) Float64() float64 {
	val, _ := strconv.ParseFloat(string(a), 64)
	return val
}

// OpenAPISchemaType specifies that in the OpenAPI specification, Amount should be treated like a string.
func (Amount) OpenAPISchemaType() []string { return []string{"string", "number"} }

// OpenAPISchemaFormat specifies the format of Amount in the OpenAPI specification.
func (Amount) OpenAPISchemaFormat() string { return "double" }

// OpenAPIV3Types is used by the kube-openapi generator when constructing the OpenAPI v3 spec of this type.
func (Amount) OpenAPIV3Types() []string { return []string{"string"} }
