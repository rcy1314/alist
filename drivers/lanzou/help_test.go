package lanzou

import "testing"

func TestFindJSVarFunc(t *testing.T) {
	tests := []struct {
		name string
		key  string
		data string
		want string
	}{
		{name: "single quoted", key: "sign", data: `var sign = 'complete-sign';`, want: "complete-sign"},
		{name: "double quoted", key: "sign", data: `var sign = "complete-sign";`, want: "complete-sign"},
		{name: "unquoted", key: "kd", data: `var kd = 1;`, want: "1"},
		{name: "last non-empty declaration", key: "isngis", data: `var isngis = ''; var isngis = 'real-sign';`, want: "real-sign"},
		{name: "sasign middle declaration", key: "sasign", data: `var sasign='first'; var sasign='middle'; var sasign='last';`, want: "middle"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := findJSVarFunc(tt.key, tt.data); got != tt.want {
				t.Fatalf("findJSVarFunc() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestHTMLJSONToMapUsesCompleteLastVariable(t *testing.T) {
	data := `var isngis = ''; var isngis = 'complete-sign'; data: { 'action':'downprocess','sign':isngis,'kd':1,'p':pwd }`
	params, err := htmlJsonToMap(data)
	if err != nil {
		t.Fatal(err)
	}
	if params["sign"] != "complete-sign" {
		t.Fatalf("sign = %q, want %q", params["sign"], "complete-sign")
	}
}
