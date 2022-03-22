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
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"reflect"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/olekukonko/tablewriter"
	"github.com/urfave/cli"

	"github.com/uber/cadence/common/types"
)

const (
	formatTable = "table"
	formatJSON  = "json"
)

var tableHeaderBlue = tablewriter.Colors{tablewriter.FgHiBlueColor}

// TableOptions allows passing optional flags for altering rendered table
type TableOptions struct {
	// OptionalColumns may contain column header names which can be hidden
	OptionalColumns map[string]bool

	// Border specified whether to render table border
	Border bool

	// Color will use coloring characters while printing table
	Color bool

	// PrintRawTime will print time as int64 unix nanos
	PrintRawTime bool
	// PrintDateTime will print both date & time
	PrintDateTime bool
}

// Render is an entry point for presentation layer. It uses --format flat to determine output format.
func Render(c *cli.Context, data interface{}, opts TableOptions) (err error) {
	defer func() {
		if err != nil {
			ErrorAndExit("failed to render", err)
		}
	}()

	// For now always output to stdout
	w := os.Stdout

	switch format := c.String(FlagFormat); format {
	case formatTable, "":
		return RenderTable(w, data, opts)
	case formatJSON:
		return RenderJSON(w, data)
	default:
		return RenderTemplate(w, data, format)
	}
}

// RenderTemplate uses golang text/template format to render data with user provided template
func RenderTemplate(w io.Writer, data interface{}, formatTemplate string) error {
	t, err := template.New("").Parse(formatTemplate + "\n")
	if err != nil {
		return fmt.Errorf("invalid template %q: %w", formatTemplate, err)
	}

	dataValue := reflect.ValueOf(data)
	switch dataValue.Kind() {
	case reflect.Struct:
		return t.Execute(w, data)
	case reflect.Slice:
		for i := 0; i < dataValue.Len(); i++ {
			if err := t.Execute(w, dataValue.Index(i).Interface()); err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("value must be a struct or a slice, provided: %s", dataValue.Kind())

	}

	return nil
}

// RenderJSON renders given value in JSON format
func RenderJSON(w io.Writer, data interface{}) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}

// RenderTable is generic function for rendering a slice of structs as a table
func RenderTable(w io.Writer, slice interface{}, opts TableOptions) error {
	sliceValue := reflect.ValueOf(slice)
	if sliceValue.Kind() != reflect.Slice {
		return fmt.Errorf("table must be a slice, provided: %s", sliceValue.Kind())
	}

	// No elements - nothing to render
	if sliceValue.Len() == 0 {
		return nil
	}

	firstElem := sliceValue.Index(0)
	if firstElem.Kind() != reflect.Struct {
		return fmt.Errorf("table slice element must be a struct, provided: %s", firstElem.Kind())
	}

	table := tablewriter.NewWriter(w)
	table.SetBorder(opts.Border)
	table.SetColumnSeparator("|")
	table.SetHeaderLine(opts.Border)

	for r := 0; r < sliceValue.Len(); r++ {
		var row []string
		var headers []string
		var colors []tablewriter.Colors

		elem := sliceValue.Index(r)
		for f := 0; f < elem.NumField(); f++ {
			tag := elem.Type().Field(f).Tag

			header := columnHeader(tag, opts)
			if header == "" {
				continue
			}
			if r == 0 {
				headers = append(headers, header)
				colors = append(colors, tableHeaderBlue)
			}

			row = append(row, formatValue(elem.Field(f).Interface(), opts, tag))
		}
		if r == 0 {
			table.SetHeader(headers)
			if opts.Color {
				table.SetHeaderColor(colors...)
			}
		}

		table.Append(row)
	}

	table.Render()

	return nil
}

func columnHeader(tag reflect.StructTag, opts TableOptions) string {
	header, ok := tag.Lookup("header")
	if !ok {
		// No header tag - do not display
		return ""
	}

	if opts.OptionalColumns == nil {
		// No optional columns defined - display
		return header
	}

	include, optional := opts.OptionalColumns[header]
	if !optional {
		// Display if it is non-optional
		return header
	}

	if include {
		// Display if it is optional but included
		return header
	}

	// Do not display optional and excluded
	return ""
}

func formatValue(value interface{}, opts TableOptions, tag reflect.StructTag) string {
	switch v := value.(type) {
	case time.Time:
		return formatTime(v, opts)
	case string:
		return formatString(v, tag)
	case *types.Memo:
		return formatMemo(v)
	case *types.SearchAttributes:
		return formatSearchAttributes(v)
	default:
		return fmt.Sprintf("%v", v)
	}
}

func formatTime(t time.Time, opts TableOptions) string {
	if opts.PrintRawTime {
		return strconv.FormatInt(t.Unix(), 10)
	}
	if opts.PrintDateTime {
		return t.Format(defaultDateTimeFormat)
	}
	return t.Format(defaultTimeFormat)
}

func formatMemo(memo *types.Memo) string {
	if memo == nil {
		return ""
	}
	buf := new(bytes.Buffer)
	for k, v := range memo.Fields {
		fmt.Fprintf(buf, "%s=%s\n", k, string(v))
	}
	return strings.TrimRight(buf.String(), "\n")
}

func formatSearchAttributes(searchAttr *types.SearchAttributes) string {
	if searchAttr == nil {
		return ""
	}
	buf := new(bytes.Buffer)
	for k, v := range searchAttr.IndexedFields {
		var decodedVal interface{}
		json.Unmarshal(v, &decodedVal)
		fmt.Fprintf(buf, "%s=%v\n", k, decodedVal)
	}
	return strings.TrimRight(buf.String(), "\n")
}

func formatString(str string, tag reflect.StructTag) string {
	if maxLengthStr, ok := tag.Lookup("maxLength"); ok {
		maxLength, _ := strconv.ParseInt(maxLengthStr, 10, 64)
		str = trimString(str, int(maxLength))
	}

	return str
}

func trimString(str string, maxLength int) string {
	if len(str) < maxLength {
		return str
	}

	items := strings.Split(str, "/")
	lastItem := items[len(items)-1]
	if len(str) < maxLength {
		return ".../" + lastItem
	}

	return "..." + lastItem[len(lastItem)-maxLength:]
}
