// Copyright (c) 2022 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package cli

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/uber/cadence/common/types"
)

func Test_RenderTable(t *testing.T) {
	builder := &strings.Builder{}
	RenderTable(builder, testTable, TableOptions{})
	assert.Equal(t, ""+
		"        STRING        | INTEGER | BOOL  |   TIME   | MEMO | SEARCH ATTRIBUTES  \n"+
		"  text                |     123 | true  | 03:04:05 | A=AA | X=XX               \n"+
		"  ...g long long long |     456 | false | 13:14:15 |      |                    \n",
		builder.String())

	builder = &strings.Builder{}
	RenderTable(builder, testTable, TableOptions{OptionalColumns: map[string]bool{"memo": true, "search attributes": false}, PrintDateTime: true})
	assert.Equal(t, ""+
		"        STRING        | INTEGER | BOOL  |         TIME         | MEMO  \n"+
		"  text                |     123 | true  | 2000-01-02T03:04:05Z | A=AA  \n"+
		"  ...g long long long |     456 | false | 2000-11-12T13:14:15Z |       \n",
		builder.String())

	assert.EqualError(t, RenderTable(nil, 123, TableOptions{}), "table must be a slice, provided: int")
	assert.EqualError(t, RenderTable(nil, []*testRow{{}}, TableOptions{}), "table slice element must be a struct, provided: ptr")
}

func Test_RenderTemplate(t *testing.T) {
	builder := &strings.Builder{}
	RenderTemplate(builder, testTable, "{{.StringField}}")
	assert.Equal(t, ""+
		"text\n"+
		"long long long long long long\n",
		builder.String())

	builder = &strings.Builder{}
	RenderTemplate(builder, testTable[0], "{{.StringField}}")
	assert.Equal(t, "text\n", builder.String())

	assert.EqualError(t, RenderTemplate(nil, testTable, "{{invalid}}"), "invalid template \"{{invalid}}\": template: :1: function \"invalid\" not defined")
	assert.EqualError(t, RenderTemplate(nil, 123, ""), "value must be a struct or a slice, provided: int")
}

type testRow struct {
	StringField  string                  `header:"string" maxLength:"16"`
	IntField     int                     `header:"integer"`
	BoolField    bool                    `header:"bool"`
	TimeField    time.Time               `header:"time"`
	MemoField    *types.Memo             `header:"memo"`
	SAField      *types.SearchAttributes `header:"search attributes"`
	IgnoredField int
}

var testTable = []testRow{
	{
		StringField: "text",
		IntField:    123,
		BoolField:   true,
		TimeField:   time.Date(2000, 1, 2, 3, 4, 5, 6, time.UTC),
		MemoField:   &types.Memo{Fields: map[string][]byte{"A": []byte("AA")}},
		SAField:     &types.SearchAttributes{IndexedFields: map[string][]byte{"X": []byte("\"XX\"")}},
	},
	{
		StringField: "long long long long long long",
		IntField:    456,
		BoolField:   false,
		TimeField:   time.Date(2000, 11, 12, 13, 14, 15, 16, time.Local),
		MemoField:   nil,
		SAField:     nil,
	},
}
