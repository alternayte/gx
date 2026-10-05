package main

import "testing"

// TestMergeResults covers the evidence of a timing package: its result
// comes from the solo run, a full run cannot replace it, and a failure is
// never replaced by a pass.
func TestMergeResults(t *testing.T) {
	solo := []byte(`{"Action":"pass","Package":"example.com/dev","Test":"TestTiming"}
{"Action":"pass","Package":"example.com/dev","Test":"TestTiming/sub"}
`)
	full := []byte(`{"Action":"fail","Package":"example.com/dev","Test":"TestTiming"}
{"Action":"pass","Package":"example.com/other","Test":"TestOther"}
{"Action":"fail","Package":"example.com/other","Test":"TestBroken"}
{"Action":"pass","Package":"example.com/more","Test":"TestBroken"}
{"Action":"output","Package":"example.com/other","Output":"text"}
`)
	results := map[string]string{}
	if err := mergeResults(results, solo, nil); err != nil {
		t.Fatal(err)
	}
	if err := mergeResults(results, full, map[string]bool{"example.com/dev": true}); err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"TestTiming": "pass", "TestOther": "pass", "TestBroken": "fail"}
	if len(results) != len(want) {
		t.Fatalf("results = %v, want %v", results, want)
	}
	for name, status := range want {
		if results[name] != status {
			t.Fatalf("%s = %q, want %q (%v)", name, results[name], status, results)
		}
	}
	if err := mergeResults(results, []byte("not json"), nil); err == nil {
		t.Fatal("a bad stream gave no error")
	}
}
