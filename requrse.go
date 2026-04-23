/*
Copyright © 2025 defektive

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.
*/
package main

import (
	"github.com/defektive/requrse/pkg/cmd"
)

func main() {
	cmd.Execute()
}

//func main() {
//	// 1. Raw JSON string input (e.g., from an API)
//	inputStr := `{"foo": "bar", "data": "{\"inner\":\"value\"}"}`
//	var rawInput interface{}
//	json.Unmarshal([]byte(inputStr), &rawInput)
//
//	// 2. Compile query
//	query, err := gojq.Parse(".data | fromjson")
//	if err != nil {
//		log.Fatalln(err)
//	}
//	code, err := gojq.Compile(query)
//	if err != nil {
//		log.Fatalln(err)
//	}
//
//	// 3. Run against Go object
//	iter := code.Run(rawInput)
//	for {
//		v, ok := iter.Next()
//		if !ok {
//			break
//		}
//		if err, ok := v.(error); ok {
//			log.Fatalln(err)
//		}
//		fmt.Printf("%+v\n", v) // Output: map[inner:value]
//	}
//}
