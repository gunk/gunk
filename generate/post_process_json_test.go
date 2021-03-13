package generate

import (
	"testing"
)

func TestJSONTagPostProcessor(t *testing.T) {
	tests := []struct {
		input  string
		output string
	}{
		{
			input: `package test
// Person represents a homo sapien instance.
type Person struct {
	FirstName            string   ` + "`" + `protobuf:"bytes,1,opt,name=FirstName,json=first_name,proto3" json:"FirstName,omitempty"` + "`" + `
	LastName             string   ` + "`" + `protobuf:"bytes,2,opt,name=LastName,json=last_name,proto3" json:"LastName,omitempty"` + "`" + `
	XXX_NoUnkeyedLiteral struct{} ` + "`" + `json:"-"` + "`" + `
	XXX_unrecognized     []byte   ` + "`" + `json:"-"` + "`" + `
	XXX_sizecache        int32    ` + "`" + `json:"-"` + "`" + `
}
`,
			output: `package test
// Person represents a homo sapien instance.
type Person struct {
	FirstName            string   ` + "`" + `protobuf:"bytes,1,opt,name=FirstName,json=first_name,proto3" json:"first_name,omitempty"` + "`" + `
	LastName             string   ` + "`" + `protobuf:"bytes,2,opt,name=LastName,json=last_name,proto3" json:"last_name,omitempty"` + "`" + `
	XXX_NoUnkeyedLiteral struct{} ` + "`" + `json:"-"` + "`" + `
	XXX_unrecognized     []byte   ` + "`" + `json:"-"` + "`" + `
	XXX_sizecache        int32    ` + "`" + `json:"-"` + "`" + `
}
`,
		},
		{
			input: `// This file intentionally has many json strings to confuse the post processor
package test
type ABCJSONTest struct {
	// JSONField is a json field
	JSONField string ` + "`" + `json2:"abc"` + "`" + `
}
`,
			output: `// This file intentionally has many json strings to confuse the post processor
package test
type ABCJSONTest struct {
	// JSONField is a json field
	JSONField string ` + "`" + `json2:"abc"` + "`" + `
}
`,
		},
	}
	for _, tc := range tests {
		output, err := jsonTagPostProcessor([]byte(tc.input))
		if err != nil {
			t.Fatalf("failed to postproc json struct tag: err=%s", err.Error())
		}
		if string(output) != tc.output {
			t.Errorf("wrong JSON tag post process result: expected=%q actual=%q", tc.output, string(output))
		}
	}
}
