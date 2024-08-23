package v1alpha1

import (
	"encoding/binary"
	"encoding/json"
	"math"
	"testing"
)

func TestFromStringBinary(t *testing.T) {
	val := "test string"
	result := FromString(val)
	if result.Type != String || result.StrVal != val {
		t.Errorf("FromString() test failed, expected %v got %v", val, result.StrVal)
	}
}

func TestFromFloat64Binary(t *testing.T) {
	var val float64 = 123.456
	result := FromFloat64(val)
	decodedFloat := math.Float64frombits(binary.LittleEndian.Uint64(result.Float64Val))
	if result.Type != Float64 || decodedFloat != val {
		t.Errorf("FromFloat64() test failed, expected %f got %f", val, decodedFloat)
	}
}

func TestParseStringBinary(t *testing.T) {
	val := "test parse"
	result := Parse(val)
	if result.Type != String || result.StrVal != val {
		t.Errorf("Parse() string test failed, expected %v got %v", val, result.StrVal)
	}
}

func TestParseFloat64Binary(t *testing.T) {
	val := "123.456"
	expectedVal := 123.456
	result := Parse(val)
	decodedFloat := math.Float64frombits(binary.LittleEndian.Uint64(result.Float64Val))
	if result.Type != Float64 || decodedFloat != expectedVal {
		t.Errorf("Parse() float64 test failed, expected %f got %f", expectedVal, decodedFloat)
	}
}

func TestMarshalUnmarshalJSONBinary(t *testing.T) {
	original := FromFloat64(123.456)
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	var unmarshaled Float64OrString
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	decodedFloat := math.Float64frombits(binary.LittleEndian.Uint64(unmarshaled.Float64Val))
	originalFloat := math.Float64frombits(binary.LittleEndian.Uint64(original.Float64Val))
	if decodedFloat != originalFloat {
		t.Errorf("Unmarshal mismatch: expected %v got %v", originalFloat, decodedFloat)
	}
}

func TestFloat64ValueFromStringBinary(t *testing.T) {
	valStr := "123.456"
	expectedFloat64 := 123.456
	float64OrString := FromString(valStr)
	float64Val := float64OrString.Float64Value()
	if float64Val != expectedFloat64 {
		t.Errorf("Float64Value() from string failed, expected %f got %f", expectedFloat64, float64Val)
	}
}

func TestFloat64ValueFromFloat64Binary(t *testing.T) {
	valFloat64 := 123.456
	float64OrString := FromFloat64(valFloat64)
	float64Val := float64OrString.Float64Value()
	if float64Val != valFloat64 {
		t.Errorf("Float64Value() from float64 binary failed, expected %f got %f", valFloat64, float64Val)
	}
}
