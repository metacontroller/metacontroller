/*
Copyright 2018 Google Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    https://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"encoding/json"
	"fmt"
	"strconv"

	jsonnet "github.com/google/go-jsonnet"
	"github.com/google/go-jsonnet/ast"
)

var extensions = []*jsonnet.NativeFunction{
	// jsonUnmarshal adds a native function for unmarshaling JSON,
	// since there doesn't seem to be one in the standard library.
	{
		Name:   "jsonUnmarshal",
		Params: ast.Identifiers{"jsonStr"},
		Func: func(args []interface{}) (interface{}, error) {
			jsonStr, ok := args[0].(string)
			if !ok {
				return nil, fmt.Errorf("unexpected type %T for 'jsonStr' arg", args[0])
			}
			val := make(map[string]interface{})
			if err := json.Unmarshal([]byte(jsonStr), &val); err != nil {
				return nil, fmt.Errorf("can't unmarshal JSON: %v", err)
			}
			return val, nil
		},
	},

	// parseInt adds a native function for parsing non-decimal integers,
	// since there doesn't seem to be one in the standard library.
	{
		Name:   "parseInt",
		Params: ast.Identifiers{"intStr", "base"},
		Func: func(args []interface{}) (interface{}, error) {
			str, ok := args[0].(string)
			if !ok {
				return nil, fmt.Errorf("unexpected type %T for 'intStr' arg", args[0])
			}
			base, ok := args[1].(float64)
			if !ok {
				return nil, fmt.Errorf("unexpected type %T for 'base' arg", args[1])
			}
			intVal, err := strconv.ParseInt(str, int(base), 64)
			if err != nil {
				return nil, fmt.Errorf("can't parse 'intStr': %v", err)
			}
			return float64(intVal), nil
		},
	},
}
