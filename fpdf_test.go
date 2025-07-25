/*
 * Copyright (c) 2013-2015 Kurt Jung (Gmail: kurt.w.jung)
 *
 * Permission to use, copy, modify, and distribute this software for any
 * purpose with or without fee is hereby granted, provided that the above
 * copyright notice and this permission notice appear in all copies.
 *
 * THE SOFTWARE IS PROVIDED "AS IS" AND THE AUTHOR DISCLAIMS ALL WARRANTIES
 * WITH REGARD TO THIS SOFTWARE INCLUDING ALL IMPLIED WARRANTIES OF
 * MERCHANTABILITY AND FITNESS. IN NO EVENT SHALL THE AUTHOR BE LIABLE FOR
 * ANY SPECIAL, DIRECT, INDIRECT, OR CONSEQUENTIAL DAMAGES OR ANY DAMAGES
 * WHATSOEVER RESULTING FROM LOSS OF USE, DATA OR PROFITS, WHETHER IN AN
 * ACTION OF CONTRACT, NEGLIGENCE OR OTHER TORTIOUS ACTION, ARISING OUT OF
 * OR IN CONNECTION WITH THE USE OR PERFORMANCE OF THIS SOFTWARE.
 */

package gofpdf_test

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/airb3/gofpdf"
	"github.com/airb3/gofpdf/internal/example"
	"github.com/airb3/gofpdf/internal/files"
)

func init() {
	cleanup()
}

func cleanup() {
	filepath.Walk(example.PdfDir(),
		func(path string, info os.FileInfo, err error) (reterr error) {
			if info.Mode().IsRegular() {
				dir, _ := filepath.Split(path)
				if "reference" != filepath.Base(dir) {
					if len(path) > 3 {
						if path[len(path)-4:] == ".pdf" {
							os.Remove(path)
						}
					}
				}
			}
			return
		})
}

func TestFpdfImplementPdf(t *testing.T) {
	// this will not compile if Fpdf and Tpl
	// do not implement Pdf
	var _ gofpdf.Pdf = (*gofpdf.Fpdf)(nil)
	var _ gofpdf.Pdf = (*gofpdf.Tpl)(nil)
}

// TestPagedTemplate ensures new paged templates work
func TestPagedTemplate(t *testing.T) {
	pdf := gofpdf.New("P", "mm", "A4", "")
	tpl := pdf.CreateTemplate(func(t *gofpdf.Tpl) {
		// this will be the second page, as a page is already
		// created by default
		t.AddPage()
		t.AddPage()
		t.AddPage()
	})

	if tpl.NumPages() != 4 {
		t.Fatalf("The template does not have the correct number of pages %d", tpl.NumPages())
	}

	tplPages := tpl.FromPages()
	for x := 0; x < len(tplPages); x++ {
		pdf.AddPage()
		pdf.UseTemplate(tplPages[x])
	}

	// get the last template
	tpl2, err := tpl.FromPage(tpl.NumPages())
	if err != nil {
		t.Fatal(err)
	}

	// the objects should be the exact same, as the
	// template will represent the last page by default
	// therefore no new id should be set, and the object
	// should be the same object
	if fmt.Sprintf("%p", tpl2) != fmt.Sprintf("%p", tpl) {
		t.Fatal("Template no longer respecting initial template object")
	}
}

// TestIssue0116 addresses issue 116 in which library silently fails after
// calling CellFormat when no font has been set.
func TestIssue0116(t *testing.T) {
	var pdf *gofpdf.Fpdf

	pdf = gofpdf.New("P", "mm", "A4", "")
	pdf.AddPage()
	pdf.SetFont("Arial", "B", 16)
	pdf.Cell(40, 10, "OK")
	if pdf.Error() != nil {
		t.Fatalf("not expecting error when rendering text")
	}

	pdf = gofpdf.New("P", "mm", "A4", "")
	pdf.AddPage()
	pdf.Cell(40, 10, "Not OK") // Font not set
	if pdf.Error() == nil {
		t.Fatalf("expecting error when rendering text without having set font")
	}
}

// TestIssue0193 addresses issue 193 in which the error io.EOF is incorrectly
// assigned to the FPDF instance error.
func TestIssue0193(t *testing.T) {
	var png []byte
	var pdf *gofpdf.Fpdf
	var err error
	var rdr *bytes.Reader

	png, err = ioutil.ReadFile(example.ImageFile("sweden.png"))
	if err == nil {
		rdr = bytes.NewReader(png)
		pdf = gofpdf.New("P", "mm", "A4", "")
		pdf.AddPage()
		_ = pdf.RegisterImageOptionsReader("sweden", gofpdf.ImageOptions{ImageType: "png", ReadDpi: true}, rdr)
		err = pdf.Error()
	}
	if err != nil {
		t.Fatalf("issue 193 error: %s", err)
	}

}

// TestIssue0209SplitLinesEqualMultiCell addresses issue 209
// make SplitLines and MultiCell split at the same place
func TestIssue0209SplitLinesEqualMultiCell(t *testing.T) {
	var pdf *gofpdf.Fpdf

	pdf = gofpdf.New("P", "mm", "A4", "")
	pdf.AddPage()
	pdf.SetFont("Arial", "", 8)
	// this sentence should not be splited
	str := "Guochin Amandine"
	lines := pdf.SplitLines([]byte(str), 26)
	_, FontSize := pdf.GetFontSize()
	y_start := pdf.GetY()
	pdf.MultiCell(26, FontSize, str, "", "L", false)
	y_end := pdf.GetY()

	if len(lines) != 1 {
		t.Fatalf("expect SplitLines split in one line")
	}
	if int(y_end-y_start) != int(FontSize) {
		t.Fatalf("expect MultiCell split in one line %.2f != %.2f", y_end-y_start, FontSize)
	}

	// this sentence should be splited in two lines
	str = "Guiochini Amandine"
	lines = pdf.SplitLines([]byte(str), 26)
	y_start = pdf.GetY()
	pdf.MultiCell(26, FontSize, str, "", "L", false)
	y_end = pdf.GetY()

	if len(lines) != 2 {
		t.Fatalf("expect SplitLines split in two lines")
	}
	if int(y_end-y_start) != int(FontSize*2) {
		t.Fatalf("expect MultiCell split in two lines %.2f != %.2f", y_end-y_start, FontSize*2)
	}
}

// TestFooterFuncLpi tests to make sure the footer is not call twice and SetFooterFuncLpi can work
// without SetFooterFunc.
func TestFooterFuncLpi(t *testing.T) {
	pdf := gofpdf.New("P", "mm", "A4", "")
	var (
		oldFooterFnc  = "oldFooterFnc"
		bothPages     = "bothPages"
		firstPageOnly = "firstPageOnly"
		lastPageOnly  = "lastPageOnly"
	)

	// This set just for testing, only set SetFooterFuncLpi.
	pdf.SetFooterFunc(func() {
		pdf.SetY(-15)
		pdf.SetFont("Arial", "I", 8)
		pdf.CellFormat(0, 10, oldFooterFnc,
			"", 0, "C", false, 0, "")
	})
	pdf.SetFooterFuncLpi(func(lastPage bool) {
		pdf.SetY(-15)
		pdf.SetFont("Arial", "I", 8)
		pdf.CellFormat(0, 10, bothPages, "", 0, "L", false, 0, "")
		if !lastPage {
			pdf.CellFormat(0, 10, firstPageOnly, "", 0, "C", false, 0, "")
		} else {
			pdf.CellFormat(0, 10, lastPageOnly, "", 0, "C", false, 0, "")
		}
	})
	pdf.AddPage()
	pdf.SetFont("Arial", "B", 16)
	for j := 1; j <= 40; j++ {
		pdf.CellFormat(0, 10, fmt.Sprintf("Printing line number %d", j),
			"", 1, "", false, 0, "")
	}
	if pdf.Error() != nil {
		t.Fatalf("not expecting error when rendering text")
	}
	w := &bytes.Buffer{}
	if err := pdf.Output(w); err != nil {
		t.Errorf("unexpected err: %s", err)
	}
	b := w.Bytes()
	if bytes.Contains(b, []byte(oldFooterFnc)) {
		t.Errorf("not expecting %s render on pdf when FooterFncLpi is set", oldFooterFnc)
	}
	got := bytes.Count(b, []byte("bothPages"))
	if got != 2 {
		t.Errorf("footer %s should render on two page got:%d", bothPages, got)
	}
	got = bytes.Count(b, []byte(firstPageOnly))
	if got != 1 {
		t.Errorf("footer %s should render only on first page got: %d", firstPageOnly, got)
	}
	got = bytes.Count(b, []byte(lastPageOnly))
	if got != 1 {
		t.Errorf("footer %s should render only on first page got: %d", lastPageOnly, got)
	}
	f := bytes.Index(b, []byte(firstPageOnly))
	l := bytes.Index(b, []byte(lastPageOnly))
	if f > l {
		t.Errorf("index %d (%s) should less than index %d (%s)", f, firstPageOnly, l, lastPageOnly)
	}
}

type fontResourceType struct {
}

func (f fontResourceType) Open(name string) (rdr io.Reader, err error) {
	var buf []byte
	buf, err = ioutil.ReadFile(example.FontFile(name))
	if err == nil {
		rdr = bytes.NewReader(buf)
		fmt.Printf("Generalized font loader reading %s\n", name)
	}
	return
}

// strDelimit converts 'ABCDEFG' to, for example, 'A,BCD,EFG'
func strDelimit(str string, sepstr string, sepcount int) string {
	pos := len(str) - sepcount
	for pos > 0 {
		str = str[:pos] + sepstr + str[pos:]
		pos = pos - sepcount
	}
	return str
}

func loremList() []string {
	return []string{
		"Lorem ipsum dolor sit amet, consectetur adipisicing elit, sed do eiusmod " +
			"tempor incididunt ut labore et dolore magna aliqua.",
		"Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut " +
			"aliquip ex ea commodo consequat.",
		"Duis aute irure dolor in reprehenderit in voluptate velit esse cillum " +
			"dolore eu fugiat nulla pariatur.",
		"Excepteur sint occaecat cupidatat non proident, sunt in culpa qui " +
			"officia deserunt mollit anim id est laborum.",
	}
}

func lorem() string {
	return strings.Join(loremList(), " ")
}

// Example demonstrates the generation of a simple PDF document. Note that
// since only core fonts are used (in this case Arial, a synonym for
// Helvetica), an empty string can be specified for the font directory in the
// call to New(). Note also that the example.Filename() and example.Summary()
// functions belong to a separate, internal package and are not part of the
// gofpdf library. If an error occurs at some point during the construction of
// the document, subsequent method calls exit immediately and the error is
// finally retrieved with the output call where it can be handled by the
// application.
func Example() {
	pdf := gofpdf.New(gofpdf.OrientationPortrait, "mm", "A4", "")
	pdf.AddPage()
	pdf.SetFont("Arial", "B", 16)
	pdf.Cell(40, 10, "Hello World!")
	fileStr := example.Filename("basic")
	err := pdf.OutputFileAndClose(fileStr)
	example.Summary(err, fileStr)
	// Output:
	// Successfully generated pdf/basic.pdf
}

// ExampleFpdf_AddPage demonsrates the generation of headers, footers and page breaks.
func ExampleFpdf_AddPage() {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetTopMargin(30)
	pdf.SetHeaderFuncMode(func() {
		pdf.Image(example.ImageFile("logo.png"), 10, 6, 30, 0, false, "", 0, "")
		pdf.SetY(5)
		pdf.SetFont("Arial", "B", 15)
		pdf.Cell(80, 0, "")
		pdf.CellFormat(30, 10, "Title", "1", 0, "C", false, 0, "")
		pdf.Ln(20)
	}, true)
	pdf.SetFooterFunc(func() {
		pdf.SetY(-15)
		pdf.SetFont("Arial", "I", 8)
		pdf.CellFormat(0, 10, fmt.Sprintf("Page %d/{nb}", pdf.PageNo()),
			"", 0, "C", false, 0, "")
	})
	pdf.AliasNbPages("")
	pdf.AddPage()
	pdf.SetFont("Times", "", 12)
	for j := 1; j <= 40; j++ {
		pdf.CellFormat(0, 10, fmt.Sprintf("Printing line number %d", j),
			"", 1, "", false, 0, "")
	}
	fileStr := example.Filename("Fpdf_AddPage")
	err := pdf.OutputFileAndClose(fileStr)
	example.Summary(err, fileStr)
	// Output:
	// Successfully generated pdf/Fpdf_AddPage.pdf
}

// ExampleFpdf_MultiCell demonstrates word-wrapping, line justification and
// page-breaking.
func ExampleFpdf_MultiCell() {
	pdf := gofpdf.New("P", "mm", "A4", "")
	titleStr := "20000 Leagues Under the Seas"
	pdf.SetTitle(titleStr, false)
	pdf.SetAuthor("Jules Verne", false)
	pdf.SetHeaderFunc(func() {
		// Arial bold 15
		pdf.SetFont("Arial", "B", 15)
		// Calculate width of title and position
		wd := pdf.GetStringWidth(titleStr) + 6
		pdf.SetX((210 - wd) / 2)
		// Colors of frame, background and text
		pdf.SetDrawColor(0, 80, 180)
		pdf.SetFillColor(230, 230, 0)
		pdf.SetTextColor(220, 50, 50)
		// Thickness of frame (1 mm)
		pdf.SetLineWidth(1)
		// Title
		pdf.CellFormat(wd, 9, titleStr, "1", 1, "C", true, 0, "")
		// Line break
		pdf.Ln(10)
	})
	pdf.SetFooterFunc(func() {
		// Position at 1.5 cm from bottom
		pdf.SetY(-15)
		// Arial italic 8
		pdf.SetFont("Arial", "I", 8)
		// Text color in gray
		pdf.SetTextColor(128, 128, 128)
		// Page number
		pdf.CellFormat(0, 10, fmt.Sprintf("Page %d", pdf.PageNo()),
			"", 0, "C", false, 0, "")
	})
	chapterTitle := func(chapNum int, titleStr string) {
		// 	// Arial 12
		pdf.SetFont("Arial", "", 12)
		// Background color
		pdf.SetFillColor(200, 220, 255)
		// Title
		pdf.CellFormat(0, 6, fmt.Sprintf("Chapter %d : %s", chapNum, titleStr),
			"", 1, "L", true, 0, "")
		// Line break
		pdf.Ln(4)
	}
	chapterBody := func(fileStr string) {
		// Read text file
		txtStr, err := ioutil.ReadFile(fileStr)
		if err != nil {
			pdf.SetError(err)
		}
		// Times 12
		pdf.SetFont("Times", "", 12)
		// Output justified text
		pdf.MultiCell(0, 5, string(txtStr), "", "", false)
		// Line break
		pdf.Ln(-1)
		// Mention in italics
		pdf.SetFont("", "I", 0)
		pdf.Cell(0, 5, "(end of excerpt)")
	}
	printChapter := func(chapNum int, titleStr, fileStr string) {
		pdf.AddPage()
		chapterTitle(chapNum, titleStr)
		chapterBody(fileStr)
	}
	printChapter(1, "A RUNAWAY REEF", example.TextFile("20k_c1.txt"))
	printChapter(2, "THE PROS AND CONS", example.TextFile("20k_c2.txt"))
	fileStr := example.Filename("Fpdf_MultiCell")
	err := pdf.OutputFileAndClose(fileStr)
	example.Summary(err, fileStr)
	// Output:
	// Successfully generated pdf/Fpdf_MultiCell.pdf
}

// ExampleFpdf_SetLeftMargin demonstrates the generation of a PDF document that has multiple
// columns. This is accomplished with the SetLeftMargin() and Cell() methods.
func ExampleFpdf_SetLeftMargin() {
	var y0 float64
	var crrntCol int
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetDisplayMode("fullpage", "TwoColumnLeft")
	titleStr := "20000 Leagues Under the Seas"
	pdf.SetTitle(titleStr, false)
	pdf.SetAuthor("Jules Verne", false)
	setCol := func(col int) {
		// Set position at a given column
		crrntCol = col
		x := 10.0 + float64(col)*65.0
		pdf.SetLeftMargin(x)
		pdf.SetX(x)
	}
	chapterTitle := func(chapNum int, titleStr string) {
		// Arial 12
		pdf.SetFont("Arial", "", 12)
		// Background color
		pdf.SetFillColor(200, 220, 255)
		// Title
		pdf.CellFormat(0, 6, fmt.Sprintf("Chapter %d : %s", chapNum, titleStr),
			"", 1, "L", true, 0, "")
		// Line break
		pdf.Ln(4)
		y0 = pdf.GetY()
	}
	chapterBody := func(fileStr string) {
		// Read text file
		txtStr, err := ioutil.ReadFile(fileStr)
		if err != nil {
			pdf.SetError(err)
		}
		// Font
		pdf.SetFont("Times", "", 12)
		// Output text in a 6 cm width column
		pdf.MultiCell(60, 5, string(txtStr), "", "", false)
		pdf.Ln(-1)
		// Mention
		pdf.SetFont("", "I", 0)
		pdf.Cell(0, 5, "(end of excerpt)")
		// Go back to first column
		setCol(0)
	}
	printChapter := func(num int, titleStr, fileStr string) {
		// Add chapter
		pdf.AddPage()
		chapterTitle(num, titleStr)
		chapterBody(fileStr)
	}
	pdf.SetAcceptPageBreakFunc(func() bool {
		// Method accepting or not automatic page break
		if crrntCol < 2 {
			// Go to next column
			setCol(crrntCol + 1)
			// Set ordinate to top
			pdf.SetY(y0)
			// Keep on page
			return false
		}
		// Go back to first column
		setCol(0)
		// Page break
		return true
	})
	pdf.SetHeaderFunc(func() {
		// Arial bold 15
		pdf.SetFont("Arial", "B", 15)
		// Calculate width of title and position
		wd := pdf.GetStringWidth(titleStr) + 6
		pdf.SetX((210 - wd) / 2)
		// Colors of frame, background and text
		pdf.SetDrawColor(0, 80, 180)
		pdf.SetFillColor(230, 230, 0)
		pdf.SetTextColor(220, 50, 50)
		// Thickness of frame (1 mm)
		pdf.SetLineWidth(1)
		// Title
		pdf.CellFormat(wd, 9, titleStr, "1", 1, "C", true, 0, "")
		// Line break
		pdf.Ln(10)
		// Save ordinate
		y0 = pdf.GetY()
	})
	pdf.SetFooterFunc(func() {
		// Position at 1.5 cm from bottom
		pdf.SetY(-15)
		// Arial italic 8
		pdf.SetFont("Arial", "I", 8)
		// Text color in gray
		pdf.SetTextColor(128, 128, 128)
		// Page number
		pdf.CellFormat(0, 10, fmt.Sprintf("Page %d", pdf.PageNo()),
			"", 0, "C", false, 0, "")
	})
	printChapter(1, "A RUNAWAY REEF", example.TextFile("20k_c1.txt"))
	printChapter(2, "THE PROS AND CONS", example.TextFile("20k_c2.txt"))
	fileStr := example.Filename("Fpdf_SetLeftMargin_multicolumn")
	err := pdf.OutputFileAndClose(fileStr)
	example.Summary(err, fileStr)
	// Output:
	// Successfully generated pdf/Fpdf_SetLeftMargin_multicolumn.pdf
}

// ExampleFpdf_SplitLines_tables demonstrates word-wrapped table cells
func ExampleFpdf_SplitLines_tables() {
	const (
		colCount = 3
		colWd    = 60.0
		marginH  = 15.0
		lineHt   = 5.5
		cellGap  = 2.0
	)
	// var colStrList [colCount]string
	type cellType struct {
		str  string
		list [][]byte
		ht   float64
	}
	var (
		cellList [colCount]cellType
		cell     cellType
	)

	pdf := gofpdf.New("P", "mm", "A4", "") // 210 x 297
	header := [colCount]string{"Column A", "Column B", "Column C"}
	alignList := [colCount]string{"L", "C", "R"}
	strList := loremList()
	pdf.SetMargins(marginH, 15, marginH)
	pdf.SetFont("Arial", "", 14)
	pdf.AddPage()

	// Headers
	pdf.SetTextColor(224, 224, 224)
	pdf.SetFillColor(64, 64, 64)
	for colJ := 0; colJ < colCount; colJ++ {
		pdf.CellFormat(colWd, 10, header[colJ], "1", 0, "CM", true, 0, "")
	}
	pdf.Ln(-1)
	pdf.SetTextColor(24, 24, 24)
	pdf.SetFillColor(255, 255, 255)

	// Rows
	y := pdf.GetY()
	count := 0
	for rowJ := 0; rowJ < 2; rowJ++ {
		maxHt := lineHt
		// Cell height calculation loop
		for colJ := 0; colJ < colCount; colJ++ {
			count++
			if count > len(strList) {
				count = 1
			}
			cell.str = strings.Join(strList[0:count], " ")
			cell.list = pdf.SplitLines([]byte(cell.str), colWd-cellGap-cellGap)
			cell.ht = float64(len(cell.list)) * lineHt
			if cell.ht > maxHt {
				maxHt = cell.ht
			}
			cellList[colJ] = cell
		}
		// Cell render loop
		x := marginH
		for colJ := 0; colJ < colCount; colJ++ {
			pdf.Rect(x, y, colWd, maxHt+cellGap+cellGap, "D")
			cell = cellList[colJ]
			cellY := y + cellGap + (maxHt-cell.ht)/2
			for splitJ := 0; splitJ < len(cell.list); splitJ++ {
				pdf.SetXY(x+cellGap, cellY)
				pdf.CellFormat(colWd-cellGap-cellGap, lineHt, string(cell.list[splitJ]), "", 0,
					alignList[colJ], false, 0, "")
				cellY += lineHt
			}
			x += colWd
		}
		y += maxHt + cellGap + cellGap
	}

	fileStr := example.Filename("Fpdf_SplitLines_tables")
	err := pdf.OutputFileAndClose(fileStr)
	example.Summary(err, fileStr)
	// Output:
	// Successfully generated pdf/Fpdf_SplitLines_tables.pdf
}

// ExampleFpdf_CellFormat_tables demonstrates various table styles.
func ExampleFpdf_CellFormat_tables() {
	pdf := gofpdf.New("P", "mm", "A4", "")
	type countryType struct {
		nameStr, capitalStr, areaStr, popStr string
	}
	countryList := make([]countryType, 0, 8)
	header := []string{"Country", "Capital", "Area (sq km)", "Pop. (thousands)"}
	loadData := func(fileStr string) {
		fl, err := os.Open(fileStr)
		if err == nil {
			scanner := bufio.NewScanner(fl)
			var c countryType
			for scanner.Scan() {
				// Austria;Vienna;83859;8075
				lineStr := scanner.Text()
				list := strings.Split(lineStr, ";")
				if len(list) == 4 {
					c.nameStr = list[0]
					c.capitalStr = list[1]
					c.areaStr = list[2]
					c.popStr = list[3]
					countryList = append(countryList, c)
				} else {
					err = fmt.Errorf("error tokenizing %s", lineStr)
				}
			}
			fl.Close()
			if len(countryList) == 0 {
				err = fmt.Errorf("error loading data from %s", fileStr)
			}
		}
		if err != nil {
			pdf.SetError(err)
		}
	}
	// Simple table
	basicTable := func() {
		left := (210.0 - 4*40) / 2
		pdf.SetX(left)
		for _, str := range header {
			pdf.CellFormat(40, 7, str, "1", 0, "", false, 0, "")
		}
		pdf.Ln(-1)
		for _, c := range countryList {
			pdf.SetX(left)
			pdf.CellFormat(40, 6, c.nameStr, "1", 0, "", false, 0, "")
			pdf.CellFormat(40, 6, c.capitalStr, "1", 0, "", false, 0, "")
			pdf.CellFormat(40, 6, c.areaStr, "1", 0, "", false, 0, "")
			pdf.CellFormat(40, 6, c.popStr, "1", 0, "", false, 0, "")
			pdf.Ln(-1)
		}
	}
	// Better table
	improvedTable := func() {
		// Column widths
		w := []float64{40.0, 35.0, 40.0, 45.0}
		wSum := 0.0
		for _, v := range w {
			wSum += v
		}
		left := (210 - wSum) / 2
		// 	Header
		pdf.SetX(left)
		for j, str := range header {
			pdf.CellFormat(w[j], 7, str, "1", 0, "C", false, 0, "")
		}
		pdf.Ln(-1)
		// Data
		for _, c := range countryList {
			pdf.SetX(left)
			pdf.CellFormat(w[0], 6, c.nameStr, "LR", 0, "", false, 0, "")
			pdf.CellFormat(w[1], 6, c.capitalStr, "LR", 0, "", false, 0, "")
			pdf.CellFormat(w[2], 6, strDelimit(c.areaStr, ",", 3),
				"LR", 0, "R", false, 0, "")
			pdf.CellFormat(w[3], 6, strDelimit(c.popStr, ",", 3),
				"LR", 0, "R", false, 0, "")
			pdf.Ln(-1)
		}
		pdf.SetX(left)
		pdf.CellFormat(wSum, 0, "", "T", 0, "", false, 0, "")
	}
	// Colored table
	fancyTable := func() {
		// Colors, line width and bold font
		pdf.SetFillColor(255, 0, 0)
		pdf.SetTextColor(255, 255, 255)
		pdf.SetDrawColor(128, 0, 0)
		pdf.SetLineWidth(.3)
		pdf.SetFont("", "B", 0)
		// 	Header
		w := []float64{40, 35, 40, 45}
		wSum := 0.0
		for _, v := range w {
			wSum += v
		}
		left := (210 - wSum) / 2
		pdf.SetX(left)
		for j, str := range header {
			pdf.CellFormat(w[j], 7, str, "1", 0, "C", true, 0, "")
		}
		pdf.Ln(-1)
		// Color and font restoration
		pdf.SetFillColor(224, 235, 255)
		pdf.SetTextColor(0, 0, 0)
		pdf.SetFont("", "", 0)
		// 	Data
		fill := false
		for _, c := range countryList {
			pdf.SetX(left)
			pdf.CellFormat(w[0], 6, c.nameStr, "LR", 0, "", fill, 0, "")
			pdf.CellFormat(w[1], 6, c.capitalStr, "LR", 0, "", fill, 0, "")
			pdf.CellFormat(w[2], 6, strDelimit(c.areaStr, ",", 3),
				"LR", 0, "R", fill, 0, "")
			pdf.CellFormat(w[3], 6, strDelimit(c.popStr, ",", 3),
				"LR", 0, "R", fill, 0, "")
			pdf.Ln(-1)
			fill = !fill
		}
		pdf.SetX(left)
		pdf.CellFormat(wSum, 0, "", "T", 0, "", false, 0, "")
	}
	loadData(example.TextFile("countries.txt"))
	pdf.SetFont("Arial", "", 14)
	pdf.AddPage()
	basicTable()
	pdf.AddPage()
	improvedTable()
	pdf.AddPage()
	fancyTable()
	fileStr := example.Filename("Fpdf_CellFormat_tables")
	err := pdf.OutputFileAndClose(fileStr)
	example.Summary(err, fileStr)
	// Output:
	// Successfully generated pdf/Fpdf_CellFormat_tables.pdf
}

// ExampleFpdf_HTMLBasicNew demonstrates internal and external links with and without basic
// HTML.
func ExampleFpdf_HTMLBasicNew() {
	pdf := gofpdf.New("P", "mm", "A4", "")
	// First page: manual local link
	pdf.AddPage()
	pdf.SetFont("Helvetica", "", 20)
	_, lineHt := pdf.GetFontSize()
	pdf.Write(lineHt, "To find out what's new in this tutorial, click ")
	pdf.SetFont("", "U", 0)
	link := pdf.AddLink()
	pdf.WriteLinkID(lineHt, "here", link)
	pdf.SetFont("", "", 0)
	// Second page: image link and basic HTML with link
	pdf.AddPage()
	pdf.SetLink(link, 0, -1)
	pdf.Image(example.ImageFile("logo.png"), 10, 12, 30, 0, false, "", 0, "http://www.fpdf.org")
	pdf.SetLeftMargin(45)
	pdf.SetFontSize(14)
	_, lineHt = pdf.GetFontSize()
	htmlStr := `You can now easily print text mixing different styles: <b>bold</b>, ` +
		`<i>italic</i>, <u>underlined</u>, or <b><i><u>all at once</u></i></b>!<br><br>` +
		`<center>You can also center text.</center>` +
		`<right>Or align it to the right.</right>` +
		`You can also insert links on text, such as ` +
		`<a href="http://www.fpdf.org">www.fpdf.org</a>, or on an image: click on the logo.`
	html := pdf.HTMLBasicNew()
	html.Write(lineHt, htmlStr)
	fileStr := example.Filename("Fpdf_HTMLBasicNew")
	err := pdf.OutputFileAndClose(fileStr)
	example.Summary(err, fileStr)
	// Output:
	// Successfully generated pdf/Fpdf_HTMLBasicNew.pdf
}

// ExampleFpdf_AddFont demonstrates the use of a non-standard font.
func ExampleFpdf_AddFont() {
	pdf := gofpdf.New("P", "mm", "A4", example.FontDir())
	pdf.AddFont("Calligrapher", "", "calligra.json")
	pdf.AddPage()
	pdf.SetFont("Calligrapher", "", 35)
	pdf.Cell(0, 10, "Enjoy new fonts with FPDF!")
	fileStr := example.Filename("Fpdf_AddFont")
	err := pdf.OutputFileAndClose(fileStr)
	example.Summary(err, fileStr)
	// Output:
	// Successfully generated pdf/Fpdf_AddFont.pdf
}

// ExampleFpdf_WriteAligned demonstrates how to align text with the Write function.
func ExampleFpdf_WriteAligned() {
	pdf := gofpdf.New("P", "mm", "A4", example.FontDir())
	pdf.SetLeftMargin(50.0)
	pdf.SetRightMargin(50.0)
	pdf.AddPage()
	pdf.SetFont("Helvetica", "", 12)
	pdf.WriteAligned(0, 35, "This text is the default alignment, Left", "")
	pdf.Ln(35)
	pdf.WriteAligned(0, 35, "This text is aligned Left", "L")
	pdf.Ln(35)
	pdf.WriteAligned(0, 35, "This text is aligned Center", "C")
	pdf.Ln(35)
	pdf.WriteAligned(0, 35, "This text is aligned Right", "R")
	pdf.Ln(35)
	line := "This can by used to write justified text"
	leftMargin, _, rightMargin, _ := pdf.GetMargins()
	pageWidth, _ := pdf.GetPageSize()
	pageWidth -= leftMargin + rightMargin
	pdf.SetWordSpacing((pageWidth - pdf.GetStringWidth(line)) / float64(strings.Count(line, " ")))
	pdf.WriteAligned(pageWidth, 35, line, "L")
	fileStr := example.Filename("Fpdf_WriteAligned")
	err := pdf.OutputFileAndClose(fileStr)
	example.Summary(err, fileStr)
	// Output:
	// Successfully generated pdf/Fpdf_WriteAligned.pdf
}

// ExampleFpdf_Image demonstrates how images are included in documents.
func ExampleFpdf_Image() {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddPage()
	pdf.SetFont("Arial", "", 11)
	pdf.Image(example.ImageFile("logo.png"), 10, 10, 30, 0, false, "", 0, "")
	pdf.Text(50, 20, "logo.png")
	pdf.Image(example.ImageFile("logo.gif"), 10, 40, 30, 0, false, "", 0, "")
	pdf.Text(50, 50, "logo.gif")
	pdf.Image(example.ImageFile("logo-gray.png"), 10, 70, 30, 0, false, "", 0, "")
	pdf.Text(50, 80, "logo-gray.png")
	pdf.Image(example.ImageFile("logo-rgb.png"), 10, 100, 30, 0, false, "", 0, "")
	pdf.Text(50, 110, "logo-rgb.png")
	pdf.Image(example.ImageFile("logo.jpg"), 10, 130, 30, 0, false, "", 0, "")
	pdf.Text(50, 140, "logo.jpg")
	fileStr := example.Filename("Fpdf_Image")
	err := pdf.OutputFileAndClose(fileStr)
	example.Summary(err, fileStr)
	// Output:
	// Successfully generated pdf/Fpdf_Image.pdf
}

// ExampleFpdf_ImageOptions demonstrates how the AllowNegativePosition field of the
// ImageOption struct can be used to affect horizontal image placement.
func ExampleFpdf_ImageOptions() {
	var opt gofpdf.ImageOptions

	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddPage()
	pdf.SetFont("Arial", "", 11)
	pdf.SetX(60)
	opt.ImageType = "png"
	pdf.ImageOptions(example.ImageFile("logo.png"), -10, 10, 30, 0, false, opt, 0, "")
	opt.AllowNegativePosition = true
	pdf.ImageOptions(example.ImageFile("logo.png"), -10, 50, 30, 0, false, opt, 0, "")
	fileStr := example.Filename("Fpdf_ImageOptions")
	err := pdf.OutputFileAndClose(fileStr)
	example.Summary(err, fileStr)
	// Output:
	// Successfully generated pdf/Fpdf_ImageOptions.pdf
}

// ExampleFpdf_RegisterImageOptionsReader demonstrates how to load an image
// from a io.Reader (in this case, a file) and register it with options.
func ExampleFpdf_RegisterImageOptionsReader() {
	var (
		opt    gofpdf.ImageOptions
		pdfStr string
		fl     *os.File
		err    error
	)

	pdfStr = example.Filename("Fpdf_RegisterImageOptionsReader")
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddPage()
	pdf.SetFont("Arial", "", 11)
	fl, err = os.Open(example.ImageFile("logo.png"))
	if err == nil {
		opt.ImageType = "png"
		opt.AllowNegativePosition = true
		_ = pdf.RegisterImageOptionsReader("logo", opt, fl)
		fl.Close()
		for x := -20.0; x <= 40.0; x += 5 {
			pdf.ImageOptions("logo", x, x+30, 0, 0, false, opt, 0, "")
		}
		err = pdf.OutputFileAndClose(pdfStr)
	}
	example.Summary(err, pdfStr)
	// Output:
	// Successfully generated pdf/Fpdf_RegisterImageOptionsReader.pdf
}

// This example demonstrates Landscape mode with images.
func ExampleFpdf_SetAcceptPageBreakFunc() {
	var y0 float64
	var crrntCol int
	loremStr := lorem()
	pdf := gofpdf.New("L", "mm", "A4", "")
	const (
		pageWd = 297.0 // A4 210.0 x 297.0
		margin = 10.0
		gutter = 4
		colNum = 3
		colWd  = (pageWd - 2*margin - (colNum-1)*gutter) / colNum
	)
	setCol := func(col int) {
		crrntCol = col
		x := margin + float64(col)*(colWd+gutter)
		pdf.SetLeftMargin(x)
		pdf.SetX(x)
	}
	pdf.SetHeaderFunc(func() {
		titleStr := "gofpdf"
		pdf.SetFont("Helvetica", "B", 48)
		wd := pdf.GetStringWidth(titleStr) + 6
		pdf.SetX((pageWd - wd) / 2)
		pdf.SetTextColor(128, 128, 160)
		pdf.Write(12, titleStr[:2])
		pdf.SetTextColor(128, 128, 128)
		pdf.Write(12, titleStr[2:])
		pdf.Ln(20)
		y0 = pdf.GetY()
	})
	pdf.SetAcceptPageBreakFunc(func() bool {
		if crrntCol < colNum-1 {
			setCol(crrntCol + 1)
			pdf.SetY(y0)
			// Start new column, not new page
			return false
		}
		setCol(0)
		return true
	})
	pdf.AddPage()
	pdf.SetFont("Times", "", 12)
	for j := 0; j < 20; j++ {
		if j == 1 {
			pdf.Image(example.ImageFile("fpdf.png"), -1, 0, colWd, 0, true, "", 0, "")
		} else if j == 5 {
			pdf.Image(example.ImageFile("golang-gopher.png"),
				-1, 0, colWd, 0, true, "", 0, "")
		}
		pdf.MultiCell(colWd, 5, loremStr, "", "", false)
		pdf.Ln(-1)
	}
	fileStr := example.Filename("Fpdf_SetAcceptPageBreakFunc_landscape")
	err := pdf.OutputFileAndClose(fileStr)
	example.Summary(err, fileStr)
	// Output:
	// Successfully generated pdf/Fpdf_SetAcceptPageBreakFunc_landscape.pdf
}

// This example tests corner cases as reported by the gocov tool.
func ExampleFpdf_SetKeywords() {
	var err error
	fileStr := example.Filename("Fpdf_SetKeywords")
	err = gofpdf.MakeFont(example.FontFile("CalligrapherRegular.pfb"),
		example.FontFile("cp1252.map"), example.FontDir(), nil, true)
	if err == nil {
		pdf := gofpdf.New("", "", "", "")
		pdf.SetFontLocation(example.FontDir())
		pdf.SetTitle("世界", true)
		pdf.SetAuthor("世界", true)
		pdf.SetSubject("世界", true)
		pdf.SetCreator("世界", true)
		pdf.SetKeywords("世界", true)
		pdf.AddFont("Calligrapher", "", "CalligrapherRegular.json")
		pdf.AddPage()
		pdf.SetFont("Calligrapher", "", 16)
		pdf.Writef(5, "\x95 %s \x95", pdf)
		err = pdf.OutputFileAndClose(fileStr)
	}
	example.Summary(err, fileStr)
	// Output:
	// Successfully generated pdf/Fpdf_SetKeywords.pdf
}

// ExampleFpdf_Circle demonstrates the construction of various geometric figures,
func ExampleFpdf_Circle() {
	const (
		thin  = 0.2
		thick = 3.0
	)
	pdf := gofpdf.New("", "", "", "")
	pdf.SetFont("Helvetica", "", 12)
	pdf.SetFillColor(200, 200, 220)
	pdf.AddPage()

	y := 15.0
	pdf.Text(10, y, "Circles")
	pdf.SetFillColor(200, 200, 220)
	pdf.SetLineWidth(thin)
	pdf.Circle(20, y+15, 10, "D")
	pdf.Circle(45, y+15, 10, "F")
	pdf.Circle(70, y+15, 10, "FD")
	pdf.SetLineWidth(thick)
	pdf.Circle(95, y+15, 10, "FD")
	pdf.SetLineWidth(thin)

	y += 40.0
	pdf.Text(10, y, "Ellipses")
	pdf.SetFillColor(220, 200, 200)
	pdf.Ellipse(30, y+15, 20, 10, 0, "D")
	pdf.Ellipse(75, y+15, 20, 10, 0, "F")
	pdf.Ellipse(120, y+15, 20, 10, 0, "FD")
	pdf.SetLineWidth(thick)
	pdf.Ellipse(165, y+15, 20, 10, 0, "FD")
	pdf.SetLineWidth(thin)

	y += 40.0
	pdf.Text(10, y, "Curves (quadratic)")
	pdf.SetFillColor(220, 220, 200)
	pdf.Curve(10, y+30, 15, y-20, 40, y+30, "D")
	pdf.Curve(45, y+30, 50, y-20, 75, y+30, "F")
	pdf.Curve(80, y+30, 85, y-20, 110, y+30, "FD")
	pdf.SetLineWidth(thick)
	pdf.Curve(115, y+30, 120, y-20, 145, y+30, "FD")
	pdf.SetLineCapStyle("round")
	pdf.Curve(150, y+30, 155, y-20, 180, y+30, "FD")
	pdf.SetLineWidth(thin)
	pdf.SetLineCapStyle("butt")

	y += 40.0
	pdf.Text(10, y, "Curves (cubic)")
	pdf.SetFillColor(220, 200, 220)
	pdf.CurveBezierCubic(10, y+30, 15, y-20, 10, y+30, 40, y+30, "D")
	pdf.CurveBezierCubic(45, y+30, 50, y-20, 45, y+30, 75, y+30, "F")
	pdf.CurveBezierCubic(80, y+30, 85, y-20, 80, y+30, 110, y+30, "FD")
	pdf.SetLineWidth(thick)
	pdf.CurveBezierCubic(115, y+30, 120, y-20, 115, y+30, 145, y+30, "FD")
	pdf.SetLineCapStyle("round")
	pdf.CurveBezierCubic(150, y+30, 155, y-20, 150, y+30, 180, y+30, "FD")
	pdf.SetLineWidth(thin)
	pdf.SetLineCapStyle("butt")

	y += 40.0
	pdf.Text(10, y, "Arcs")
	pdf.SetFillColor(200, 220, 220)
	pdf.SetLineWidth(thick)
	pdf.Arc(45, y+35, 20, 10, 0, 0, 180, "FD")
	pdf.SetLineWidth(thin)
	pdf.Arc(45, y+35, 25, 15, 0, 90, 270, "D")
	pdf.SetLineWidth(thick)
	pdf.Arc(45, y+35, 30, 20, 0, 0, 360, "D")
	pdf.SetLineCapStyle("round")
	pdf.Arc(135, y+35, 20, 10, 135, 0, 180, "FD")
	pdf.SetLineWidth(thin)
	pdf.Arc(135, y+35, 25, 15, 135, 90, 270, "D")
	pdf.SetLineWidth(thick)
	pdf.Arc(135, y+35, 30, 20, 135, 0, 360, "D")
	pdf.SetLineWidth(thin)
	pdf.SetLineCapStyle("butt")

	fileStr := example.Filename("Fpdf_Circle_figures")
	err := pdf.OutputFileAndClose(fileStr)
	example.Summary(err, fileStr)
	// Output:
	// Successfully generated pdf/Fpdf_Circle_figures.pdf
}

// ExampleFpdf_SetAlpha demonstrates alpha transparency.
func ExampleFpdf_SetAlpha() {
	const (
		gapX  = 10.0
		gapY  = 9.0
		rectW = 40.0
		rectH = 58.0
		pageW = 210
		pageH = 297
	)
	modeList := []string{"Normal", "Multiply", "Screen", "Overlay",
		"Darken", "Lighten", "ColorDodge", "ColorBurn", "HardLight", "SoftLight",
		"Difference", "Exclusion", "Hue", "Saturation", "Color", "Luminosity"}
	pdf := gofpdf.New("", "", "", "")
	pdf.SetLineWidth(2)
	pdf.SetAutoPageBreak(false, 0)
	pdf.AddPage()
	pdf.SetFont("Helvetica", "", 18)
	pdf.SetXY(0, gapY)
	pdf.SetTextColor(0, 0, 0)
	pdf.CellFormat(pageW, gapY, "Alpha Blending Modes", "", 0, "C", false, 0, "")
	j := 0
	y := 3 * gapY
	for col := 0; col < 4; col++ {
		x := gapX
		for row := 0; row < 4; row++ {
			pdf.Rect(x, y, rectW, rectH, "D")
			pdf.SetFont("Helvetica", "B", 12)
			pdf.SetFillColor(0, 0, 0)
			pdf.SetTextColor(250, 250, 230)
			pdf.SetXY(x, y+rectH-4)
			pdf.CellFormat(rectW, 5, modeList[j], "", 0, "C", true, 0, "")
			pdf.SetFont("Helvetica", "I", 150)
			pdf.SetTextColor(80, 80, 120)
			pdf.SetXY(x, y+2)
			pdf.CellFormat(rectW, rectH, "A", "", 0, "C", false, 0, "")
			pdf.SetAlpha(0.5, modeList[j])
			pdf.Image(example.ImageFile("golang-gopher.png"),
				x-gapX, y, rectW+2*gapX, 0, false, "", 0, "")
			pdf.SetAlpha(1.0, "Normal")
			x += rectW + gapX
			j++
		}
		y += rectH + gapY
	}
	fileStr := example.Filename("Fpdf_SetAlpha_transparency")
	err := pdf.OutputFileAndClose(fileStr)
	example.Summary(err, fileStr)
	// Output:
	// Successfully generated pdf/Fpdf_SetAlpha_transparency.pdf
}

// ExampleFpdf_LinearGradient deomstrates various gradients.
func ExampleFpdf_LinearGradient() {
	pdf := gofpdf.New("", "", "", "")
	pdf.SetFont("Helvetica", "", 12)
	pdf.AddPage()
	pdf.LinearGradient(0, 0, 210, 100, 250, 250, 255, 220, 220, 225, 0, 0, 0, .5)
	pdf.LinearGradient(20, 25, 75, 75, 220, 220, 250, 80, 80, 220, 0, .2, 0, .8)
	pdf.Rect(20, 25, 75, 75, "D")
	pdf.LinearGradient(115, 25, 75, 75, 220, 220, 250, 80, 80, 220, 0, 0, 1, 1)
	pdf.Rect(115, 25, 75, 75, "D")
	pdf.RadialGradient(20, 120, 75, 75, 220, 220, 250, 80, 80, 220,
		0.25, 0.75, 0.25, 0.75, 1)
	pdf.Rect(20, 120, 75, 75, "D")
	pdf.RadialGradient(115, 120, 75, 75, 220, 220, 250, 80, 80, 220,
		0.25, 0.75, 0.75, 0.75, 0.75)
	pdf.Rect(115, 120, 75, 75, "D")
	fileStr := example.Filename("Fpdf_LinearGradient_gradient")
	err := pdf.OutputFileAndClose(fileStr)
	example.Summary(err, fileStr)
	// Output:
	// Successfully generated pdf/Fpdf_LinearGradient_gradient.pdf
}

// ExampleFpdf_ClipText demonstrates clipping.
func ExampleFpdf_ClipText() {
	pdf := gofpdf.New("", "", "", "")
	y := 10.0
	pdf.AddPage()

	pdf.SetFont("Helvetica", "", 24)
	pdf.SetXY(0, y)
	pdf.ClipText(10, y+12, "Clipping examples", false)
	pdf.RadialGradient(10, y, 100, 20, 128, 128, 160, 32, 32, 48,
		0.25, 0.5, 0.25, 0.5, 0.2)
	pdf.ClipEnd()

	y += 12
	pdf.SetFont("Helvetica", "B", 120)
	pdf.SetDrawColor(64, 80, 80)
	pdf.SetLineWidth(.5)
	pdf.ClipText(10, y+40, pdf.String(), true)
	pdf.RadialGradient(10, y, 200, 50, 220, 220, 250, 80, 80, 220,
		0.25, 0.5, 0.25, 0.5, 1)
	pdf.ClipEnd()

	y += 55
	pdf.ClipRect(10, y, 105, 20, true)
	pdf.SetFillColor(255, 255, 255)
	pdf.Rect(10, y, 105, 20, "F")
	pdf.ClipCircle(40, y+10, 15, false)
	pdf.RadialGradient(25, y, 30, 30, 220, 250, 220, 40, 60, 40, 0.3,
		0.85, 0.3, 0.85, 0.5)
	pdf.ClipEnd()
	pdf.ClipEllipse(80, y+10, 20, 15, false)
	pdf.RadialGradient(60, y, 40, 30, 250, 220, 220, 60, 40, 40, 0.3,
		0.85, 0.3, 0.85, 0.5)
	pdf.ClipEnd()
	pdf.ClipEnd()

	y += 28
	pdf.ClipEllipse(26, y+10, 16, 10, true)
	pdf.Image(example.ImageFile("logo.jpg"), 10, y, 32, 0, false, "JPG", 0, "")
	pdf.ClipEnd()

	pdf.ClipCircle(60, y+10, 10, true)
	pdf.RadialGradient(50, y, 20, 20, 220, 220, 250, 40, 40, 60, 0.3,
		0.7, 0.3, 0.7, 0.5)
	pdf.ClipEnd()

	pdf.ClipPolygon([]gofpdf.PointType{{X: 80, Y: y + 20}, {X: 90, Y: y},
		{X: 100, Y: y + 20}}, true)
	pdf.LinearGradient(80, y, 20, 20, 250, 220, 250, 60, 40, 60, 0.5,
		1, 0.5, 0.5)
	pdf.ClipEnd()

	y += 30
	pdf.SetLineWidth(.1)
	pdf.SetDrawColor(180, 180, 180)
	pdf.ClipRoundedRect(10, y, 120, 20, 5, true)
	pdf.RadialGradient(10, y, 120, 20, 255, 255, 255, 240, 240, 220,
		0.25, 0.75, 0.25, 0.75, 0.5)
	pdf.SetXY(5, y-5)
	pdf.SetFont("Times", "", 12)
	pdf.MultiCell(130, 5, lorem(), "", "", false)
	pdf.ClipEnd()

	y += 30
	pdf.SetDrawColor(180, 100, 180)
	pdf.ClipRoundedRectExt(10, y, 120, 20, 5, 10, 5, 10, true)
	pdf.RadialGradient(10, y, 120, 20, 255, 255, 255, 240, 240, 220,
		0.25, 0.75, 0.25, 0.75, 0.5)
	pdf.SetXY(5, y-5)
	pdf.SetFont("Times", "", 12)
	pdf.MultiCell(130, 5, lorem(), "", "", false)
	pdf.ClipEnd()

	fileStr := example.Filename("Fpdf_ClipText")
	err := pdf.OutputFileAndClose(fileStr)
	example.Summary(err, fileStr)
	// Output:
	// Successfully generated pdf/Fpdf_ClipText.pdf
}

// ExampleFpdf_PageSize generates a PDF document with various page sizes.
func ExampleFpdf_PageSize() {
	pdf := gofpdf.NewCustom(&gofpdf.InitType{
		UnitStr:    "in",
		Size:       gofpdf.SizeType{Wd: 6, Ht: 6},
		FontDirStr: example.FontDir(),
	})
	pdf.SetMargins(0.5, 1, 0.5)
	pdf.SetFont("Times", "", 14)
	pdf.AddPageFormat("L", gofpdf.SizeType{Wd: 3, Ht: 12})
	pdf.SetXY(0.5, 1.5)
	pdf.CellFormat(11, 0.2, "12 in x 3 in", "", 0, "C", false, 0, "")
	pdf.AddPage() // Default size established in NewCustom()
	pdf.SetXY(0.5, 3)
	pdf.CellFormat(5, 0.2, "6 in x 6 in", "", 0, "C", false, 0, "")
	pdf.AddPageFormat("P", gofpdf.SizeType{Wd: 3, Ht: 12})
	pdf.SetXY(0.5, 6)
	pdf.CellFormat(2, 0.2, "3 in x 12 in", "", 0, "C", false, 0, "")
	for j := 0; j <= 3; j++ {
		wd, ht, u := pdf.PageSize(j)
		fmt.Printf("%d: %6.2f %s, %6.2f %s\n", j, wd, u, ht, u)
	}
	fileStr := example.Filename("Fpdf_PageSize")
	err := pdf.OutputFileAndClose(fileStr)
	example.Summary(err, fileStr)
	// Output:
	// 0:   6.00 in,   6.00 in
	// 1:  12.00 in,   3.00 in
	// 2:   6.00 in,   6.00 in
	// 3:   3.00 in,  12.00 in
	// Successfully generated pdf/Fpdf_PageSize.pdf
}

// ExampleFpdf_Bookmark demonstrates the Bookmark method.
func ExampleFpdf_Bookmark() {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddPage()
	pdf.SetFont("Arial", "", 15)
	pdf.Bookmark("Page 1", 0, 0)
	pdf.Bookmark("Paragraph 1", 1, -1)
	pdf.Cell(0, 6, "Paragraph 1")
	pdf.Ln(50)
	pdf.Bookmark("Paragraph 2", 1, -1)
	pdf.Cell(0, 6, "Paragraph 2")
	pdf.AddPage()
	pdf.Bookmark("Page 2", 0, 0)
	pdf.Bookmark("Paragraph 3", 1, -1)
	pdf.Cell(0, 6, "Paragraph 3")
	fileStr := example.Filename("Fpdf_Bookmark")
	err := pdf.OutputFileAndClose(fileStr)
	example.Summary(err, fileStr)
	// Output:
	// Successfully generated pdf/Fpdf_Bookmark.pdf
}

// ExampleFpdf_TransformBegin demonstrates various transformations. It is adapted from an
// example script by Moritz Wagner and Andreas Würmser.
func ExampleFpdf_TransformBegin() {
	const (
		light = 200
		dark  = 0
	)
	var refX, refY float64
	var refStr string
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddPage()
	color := func(val int) {
		pdf.SetDrawColor(val, val, val)
		pdf.SetTextColor(val, val, val)
	}
	reference := func(str string, x, y float64, val int) {
		color(val)
		pdf.Rect(x, y, 40, 10, "D")
		pdf.Text(x, y-1, str)
	}
	refDraw := func(str string, x, y float64) {
		refStr = str
		refX = x
		refY = y
		reference(str, x, y, light)
	}
	refDupe := func() {
		reference(refStr, refX, refY, dark)
	}

	titleStr := "Transformations"
	titlePt := 36.0
	titleHt := pdf.PointConvert(titlePt)
	pdf.SetFont("Helvetica", "", titlePt)
	titleWd := pdf.GetStringWidth(titleStr)
	titleX := (210 - titleWd) / 2
	pdf.Text(titleX, 10+titleHt, titleStr)
	pdf.TransformBegin()
	pdf.TransformMirrorVertical(10 + titleHt + 0.5)
	pdf.ClipText(titleX, 10+titleHt, titleStr, false)
	// Remember that the transform will mirror the gradient box too
	pdf.LinearGradient(titleX, 10, titleWd, titleHt+4, 120, 120, 120,
		255, 255, 255, 0, 0, 0, 0.6)
	pdf.ClipEnd()
	pdf.TransformEnd()

	pdf.SetFont("Helvetica", "", 12)

	// Scale by 150% centered by lower left corner of the rectangle
	refDraw("Scale", 50, 60)
	pdf.TransformBegin()
	pdf.TransformScaleXY(150, 50, 70)
	refDupe()
	pdf.TransformEnd()

	// Translate 7 to the right, 5 to the bottom
	refDraw("Translate", 125, 60)
	pdf.TransformBegin()
	pdf.TransformTranslate(7, 5)
	refDupe()
	pdf.TransformEnd()

	// Rotate 20 degrees counter-clockwise centered by the lower left corner of
	// the rectangle
	refDraw("Rotate", 50, 110)
	pdf.TransformBegin()
	pdf.TransformRotate(20, 50, 120)
	refDupe()
	pdf.TransformEnd()

	// Skew 30 degrees along the x-axis centered by the lower left corner of the
	// rectangle
	refDraw("Skew", 125, 110)
	pdf.TransformBegin()
	pdf.TransformSkewX(30, 125, 110)
	refDupe()
	pdf.TransformEnd()

	// Mirror horizontally with axis of reflection at left side of the rectangle
	refDraw("Mirror horizontal", 50, 160)
	pdf.TransformBegin()
	pdf.TransformMirrorHorizontal(50)
	refDupe()
	pdf.TransformEnd()

	// Mirror vertically with axis of reflection at bottom side of the rectangle
	refDraw("Mirror vertical", 125, 160)
	pdf.TransformBegin()
	pdf.TransformMirrorVertical(170)
	refDupe()
	pdf.TransformEnd()

	// Reflect against a point at the lower left point of rectangle
	refDraw("Mirror point", 50, 210)
	pdf.TransformBegin()
	pdf.TransformMirrorPoint(50, 220)
	refDupe()
	pdf.TransformEnd()

	// Mirror against a straight line described by a point and an angle
	angle := -20.0
	px := 120.0
	py := 220.0
	refDraw("Mirror line", 125, 210)
	pdf.TransformBegin()
	pdf.TransformRotate(angle, px, py)
	pdf.Line(px-1, py-1, px+1, py+1)
	pdf.Line(px-1, py+1, px+1, py-1)
	pdf.Line(px-5, py, px+60, py)
	pdf.TransformEnd()
	pdf.TransformBegin()
	pdf.TransformMirrorLine(angle, px, py)
	refDupe()
	pdf.TransformEnd()

	fileStr := example.Filename("Fpdf_TransformBegin")
	err := pdf.OutputFileAndClose(fileStr)
	example.Summary(err, fileStr)
	// Output:
	// Successfully generated pdf/Fpdf_TransformBegin.pdf
}

// ExampleFpdf_RegisterImage demonstrates Lawrence Kesteloot's image registration code.
func ExampleFpdf_RegisterImage() {
	const (
		margin = 10
		wd     = 210
		ht     = 297
	)
	fileList := []string{
		"logo-gray.png",
		"logo.jpg",
		"logo.png",
		"logo-rgb.png",
		"logo-progressive.jpg",
	}
	var infoPtr *gofpdf.ImageInfoType
	var imageFileStr string
	var imgWd, imgHt, lf, tp float64
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddPage()
	pdf.SetMargins(10, 10, 10)
	pdf.SetFont("Helvetica", "", 15)
	for j, str := range fileList {
		imageFileStr = example.ImageFile(str)
		infoPtr = pdf.RegisterImage(imageFileStr, "")
		imgWd, imgHt = infoPtr.Extent()
		switch j {
		case 0:
			lf = margin
			tp = margin
		case 1:
			lf = wd - margin - imgWd
			tp = margin
		case 2:
			lf = (wd - imgWd) / 2.0
			tp = (ht - imgHt) / 2.0
		case 3:
			lf = margin
			tp = ht - imgHt - margin
		case 4:
			lf = wd - imgWd - margin
			tp = ht - imgHt - margin
		}
		pdf.Image(imageFileStr, lf, tp, imgWd, imgHt, false, "", 0, "")
	}
	fileStr := example.Filename("Fpdf_RegisterImage")
	// Test the image information retrieval method
	infoShow := func(imageStr string) {
		imageStr = example.ImageFile(imageStr)
		info := pdf.GetImageInfo(imageStr)
		if info != nil {
			if info.Width() > 0.0 {
				fmt.Printf("Image %s is registered\n", filepath.ToSlash(imageStr))
			} else {
				fmt.Printf("Incorrect information for image %s\n", filepath.ToSlash(imageStr))
			}
		} else {
			fmt.Printf("Image %s is not registered\n", filepath.ToSlash(imageStr))
		}
	}
	infoShow(fileList[0])
	infoShow("foo.png")
	err := pdf.OutputFileAndClose(fileStr)
	example.Summary(err, fileStr)
	// Output:
	// Image image/logo-gray.png is registered
	// Image image/foo.png is not registered
	// Successfully generated pdf/Fpdf_RegisterImage.pdf
}

// ExampleFpdf_SplitLines demonstrates Bruno Michel's line splitting function.
func ExampleFpdf_SplitLines() {
	const (
		fontPtSize = 18.0
		wd         = 100.0
	)
	pdf := gofpdf.New("P", "mm", "A4", "") // A4 210.0 x 297.0
	pdf.SetFont("Times", "", fontPtSize)
	_, lineHt := pdf.GetFontSize()
	pdf.AddPage()
	pdf.SetMargins(10, 10, 10)
	lines := pdf.SplitLines([]byte(lorem()), wd)
	ht := float64(len(lines)) * lineHt
	y := (297.0 - ht) / 2.0
	pdf.SetDrawColor(128, 128, 128)
	pdf.SetFillColor(255, 255, 210)
	x := (210.0 - (wd + 40.0)) / 2.0
	pdf.Rect(x, y-20.0, wd+40.0, ht+40.0, "FD")
	pdf.SetY(y)
	for _, line := range lines {
		pdf.CellFormat(190.0, lineHt, string(line), "", 1, "C", false, 0, "")
	}
	fileStr := example.Filename("Fpdf_Splitlines")
	err := pdf.OutputFileAndClose(fileStr)
	example.Summary(err, fileStr)
	// Output:
	// Successfully generated pdf/Fpdf_Splitlines.pdf
}

// ExampleFpdf_SVGBasicWrite demonstrates how to render a simple path-only SVG image of the
// type generated by the jSignature web control.
func ExampleFpdf_SVGBasicWrite() {
	const (
		fontPtSize = 16.0
		wd         = 100.0
		sigFileStr = "signature.svg"
	)
	var (
		sig gofpdf.SVGBasicType
		err error
	)
	pdf := gofpdf.New("P", "mm", "A4", "") // A4 210.0 x 297.0
	pdf.SetFont("Times", "", fontPtSize)
	lineHt := pdf.PointConvert(fontPtSize)
	pdf.AddPage()
	pdf.SetMargins(10, 10, 10)
	htmlStr := `This example renders a simple ` +
		`<a href="http://www.w3.org/TR/SVG/">SVG</a> (scalable vector graphics) ` +
		`image that contains only basic path commands without any styling, ` +
		`color fill, reflection or endpoint closures. In particular, the ` +
		`type of vector graphic returned from a ` +
		`<a href="http://willowsystems.github.io/jSignature/#/demo/">jSignature</a> ` +
		`web control is supported and is used in this example.`
	html := pdf.HTMLBasicNew()
	html.Write(lineHt, htmlStr)
	sig, err = gofpdf.SVGBasicFileParse(example.ImageFile(sigFileStr))
	if err == nil {
		scale := 100 / sig.Wd
		scaleY := 30 / sig.Ht
		if scale > scaleY {
			scale = scaleY
		}
		pdf.SetLineCapStyle("round")
		pdf.SetLineWidth(0.25)
		pdf.SetDrawColor(0, 0, 128)
		pdf.SetXY((210.0-scale*sig.Wd)/2.0, pdf.GetY()+10)
		pdf.SVGBasicWrite(&sig, scale)
	} else {
		pdf.SetError(err)
	}
	fileStr := example.Filename("Fpdf_SVGBasicWrite")
	err = pdf.OutputFileAndClose(fileStr)
	example.Summary(err, fileStr)
	// Output:
	// Successfully generated pdf/Fpdf_SVGBasicWrite.pdf
}

// ExampleFpdf_CellFormat_align demonstrates Stefan Schroeder's code to control vertical
// alignment.
func ExampleFpdf_CellFormat_align() {
	type recType struct {
		align, txt string
	}
	recList := []recType{
		{"TL", "top left"},
		{"TC", "top center"},
		{"TR", "top right"},
		{"LM", "middle left"},
		{"CM", "middle center"},
		{"RM", "middle right"},
		{"BL", "bottom left"},
		{"BC", "bottom center"},
		{"BR", "bottom right"},
	}
	recListBaseline := []recType{
		{"AL", "baseline left"},
		{"AC", "baseline center"},
		{"AR", "baseline right"},
	}
	var formatRect = func(pdf *gofpdf.Fpdf, recList []recType) {
		linkStr := ""
		for pageJ := 0; pageJ < 2; pageJ++ {
			pdf.AddPage()
			pdf.SetMargins(10, 10, 10)
			pdf.SetAutoPageBreak(false, 0)
			borderStr := "1"
			for _, rec := range recList {
				pdf.SetXY(20, 20)
				pdf.CellFormat(170, 257, rec.txt, borderStr, 0, rec.align, false, 0, linkStr)
				borderStr = ""
			}
			linkStr = "https://github.com/airb3/gofpdf"
		}
	}
	pdf := gofpdf.New("P", "mm", "A4", "") // A4 210.0 x 297.0
	pdf.SetFont("Helvetica", "", 16)
	formatRect(pdf, recList)
	formatRect(pdf, recListBaseline)
	var fr fontResourceType
	pdf.SetFontLoader(fr)
	pdf.AddFont("Calligrapher", "", "calligra.json")
	pdf.SetFont("Calligrapher", "", 16)
	formatRect(pdf, recListBaseline)
	fileStr := example.Filename("Fpdf_CellFormat_align")
	err := pdf.OutputFileAndClose(fileStr)
	example.Summary(err, fileStr)
	// Output:
	// Generalized font loader reading calligra.json
	// Generalized font loader reading calligra.z
	// Successfully generated pdf/Fpdf_CellFormat_align.pdf
}

// ExampleFpdf_CellFormat_codepageescape demonstrates the use of characters in the high range of the
// Windows-1252 code page (gofdpf default). See the example for CellFormat (4)
// for a way to do this automatically.
func ExampleFpdf_CellFormat_codepageescape() {
	pdf := gofpdf.New("P", "mm", "A4", "") // A4 210.0 x 297.0
	fontSize := 16.0
	pdf.SetFont("Helvetica", "", fontSize)
	ht := pdf.PointConvert(fontSize)
	write := func(str string) {
		pdf.CellFormat(190, ht, str, "", 1, "C", false, 0, "")
		pdf.Ln(ht)
	}
	pdf.AddPage()
	htmlStr := `Until gofpdf supports UTF-8 encoded source text, source text needs ` +
		`to be specified with all special characters escaped to match the code page ` +
		`layout of the currently selected font. By default, gofdpf uses code page 1252.` +
		` See <a href="http://en.wikipedia.org/wiki/Windows-1252">Wikipedia</a> for ` +
		`a table of this layout.`
	html := pdf.HTMLBasicNew()
	html.Write(ht, htmlStr)
	pdf.Ln(2 * ht)
	write("Voix ambigu\xeb d'un c\x9cur qui au z\xe9phyr pr\xe9f\xe8re les jattes de kiwi.")
	write("Falsches \xdcben von Xylophonmusik qu\xe4lt jeden gr\xf6\xdferen Zwerg.")
	write("Heiz\xf6lr\xfccksto\xdfabd\xe4mpfung")
	write("For\xe5rsj\xe6vnd\xf8gn / Efter\xe5rsj\xe6vnd\xf8gn")
	fileStr := example.Filename("Fpdf_CellFormat_codepageescape")
	err := pdf.OutputFileAndClose(fileStr)
	example.Summary(err, fileStr)
	// Output:
	// Successfully generated pdf/Fpdf_CellFormat_codepageescape.pdf
}

// ExampleFpdf_CellFormat_codepage demonstrates the automatic conversion of UTF-8 strings to an
// 8-bit font encoding.
func ExampleFpdf_CellFormat_codepage() {
	pdf := gofpdf.New("P", "mm", "A4", example.FontDir()) // A4 210.0 x 297.0
	// See documentation for details on how to generate fonts
	pdf.AddFont("Helvetica-1251", "", "helvetica_1251.json")
	pdf.AddFont("Helvetica-1253", "", "helvetica_1253.json")
	fontSize := 16.0
	pdf.SetFont("Helvetica", "", fontSize)
	ht := pdf.PointConvert(fontSize)
	tr := pdf.UnicodeTranslatorFromDescriptor("") // "" defaults to "cp1252"
	write := func(str string) {
		// pdf.CellFormat(190, ht, tr(str), "", 1, "C", false, 0, "")
		pdf.MultiCell(190, ht, tr(str), "", "C", false)
		pdf.Ln(ht)
	}
	pdf.AddPage()
	str := `Gofpdf provides a translator that will convert any UTF-8 code point ` +
		`that is present in the specified code page.`
	pdf.MultiCell(190, ht, str, "", "L", false)
	pdf.Ln(2 * ht)
	write("Voix ambiguë d'un cœur qui au zéphyr préfère les jattes de kiwi.")
	write("Falsches Üben von Xylophonmusik quält jeden größeren Zwerg.")
	write("Heizölrückstoßabdämpfung")
	write("Forårsjævndøgn / Efterårsjævndøgn")
	write("À noite, vovô Kowalsky vê o ímã cair no pé do pingüim queixoso e vovó" +
		"põe açúcar no chá de tâmaras do jabuti feliz.")
	pdf.SetFont("Helvetica-1251", "", fontSize) // Name matches one specified in AddFont()
	tr = pdf.UnicodeTranslatorFromDescriptor("cp1251")
	write("Съешь же ещё этих мягких французских булок, да выпей чаю.")

	pdf.SetFont("Helvetica-1253", "", fontSize)
	tr = pdf.UnicodeTranslatorFromDescriptor("cp1253")
	write("Θέλει αρετή και τόλμη η ελευθερία. (Ανδρέας Κάλβος)")

	fileStr := example.Filename("Fpdf_CellFormat_codepage")
	err := pdf.OutputFileAndClose(fileStr)
	example.Summary(err, fileStr)
	// Output:
	// Successfully generated pdf/Fpdf_CellFormat_codepage.pdf
}

// ExampleFpdf_SetProtection demonstrates password protection for documents.
func ExampleFpdf_SetProtection() {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetProtection(gofpdf.CnProtectPrint, "123", "abc")
	pdf.AddPage()
	pdf.SetFont("Arial", "", 12)
	pdf.Write(10, "Password-protected.")
	fileStr := example.Filename("Fpdf_SetProtection")
	err := pdf.OutputFileAndClose(fileStr)
	example.Summary(err, fileStr)
	// Output:
	// Successfully generated pdf/Fpdf_SetProtection.pdf
}

// ExampleFpdf_Polygon displays equilateral polygons in a demonstration of the Polygon
// function.
func ExampleFpdf_Polygon() {
	const rowCount = 5
	const colCount = 4
	const ptSize = 36
	var x, y, radius, gap, advance float64
	var rgVal int
	var pts []gofpdf.PointType
	vertices := func(count int) (res []gofpdf.PointType) {
		var pt gofpdf.PointType
		res = make([]gofpdf.PointType, 0, count)
		mlt := 2.0 * math.Pi / float64(count)
		for j := 0; j < count; j++ {
			pt.Y, pt.X = math.Sincos(float64(j) * mlt)
			res = append(res, gofpdf.PointType{
				X: x + radius*pt.X,
				Y: y + radius*pt.Y})
		}
		return
	}
	pdf := gofpdf.New("P", "mm", "A4", "") // A4 210.0 x 297.0
	pdf.AddPage()
	pdf.SetFont("Helvetica", "", ptSize)
	pdf.SetDrawColor(0, 80, 180)
	gap = 12.0
	pdf.SetY(gap)
	pdf.CellFormat(190.0, gap, "Equilateral polygons", "", 1, "C", false, 0, "")
	radius = (210.0 - float64(colCount+1)*gap) / (2.0 * float64(colCount))
	advance = gap + 2.0*radius
	y = 2*gap + pdf.PointConvert(ptSize) + radius
	rgVal = 230
	for row := 0; row < rowCount; row++ {
		pdf.SetFillColor(rgVal, rgVal, 0)
		rgVal -= 12
		x = gap + radius
		for col := 0; col < colCount; col++ {
			pts = vertices(row*colCount + col + 3)
			pdf.Polygon(pts, "FD")
			x += advance
		}
		y += advance
	}
	fileStr := example.Filename("Fpdf_Polygon")
	err := pdf.OutputFileAndClose(fileStr)
	example.Summary(err, fileStr)
	// Output:
	// Successfully generated pdf/Fpdf_Polygon.pdf
}

// ExampleFpdf_AddLayer demonstrates document layers. The initial visibility of a layer
// is specified with the second parameter to AddLayer(). The layer list
// displayed by the document reader allows layer visibility to be controlled
// interactively.
func ExampleFpdf_AddLayer() {

	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddPage()
	pdf.SetFont("Arial", "", 15)
	pdf.Write(8, "This line doesn't belong to any layer.\n")

	// Define layers
	l1 := pdf.AddLayer("Layer 1", true)
	l2 := pdf.AddLayer("Layer 2", true)

	// Open layer pane in PDF viewer
	pdf.OpenLayerPane()

	// First layer
	pdf.BeginLayer(l1)
	pdf.Write(8, "This line belongs to layer 1.\n")
	pdf.EndLayer()

	// Second layer
	pdf.BeginLayer(l2)
	pdf.Write(8, "This line belongs to layer 2.\n")
	pdf.EndLayer()

	// First layer again
	pdf.BeginLayer(l1)
	pdf.Write(8, "This line belongs to layer 1 again.\n")
	pdf.EndLayer()

	fileStr := example.Filename("Fpdf_AddLayer")
	err := pdf.OutputFileAndClose(fileStr)
	example.Summary(err, fileStr)
	// Output:
	// Successfully generated pdf/Fpdf_AddLayer.pdf
}

// ExampleFpdf_RegisterImageReader demonstrates the use of an image that is retrieved from a web
// server.
func ExampleFpdf_RegisterImageReader() {

	const (
		margin   = 10
		wd       = 210
		ht       = 297
		fontSize = 15
		urlStr   = "https://github.com/airb3/gofpdf/blob/master/image/gofpdf.png?raw=true"
		msgStr   = `Images from the web can be easily embedded when a PDF document is generated.`
	)

	var (
		rsp *http.Response
		err error
		tp  string
	)

	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddPage()
	pdf.SetFont("Helvetica", "", fontSize)
	ln := pdf.PointConvert(fontSize)
	pdf.MultiCell(wd-margin-margin, ln, msgStr, "", "L", false)
	rsp, err = http.Get(urlStr)
	if err == nil {
		tp = pdf.ImageTypeFromMime(rsp.Header["Content-Type"][0])
		infoPtr := pdf.RegisterImageReader(urlStr, tp, rsp.Body)
		if pdf.Ok() {
			imgWd, imgHt := infoPtr.Extent()
			pdf.Image(urlStr, (wd-imgWd)/2.0, pdf.GetY()+ln,
				imgWd, imgHt, false, tp, 0, "")
		}
	} else {
		pdf.SetError(err)
	}
	fileStr := example.Filename("Fpdf_RegisterImageReader_url")
	err = pdf.OutputFileAndClose(fileStr)
	example.Summary(err, fileStr)
	// Output:
	// Successfully generated pdf/Fpdf_RegisterImageReader_url.pdf

}

// ExampleFpdf_Beziergon demonstrates the Beziergon function.
func ExampleFpdf_Beziergon() {

	const (
		margin      = 10
		wd          = 210
		unit        = (wd - 2*margin) / 6
		ht          = 297
		fontSize    = 15
		msgStr      = `Demonstration of Beziergon function`
		coefficient = 0.6
		delta       = coefficient * unit
		ln          = fontSize * 25.4 / 72
		offsetX     = (wd - 4*unit) / 2.0
		offsetY     = offsetX + 2*ln
	)

	srcList := []gofpdf.PointType{
		{X: 0, Y: 0},
		{X: 1, Y: 0},
		{X: 1, Y: 1},
		{X: 2, Y: 1},
		{X: 2, Y: 2},
		{X: 3, Y: 2},
		{X: 3, Y: 3},
		{X: 4, Y: 3},
		{X: 4, Y: 4},
		{X: 1, Y: 4},
		{X: 1, Y: 3},
		{X: 0, Y: 3},
	}

	ctrlList := []gofpdf.PointType{
		{X: 1, Y: -1},
		{X: 1, Y: 1},
		{X: 1, Y: 1},
		{X: 1, Y: 1},
		{X: 1, Y: 1},
		{X: 1, Y: 1},
		{X: 1, Y: 1},
		{X: 1, Y: 1},
		{X: -1, Y: 1},
		{X: -1, Y: -1},
		{X: -1, Y: -1},
		{X: -1, Y: -1},
	}

	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddPage()
	pdf.SetFont("Helvetica", "", fontSize)
	for j, src := range srcList {
		srcList[j].X = offsetX + src.X*unit
		srcList[j].Y = offsetY + src.Y*unit
	}
	for j, ctrl := range ctrlList {
		ctrlList[j].X = ctrl.X * delta
		ctrlList[j].Y = ctrl.Y * delta
	}
	jPrev := len(srcList) - 1
	srcPrev := srcList[jPrev]
	curveList := []gofpdf.PointType{srcPrev} // point [, control 0, control 1, point]*
	control := func(x, y float64) {
		curveList = append(curveList, gofpdf.PointType{X: x, Y: y})
	}
	for j, src := range srcList {
		ctrl := ctrlList[jPrev]
		control(srcPrev.X+ctrl.X, srcPrev.Y+ctrl.Y) // Control 0
		ctrl = ctrlList[j]
		control(src.X-ctrl.X, src.Y-ctrl.Y) // Control 1
		curveList = append(curveList, src)  // Destination
		jPrev = j
		srcPrev = src
	}
	pdf.MultiCell(wd-margin-margin, ln, msgStr, "", "C", false)
	pdf.SetDashPattern([]float64{0.8, 0.8}, 0)
	pdf.SetDrawColor(160, 160, 160)
	pdf.Polygon(srcList, "D")
	pdf.SetDashPattern([]float64{}, 0)
	pdf.SetDrawColor(64, 64, 128)
	pdf.SetLineWidth(pdf.GetLineWidth() * 3)
	pdf.Beziergon(curveList, "D")
	fileStr := example.Filename("Fpdf_Beziergon")
	err := pdf.OutputFileAndClose(fileStr)
	example.Summary(err, fileStr)
	// Output:
	// Successfully generated pdf/Fpdf_Beziergon.pdf

}

// ExampleFpdf_SetFontLoader demonstrates loading a non-standard font using a generalized
// font loader. fontResourceType implements the FontLoader interface and is
// defined locally in the test source code.
func ExampleFpdf_SetFontLoader() {
	var fr fontResourceType
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetFontLoader(fr)
	pdf.AddFont("Calligrapher", "", "calligra.json")
	pdf.AddPage()
	pdf.SetFont("Calligrapher", "", 35)
	pdf.Cell(0, 10, "Load fonts from any source")
	fileStr := example.Filename("Fpdf_SetFontLoader")
	err := pdf.OutputFileAndClose(fileStr)
	example.Summary(err, fileStr)
	// Output:
	// Generalized font loader reading calligra.json
	// Generalized font loader reading calligra.z
	// Successfully generated pdf/Fpdf_SetFontLoader.pdf
}

// ExampleFpdf_MoveTo demonstrates the Path Drawing functions, such as: MoveTo,
// LineTo, CurveTo, ..., ClosePath and DrawPath.
func ExampleFpdf_MoveTo() {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddPage()
	pdf.MoveTo(20, 20)
	pdf.LineTo(170, 20)
	pdf.ArcTo(170, 40, 20, 20, 0, 90, 0)
	pdf.CurveTo(190, 100, 105, 100)
	pdf.CurveBezierCubicTo(20, 100, 105, 200, 20, 200)
	pdf.ClosePath()
	pdf.SetFillColor(200, 200, 200)
	pdf.SetLineWidth(3)
	pdf.DrawPath("DF")
	fileStr := example.Filename("Fpdf_MoveTo_path")
	err := pdf.OutputFileAndClose(fileStr)
	example.Summary(err, fileStr)
	// Output:
	// Successfully generated pdf/Fpdf_MoveTo_path.pdf
}

// ExampleFpdf_SetLineJoinStyle demonstrates various line cap and line join styles.
func ExampleFpdf_SetLineJoinStyle() {
	const offset = 75.0
	pdf := gofpdf.New("L", "mm", "A4", "")
	pdf.AddPage()
	var draw = func(cap, join string, x0, y0, x1, y1 float64) {
		// transform begin & end needed to isolate caps and joins
		pdf.SetLineCapStyle(cap)
		pdf.SetLineJoinStyle(join)

		// Draw thick line
		pdf.SetDrawColor(0x33, 0x33, 0x33)
		pdf.SetLineWidth(30.0)
		pdf.MoveTo(x0, y0)
		pdf.LineTo((x0+x1)/2+offset, (y0+y1)/2)
		pdf.LineTo(x1, y1)
		pdf.DrawPath("D")

		// Draw thin helping line
		pdf.SetDrawColor(0xFF, 0x33, 0x33)
		pdf.SetLineWidth(2.56)
		pdf.MoveTo(x0, y0)
		pdf.LineTo((x0+x1)/2+offset, (y0+y1)/2)
		pdf.LineTo(x1, y1)
		pdf.DrawPath("D")

	}
	x := 35.0
	caps := []string{"butt", "square", "round"}
	joins := []string{"bevel", "miter", "round"}
	for i := range caps {
		draw(caps[i], joins[i], x, 50, x, 160)
		x += offset
	}
	fileStr := example.Filename("Fpdf_SetLineJoinStyle_caps")
	err := pdf.OutputFileAndClose(fileStr)
	example.Summary(err, fileStr)
	// Output:
	// Successfully generated pdf/Fpdf_SetLineJoinStyle_caps.pdf
}

// ExampleFpdf_DrawPath demonstrates various fill modes.
func ExampleFpdf_DrawPath() {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetDrawColor(0xff, 0x00, 0x00)
	pdf.SetFillColor(0x99, 0x99, 0x99)
	pdf.SetFont("Helvetica", "", 15)
	pdf.AddPage()
	pdf.SetAlpha(1, "Multiply")
	var (
		polygon = func(cx, cy, r, n, dir float64) {
			da := 2 * math.Pi / n
			pdf.MoveTo(cx+r, cy)
			pdf.Text(cx+r, cy, "0")
			i := 1
			for a := da; a < 2*math.Pi; a += da {
				x, y := cx+r*math.Cos(dir*a), cy+r*math.Sin(dir*a)
				pdf.LineTo(x, y)
				pdf.Text(x, y, strconv.Itoa(i))
				i++
			}
			pdf.ClosePath()
		}
		polygons = func(cx, cy, r, n, dir float64) {
			d := 1.0
			for rf := r; rf > 0; rf -= 10 {
				polygon(cx, cy, rf, n, d)
				d *= dir
			}
		}
		star = func(cx, cy, r, n float64) {
			da := 4 * math.Pi / n
			pdf.MoveTo(cx+r, cy)
			for a := da; a < 4*math.Pi+da; a += da {
				x, y := cx+r*math.Cos(a), cy+r*math.Sin(a)
				pdf.LineTo(x, y)
			}
			pdf.ClosePath()
		}
	)
	// triangle
	polygons(55, 45, 40, 3, 1)
	pdf.DrawPath("B")
	pdf.Text(15, 95, "B (same direction, non zero winding)")

	// square
	polygons(155, 45, 40, 4, 1)
	pdf.DrawPath("B*")
	pdf.Text(115, 95, "B* (same direction, even odd)")

	// pentagon
	polygons(55, 145, 40, 5, -1)
	pdf.DrawPath("B")
	pdf.Text(15, 195, "B (different direction, non zero winding)")

	// hexagon
	polygons(155, 145, 40, 6, -1)
	pdf.DrawPath("B*")
	pdf.Text(115, 195, "B* (different direction, even odd)")

	// star
	star(55, 245, 40, 5)
	pdf.DrawPath("B")
	pdf.Text(15, 290, "B (non zero winding)")

	// star
	star(155, 245, 40, 5)
	pdf.DrawPath("B*")
	pdf.Text(115, 290, "B* (even odd)")

	fileStr := example.Filename("Fpdf_DrawPath_fill")
	err := pdf.OutputFileAndClose(fileStr)
	example.Summary(err, fileStr)
	// Output:
	// Successfully generated pdf/Fpdf_DrawPath_fill.pdf
}

// ExampleFpdf_CreateTemplate demonstrates creating and using templates
func ExampleFpdf_CreateTemplate() {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetCompression(false)
	// pdf.SetFont("Times", "", 12)
	template := pdf.CreateTemplate(func(tpl *gofpdf.Tpl) {
		tpl.Image(example.ImageFile("logo.png"), 6, 6, 30, 0, false, "", 0, "")
		tpl.SetFont("Arial", "B", 16)
		tpl.Text(40, 20, "Template says hello")
		tpl.SetDrawColor(0, 100, 200)
		tpl.SetLineWidth(2.5)
		tpl.Line(95, 12, 105, 22)
	})
	_, tplSize := template.Size()
	// fmt.Println("Size:", tplSize)
	// fmt.Println("Scaled:", tplSize.ScaleBy(1.5))

	template2 := pdf.CreateTemplate(func(tpl *gofpdf.Tpl) {
		tpl.UseTemplate(template)
		subtemplate := tpl.CreateTemplate(func(tpl2 *gofpdf.Tpl) {
			tpl2.Image(example.ImageFile("logo.png"), 6, 86, 30, 0, false, "", 0, "")
			tpl2.SetFont("Arial", "B", 16)
			tpl2.Text(40, 100, "Subtemplate says hello")
			tpl2.SetDrawColor(0, 200, 100)
			tpl2.SetLineWidth(2.5)
			tpl2.Line(102, 92, 112, 102)
		})
		tpl.UseTemplate(subtemplate)
	})

	pdf.SetDrawColor(200, 100, 0)
	pdf.SetLineWidth(2.5)
	pdf.SetFont("Arial", "B", 16)

	// serialize and deserialize template
	b, _ := template2.Serialize()
	template3, _ := gofpdf.DeserializeTemplate(b)

	pdf.AddPage()
	pdf.UseTemplate(template3)
	pdf.UseTemplateScaled(template3, gofpdf.PointType{X: 0, Y: 30}, tplSize)
	pdf.Line(40, 210, 60, 210)
	pdf.Text(40, 200, "Template example page 1")

	pdf.AddPage()
	pdf.UseTemplate(template2)
	pdf.UseTemplateScaled(template3, gofpdf.PointType{X: 0, Y: 30}, tplSize.ScaleBy(1.4))
	pdf.Line(60, 210, 80, 210)
	pdf.Text(40, 200, "Template example page 2")

	fileStr := example.Filename("Fpdf_CreateTemplate")
	err := pdf.OutputFileAndClose(fileStr)
	example.Summary(err, fileStr)
	// Output:
	// Successfully generated pdf/Fpdf_CreateTemplate.pdf
}

// ExampleFpdf_AddFontFromBytes demonstrate how to use embedded fonts from byte array
func ExampleFpdf_AddFontFromBytes() {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddPage()
	pdf.AddFontFromBytes("calligra", "", files.CalligraJson, files.CalligraZ)
	pdf.SetFont("calligra", "", 16)
	pdf.Cell(40, 10, "Hello World With Embedded Font!")
	fileStr := example.Filename("Fpdf_EmbeddedFont")
	err := pdf.OutputFileAndClose(fileStr)
	example.Summary(err, fileStr)
	// Output:
	// Successfully generated pdf/Fpdf_EmbeddedFont.pdf
}

// This example demonstrate Clipped table cells
func ExampleFpdf_ClipRect() {
	marginCell := 2. // margin of top/bottom of cell
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetFont("Arial", "", 12)
	pdf.AddPage()
	pagew, pageh := pdf.GetPageSize()
	mleft, mright, _, mbottom := pdf.GetMargins()

	cols := []float64{60, 100, pagew - mleft - mright - 100 - 60}
	rows := [][]string{}
	for i := 1; i <= 50; i++ {
		word := fmt.Sprintf("%d:%s", i, strings.Repeat("A", i%100))
		rows = append(rows, []string{word, word, word})
	}

	for _, row := range rows {
		_, lineHt := pdf.GetFontSize()
		height := lineHt + marginCell

		x, y := pdf.GetXY()
		// add a new page if the height of the row doesn't fit on the page
		if y+height >= pageh-mbottom {
			pdf.AddPage()
			x, y = pdf.GetXY()
		}
		for i, txt := range row {
			width := cols[i]
			pdf.Rect(x, y, width, height, "")
			pdf.ClipRect(x, y, width, height, false)
			pdf.Cell(width, height, txt)
			pdf.ClipEnd()
			x += width
		}
		pdf.Ln(-1)
	}
	fileStr := example.Filename("Fpdf_ClippedTableCells")
	err := pdf.OutputFileAndClose(fileStr)
	example.Summary(err, fileStr)
	// Output:
	// Successfully generated pdf/Fpdf_ClippedTableCells.pdf
}

// This example demonstrate wrapped table cells
func ExampleFpdf_Rect() {
	marginCell := 2. // margin of top/bottom of cell
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetFont("Arial", "", 12)
	pdf.AddPage()
	pagew, pageh := pdf.GetPageSize()
	mleft, mright, _, mbottom := pdf.GetMargins()

	cols := []float64{60, 100, pagew - mleft - mright - 100 - 60}
	rows := [][]string{}
	for i := 1; i <= 30; i++ {
		word := fmt.Sprintf("%d:%s", i, strings.Repeat("A", i%100))
		rows = append(rows, []string{word, word, word})
	}

	for _, row := range rows {
		curx, y := pdf.GetXY()
		x := curx

		height := 0.
		_, lineHt := pdf.GetFontSize()

		for i, txt := range row {
			lines := pdf.SplitLines([]byte(txt), cols[i])
			h := float64(len(lines))*lineHt + marginCell*float64(len(lines))
			if h > height {
				height = h
			}
		}
		// add a new page if the height of the row doesn't fit on the page
		if pdf.GetY()+height > pageh-mbottom {
			pdf.AddPage()
			y = pdf.GetY()
		}
		for i, txt := range row {
			width := cols[i]
			pdf.Rect(x, y, width, height, "")
			pdf.MultiCell(width, lineHt+marginCell, txt, "", "", false)
			x += width
			pdf.SetXY(x, y)
		}
		pdf.SetXY(curx, y+height)
	}
	fileStr := example.Filename("Fpdf_WrappedTableCells")
	err := pdf.OutputFileAndClose(fileStr)
	example.Summary(err, fileStr)
	// Output:
	// Successfully generated pdf/Fpdf_WrappedTableCells.pdf
}

// ExampleFpdf_SetJavascript demonstrates including JavaScript in the document.
func ExampleFpdf_SetJavascript() {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetJavascript("print(true);")
	pdf.AddPage()
	pdf.SetFont("Arial", "", 12)
	pdf.Write(10, "Auto-print.")
	fileStr := example.Filename("Fpdf_SetJavascript")
	err := pdf.OutputFileAndClose(fileStr)
	example.Summary(err, fileStr)
	// Output:
	// Successfully generated pdf/Fpdf_SetJavascript.pdf
}

// ExampleFpdf_AddSpotColor demonstrates spot color use
func ExampleFpdf_AddSpotColor() {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddSpotColor("PANTONE 145 CVC", 0, 42, 100, 25)
	pdf.AddPage()
	pdf.SetFillSpotColor("PANTONE 145 CVC", 90)
	pdf.Rect(80, 40, 50, 50, "F")
	fileStr := example.Filename("Fpdf_AddSpotColor")
	err := pdf.OutputFileAndClose(fileStr)
	example.Summary(err, fileStr)
	// Output:
	// Successfully generated pdf/Fpdf_AddSpotColor.pdf
}

// ExampleFpdf_RegisterAlias demonstrates how to use `RegisterAlias` to create a table of
// contents.
func ExampleFpdf_RegisterAlias() {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetFont("Arial", "", 12)
	pdf.AliasNbPages("")
	pdf.AddPage()

	// Write the table of contents. We use aliases instead of the page number
	// because we don't know which page the section will begin on.
	numSections := 3
	for i := 1; i <= numSections; i++ {
		pdf.Cell(0, 10, fmt.Sprintf("Section %d begins on page {mark %d}", i, i))
		pdf.Ln(10)
	}

	// Write the sections. Before we start writing, we use `RegisterAlias` to
	// ensure that the alias written in the table of contents will be replaced
	// by the current page number.
	for i := 1; i <= numSections; i++ {
		pdf.AddPage()
		pdf.RegisterAlias(fmt.Sprintf("{mark %d}", i), fmt.Sprintf("%d", pdf.PageNo()))
		pdf.Write(10, fmt.Sprintf("Section %d, page %d of {nb}", i, pdf.PageNo()))
	}

	fileStr := example.Filename("Fpdf_RegisterAlias")
	err := pdf.OutputFileAndClose(fileStr)
	example.Summary(err, fileStr)
	// Output:
	// Successfully generated pdf/Fpdf_RegisterAlias.pdf
}

// ExampleFpdf_RegisterAlias_utf8 demonstrates how to use `RegisterAlias` to
// create a table of contents. This particular example demonstrates the use of
// UTF-8 aliases.
func ExampleFpdf_RegisterAlias_utf8() {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddUTF8Font("dejavu", "", example.FontFile("DejaVuSansCondensed.ttf"))
	pdf.SetFont("dejavu", "", 12)
	pdf.AliasNbPages("{entute}")
	pdf.AddPage()

	// Write the table of contents. We use aliases instead of the page number
	// because we don't know which page the section will begin on.
	numSections := 3
	for i := 1; i <= numSections; i++ {
		pdf.Cell(0, 10, fmt.Sprintf("Sekcio %d komenciĝas ĉe paĝo {ĉi tiu marko %d}", i, i))
		pdf.Ln(10)
	}

	// Write the sections. Before we start writing, we use `RegisterAlias` to
	// ensure that the alias written in the table of contents will be replaced
	// by the current page number.
	for i := 1; i <= numSections; i++ {
		pdf.AddPage()
		pdf.RegisterAlias(fmt.Sprintf("{ĉi tiu marko %d}", i), fmt.Sprintf("%d", pdf.PageNo()))
		pdf.Write(10, fmt.Sprintf("Sekcio %d, paĝo %d de {entute}", i, pdf.PageNo()))
	}

	fileStr := example.Filename("Fpdf_RegisterAliasUTF8")
	err := pdf.OutputFileAndClose(fileStr)
	example.Summary(err, fileStr)
	// Output:
	// Successfully generated pdf/Fpdf_RegisterAliasUTF8.pdf
}

// ExampleNewGrid demonstrates the generation of graph grids.
func ExampleNewGrid() {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetFont("Arial", "", 12)
	pdf.AddPage()

	gr := gofpdf.NewGrid(13, 10, 187, 130)
	gr.TickmarksExtentX(0, 10, 4)
	gr.TickmarksExtentY(0, 10, 3)
	gr.Grid(pdf)

	gr = gofpdf.NewGrid(13, 154, 187, 128)
	gr.XLabelRotate = true
	gr.TickmarksExtentX(0, 1, 12)
	gr.XDiv = 5
	gr.TickmarksContainY(0, 1.1)
	gr.YDiv = 20
	// Replace X label formatter with month abbreviation
	gr.XTickStr = func(val float64, precision int) string {
		return time.Month(math.Mod(val, 12) + 1).String()[0:3]
	}
	gr.Grid(pdf)
	dot := func(x, y float64) {
		pdf.Circle(gr.X(x), gr.Y(y), 0.5, "F")
	}
	pts := []float64{0.39, 0.457, 0.612, 0.84, 0.998, 1.037, 1.015, 0.918, 0.772, 0.659, 0.593, 0.164}
	for month, val := range pts {
		dot(float64(month)+0.5, val)
	}
	pdf.SetDrawColor(255, 64, 64)
	pdf.SetAlpha(0.5, "Normal")
	pdf.SetLineWidth(1.2)
	gr.Plot(pdf, 0.5, 11.5, 50, func(x float64) float64 {
		// http://www.xuru.org/rt/PR.asp
		return 0.227 * math.Exp(-0.0373*x*x+0.471*x)
	})
	pdf.SetAlpha(1.0, "Normal")
	pdf.SetXY(gr.X(0.5), gr.Y(1.35))
	pdf.SetFontSize(14)
	pdf.Write(0, "Solar energy (MWh) per month, 2016")
	pdf.AddPage()

	gr = gofpdf.NewGrid(13, 10, 187, 274)
	gr.TickmarksContainX(2.3, 3.4)
	gr.TickmarksContainY(10.4, 56.8)
	gr.Grid(pdf)

	fileStr := example.Filename("Fpdf_Grid")
	err := pdf.OutputFileAndClose(fileStr)
	example.Summary(err, fileStr)
	// Output:
	// Successfully generated pdf/Fpdf_Grid.pdf
}

// ExampleFpdf_SetPageBox demonstrates the use of a page box
func ExampleFpdf_SetPageBox() {
	// pdfinfo (from http://www.xpdfreader.com) reports the following for this example:
	// ~ pdfinfo -box pdf/Fpdf_PageBox.pdf
	// Producer:       FPDF 1.7
	// CreationDate:   Sat Jan  1 00:00:00 2000
	// ModDate:   	   Sat Jan  1 00:00:00 2000
	// Tagged:         no
	// Form:           none
	// Pages:          1
	// Encrypted:      no
	// Page size:      493.23 x 739.85 pts (rotated 0 degrees)
	// MediaBox:           0.00     0.00   595.28   841.89
	// CropBox:           51.02    51.02   544.25   790.87
	// BleedBox:          51.02    51.02   544.25   790.87
	// TrimBox:           51.02    51.02   544.25   790.87
	// ArtBox:            51.02    51.02   544.25   790.87
	// File size:      1053 bytes
	// Optimized:      no
	// PDF version:    1.3
	const (
		wd        = 210
		ht        = 297
		fontsize  = 6
		boxmargin = 3 * fontsize
	)
	pdf := gofpdf.New("P", "mm", "A4", "") // 210mm x 297mm
	pdf.SetPageBox("crop", boxmargin, boxmargin, wd-2*boxmargin, ht-2*boxmargin)
	pdf.SetFont("Arial", "", pdf.UnitToPointConvert(fontsize))
	pdf.AddPage()
	pdf.MoveTo(fontsize, fontsize)
	pdf.Write(fontsize, "This will be cropped from printed output")
	pdf.MoveTo(boxmargin+fontsize, boxmargin+fontsize)
	pdf.Write(fontsize, "This will be displayed in cropped output")
	fileStr := example.Filename("Fpdf_PageBox")
	err := pdf.OutputFileAndClose(fileStr)
	example.Summary(err, fileStr)
	// Output:
	// Successfully generated pdf/Fpdf_PageBox.pdf
}

// ExampleFpdf_SubWrite demonstrates subscripted and superscripted text
// Adapted from http://www.fpdf.org/en/script/script61.php
func ExampleFpdf_SubWrite() {

	const (
		fontSize = 12
		halfX    = 105
	)

	pdf := gofpdf.New("P", "mm", "A4", "") // 210mm x 297mm
	pdf.AddPage()
	pdf.SetFont("Arial", "", fontSize)
	_, lineHt := pdf.GetFontSize()

	pdf.Write(lineHt, "Hello World!")
	pdf.SetX(halfX)
	pdf.Write(lineHt, "This is standard text.\n")
	pdf.Ln(lineHt * 2)

	pdf.SubWrite(10, "H", 33, 0, 0, "")
	pdf.Write(10, "ello World!")
	pdf.SetX(halfX)
	pdf.Write(10, "This is text with a capital first letter.\n")
	pdf.Ln(lineHt * 2)

	pdf.SubWrite(lineHt, "Y", 6, 0, 0, "")
	pdf.Write(lineHt, "ou can also begin the sentence with a small letter. And word wrap also works if the line is too long, like this one is.")
	pdf.SetX(halfX)
	pdf.Write(lineHt, "This is text with a small first letter.\n")
	pdf.Ln(lineHt * 2)

	pdf.Write(lineHt, "The world has a lot of km")
	pdf.SubWrite(lineHt, "2", 6, 4, 0, "")
	pdf.SetX(halfX)
	pdf.Write(lineHt, "This is text with a superscripted letter.\n")
	pdf.Ln(lineHt * 2)

	pdf.Write(lineHt, "The world has a lot of H")
	pdf.SubWrite(lineHt, "2", 6, -3, 0, "")
	pdf.Write(lineHt, "O")
	pdf.SetX(halfX)
	pdf.Write(lineHt, "This is text with a subscripted letter.\n")

	fileStr := example.Filename("Fpdf_SubWrite")
	err := pdf.OutputFileAndClose(fileStr)
	example.Summary(err, fileStr)
	// Output:
	// Successfully generated pdf/Fpdf_SubWrite.pdf
}

// ExampleFpdf_SetPage demomstrates the SetPage() method, allowing content
// generation to be deferred until all pages have been added.
func ExampleFpdf_SetPage() {
	rnd := rand.New(rand.NewSource(0)) // Make reproducible documents
	pdf := gofpdf.New("L", "cm", "A4", "")
	pdf.SetFont("Times", "", 12)

	var time []float64
	temperaturesFromSensors := make([][]float64, 5)
	maxs := []float64{25, 41, 89, 62, 11}
	for i := range temperaturesFromSensors {
		temperaturesFromSensors[i] = make([]float64, 0)
	}

	for i := 0.0; i < 100; i += 0.5 {
		time = append(time, i)
		for j, sensor := range temperaturesFromSensors {
			dataValue := rnd.Float64() * maxs[j]
			sensor = append(sensor, dataValue)
			temperaturesFromSensors[j] = sensor
		}
	}
	var graphs []gofpdf.GridType
	var pageNums []int
	xMax := time[len(time)-1]
	for i := range temperaturesFromSensors {
		//Create a new page and graph for each sensor we want to graph.
		pdf.AddPage()
		pdf.Ln(1)
		//Custom label per sensor
		pdf.WriteAligned(0, 0, "Temperature Sensor "+strconv.Itoa(i+1)+" (C) vs Time (min)", "C")
		pdf.Ln(0.5)
		graph := gofpdf.NewGrid(pdf.GetX(), pdf.GetY(), 20, 10)
		graph.TickmarksContainX(0, xMax)
		//Custom Y axis
		graph.TickmarksContainY(0, maxs[i])
		graph.Grid(pdf)
		//Save references and locations.
		graphs = append(graphs, graph)
		pageNums = append(pageNums, pdf.PageNo())
	}
	// For each X, graph the Y in each sensor.
	for i, currTime := range time {
		for j, sensor := range temperaturesFromSensors {
			pdf.SetPage(pageNums[j])
			graph := graphs[j]
			temperature := sensor[i]
			pdf.Circle(graph.X(currTime), graph.Y(temperature), 0.04, "D")
		}
	}

	fileStr := example.Filename("Fpdf_SetPage")
	err := pdf.OutputFileAndClose(fileStr)
	example.Summary(err, fileStr)
	// Output:
	// Successfully generated pdf/Fpdf_SetPage.pdf
}

// ExampleFpdf_SetFillColor demonstrates how graphic attributes are properly
// assigned within multiple transformations. See issue #234.
func ExampleFpdf_SetFillColor() {
	pdf := gofpdf.New("P", "mm", "A4", "")

	pdf.AddPage()
	pdf.SetFont("Arial", "", 8)

	draw := func(trX, trY float64) {
		pdf.TransformBegin()
		pdf.TransformTranslateX(trX)
		pdf.TransformTranslateY(trY)
		pdf.SetLineJoinStyle("round")
		pdf.SetLineWidth(0.5)
		pdf.SetDrawColor(128, 64, 0)
		pdf.SetFillColor(255, 127, 0)
		pdf.SetAlpha(0.5, "Normal")
		pdf.SetDashPattern([]float64{5, 10}, 0)
		pdf.Rect(0, 0, 40, 40, "FD")
		pdf.SetFontSize(12)
		pdf.SetXY(5, 5)
		pdf.Write(0, "Test")
		pdf.TransformEnd()
	}

	draw(5, 5)
	draw(50, 50)

	fileStr := example.Filename("Fpdf_SetFillColor")
	err := pdf.OutputFileAndClose(fileStr)
	example.Summary(err, fileStr)
	// Output:
	// Successfully generated pdf/Fpdf_SetFillColor.pdf
}

// ExampleFpdf_TransformRotate demonstrates how to rotate text within a header
// to make a watermark that appears on each page.
func ExampleFpdf_TransformRotate() {

	loremStr := lorem() + "\n\n"
	pdf := gofpdf.New("P", "mm", "A4", "")
	margin := 25.0
	pdf.SetMargins(margin, margin, margin)

	fontHt := 13.0
	lineHt := pdf.PointToUnitConvert(fontHt)
	markFontHt := 50.0
	markLineHt := pdf.PointToUnitConvert(markFontHt)
	markY := (297.0 - markLineHt) / 2.0
	ctrX := 210.0 / 2.0
	ctrY := 297.0 / 2.0

	pdf.SetHeaderFunc(func() {
		pdf.SetFont("Arial", "B", markFontHt)
		pdf.SetTextColor(206, 216, 232)
		pdf.SetXY(margin, markY)
		pdf.TransformBegin()
		pdf.TransformRotate(45, ctrX, ctrY)
		pdf.CellFormat(0, markLineHt, "W A T E R M A R K   D E M O", "", 0, "C", false, 0, "")
		pdf.TransformEnd()
		pdf.SetXY(margin, margin)
	})

	pdf.AddPage()
	pdf.SetFont("Arial", "", 8)
	for j := 0; j < 25; j++ {
		pdf.MultiCell(0, lineHt, loremStr, "", "L", false)
	}

	fileStr := example.Filename("Fpdf_RotateText")
	err := pdf.OutputFileAndClose(fileStr)
	example.Summary(err, fileStr)
	// Output:
	// Successfully generated pdf/Fpdf_RotateText.pdf
}

// ExampleFpdf_AddUTF8Font demonstrates how use the font
// with utf-8 mode
func ExampleFpdf_AddUTF8Font() {
	var fileStr string
	var txtStr []byte
	var err error

	pdf := gofpdf.New("P", "mm", "A4", "")

	pdf.AddPage()

	pdf.AddUTF8Font("dejavu", "", example.FontFile("DejaVuSansCondensed.ttf"))
	pdf.AddUTF8Font("dejavu", "B", example.FontFile("DejaVuSansCondensed-Bold.ttf"))
	pdf.AddUTF8Font("dejavu", "I", example.FontFile("DejaVuSansCondensed-Oblique.ttf"))
	pdf.AddUTF8Font("dejavu", "BI", example.FontFile("DejaVuSansCondensed-BoldOblique.ttf"))

	fileStr = example.Filename("Fpdf_AddUTF8Font")
	txtStr, err = ioutil.ReadFile(example.TextFile("utf-8test.txt"))
	if err == nil {

		pdf.SetFont("dejavu", "B", 17)
		pdf.MultiCell(100, 8, "Text in different languages :", "", "C", false)
		pdf.SetFont("dejavu", "", 14)
		pdf.MultiCell(100, 5, string(txtStr), "", "C", false)
		pdf.Ln(15)

		txtStr, err = ioutil.ReadFile(example.TextFile("utf-8test2.txt"))
		if err == nil {

			pdf.SetFont("dejavu", "BI", 17)
			pdf.MultiCell(100, 8, "Greek text with alignStr = \"J\":", "", "C", false)
			pdf.SetFont("dejavu", "I", 14)
			pdf.MultiCell(100, 5, string(txtStr), "", "J", false)
			err = pdf.OutputFileAndClose(fileStr)

		}
	}
	example.Summary(err, fileStr)
	// Output:
	// Successfully generated pdf/Fpdf_AddUTF8Font.pdf
}

// ExampleUTF8CutFont demonstrates how generate a TrueType font subset.
func ExampleUTF8CutFont() {
	var pdfFileStr, fullFontFileStr, subFontFileStr string
	var subFont, fullFont []byte
	var err error

	pdfFileStr = example.Filename("Fpdf_UTF8CutFont")
	fullFontFileStr = example.FontFile("calligra.ttf")
	fullFont, err = ioutil.ReadFile(fullFontFileStr)
	if err == nil {
		subFontFileStr = "calligra_abcde.ttf"
		subFont = gofpdf.UTF8CutFont(fullFont, "abcde")
		err = ioutil.WriteFile(subFontFileStr, subFont, 0600)
		if err == nil {
			y := 24.0
			pdf := gofpdf.New("P", "mm", "A4", "")
			fontHt := 17.0
			lineHt := pdf.PointConvert(fontHt)
			write := func(format string, args ...interface{}) {
				pdf.SetXY(24.0, y)
				pdf.Cell(200.0, lineHt, fmt.Sprintf(format, args...))
				y += lineHt
			}
			writeSize := func(fileStr string) {
				var info os.FileInfo
				var err error
				info, err = os.Stat(fileStr)
				if err == nil {
					write("%6d: size of %s", info.Size(), fileStr)
				}
			}
			pdf.AddPage()
			pdf.AddUTF8Font("calligra", "", subFontFileStr)
			pdf.SetFont("calligra", "", fontHt)
			write("cabbed")
			write("vwxyz")
			pdf.SetFont("courier", "", fontHt)
			writeSize(fullFontFileStr)
			writeSize(subFontFileStr)
			err = pdf.OutputFileAndClose(pdfFileStr)
			os.Remove(subFontFileStr)
		}
	}
	example.Summary(err, pdfFileStr)
	// Output:
	// Successfully generated pdf/Fpdf_UTF8CutFont.pdf
}

func ExampleFpdf_RoundedRect() {
	const (
		wd     = 40.0
		hgap   = 10.0
		radius = 10.0
		ht     = 60.0
		vgap   = 10.0
	)
	corner := func(b1, b2, b3, b4 bool) (cstr string) {
		if b1 {
			cstr = "1"
		}
		if b2 {
			cstr += "2"
		}
		if b3 {
			cstr += "3"
		}
		if b4 {
			cstr += "4"
		}
		return
	}
	pdf := gofpdf.New("P", "mm", "A4", "") // 210 x 297
	pdf.AddPage()
	pdf.SetLineWidth(0.5)
	y := vgap
	r := 40
	g := 30
	b := 20
	for row := 0; row < 4; row++ {
		x := hgap
		for col := 0; col < 4; col++ {
			pdf.SetFillColor(r, g, b)
			pdf.RoundedRect(x, y, wd, ht, radius, corner(row&1 == 1, row&2 == 2, col&1 == 1, col&2 == 2), "FD")
			r += 8
			g += 10
			b += 12
			x += wd + hgap
		}
		y += ht + vgap
	}
	pdf.AddPage()
	pdf.RoundedRectExt(10, 20, 40, 80, 4., 0., 20, 0., "FD")

	fileStr := example.Filename("Fpdf_RoundedRect")
	err := pdf.OutputFileAndClose(fileStr)
	example.Summary(err, fileStr)
	// Output:
	// Successfully generated pdf/Fpdf_RoundedRect.pdf
}

// ExampleFpdf_SetUnderlineThickness demonstrates how to adjust the text
// underline thickness.
func ExampleFpdf_SetUnderlineThickness() {
	pdf := gofpdf.New("P", "mm", "A4", "") // 210mm x 297mm
	pdf.AddPage()
	pdf.SetFont("Arial", "U", 12)

	pdf.SetUnderlineThickness(0.5)
	pdf.CellFormat(0, 10, "Thin underline", "", 1, "", false, 0, "")

	pdf.SetUnderlineThickness(1)
	pdf.CellFormat(0, 10, "Normal underline", "", 1, "", false, 0, "")

	pdf.SetUnderlineThickness(2)
	pdf.CellFormat(0, 10, "Thicker underline", "", 1, "", false, 0, "")

	fileStr := example.Filename("Fpdf_UnderlineThickness")
	err := pdf.OutputFileAndClose(fileStr)
	example.Summary(err, fileStr)
	// Output:
	// Successfully generated pdf/Fpdf_UnderlineThickness.pdf
}

// ExampleFpdf_Cell_strikeout demonstrates striked-out text
func ExampleFpdf_Cell_strikeout() {

	pdf := gofpdf.New("P", "mm", "A4", "") // 210mm x 297mm
	pdf.AddPage()

	for fontSize := 4; fontSize < 40; fontSize += 10 {
		pdf.SetFont("Arial", "S", float64(fontSize))
		pdf.SetXY(0, float64(fontSize))
		pdf.Cell(40, 10, "Hello World")
	}

	fileStr := example.Filename("Fpdf_Cell_strikeout")
	err := pdf.OutputFileAndClose(fileStr)
	example.Summary(err, fileStr)
	// Output:
	// Successfully generated pdf/Fpdf_Cell_strikeout.pdf
}

// ExampleFpdf_SetTextRenderingMode demonstrates rendering modes in PDFs.
func ExampleFpdf_SetTextRenderingMode() {

	pdf := gofpdf.New("P", "mm", "A4", "") // 210mm x 297mm
	pdf.AddPage()
	fontSz := float64(16)
	lineSz := pdf.PointToUnitConvert(fontSz)
	pdf.SetFont("Times", "", fontSz)
	pdf.Write(lineSz, "This document demonstrates various modes of text rendering. Search for \"Mode 3\" "+
		"to locate text that has been rendered invisibly. This selection can be copied "+
		"into the clipboard as usual and is useful for overlaying onto non-textual elements such "+
		"as images to make them searchable.\n\n")
	fontSz = float64(125)
	lineSz = pdf.PointToUnitConvert(fontSz)
	pdf.SetFontSize(fontSz)
	pdf.SetTextColor(170, 170, 190)
	pdf.SetDrawColor(50, 60, 90)

	write := func(mode int) {
		pdf.SetTextRenderingMode(mode)
		pdf.CellFormat(210, lineSz, fmt.Sprintf("Mode %d", mode), "", 1, "", false, 0, "")
	}

	for mode := 0; mode < 4; mode++ {
		write(mode)
	}
	write(0)

	fileStr := example.Filename("Fpdf_TextRenderingMode")
	err := pdf.OutputFileAndClose(fileStr)
	example.Summary(err, fileStr)
	// Output:
	// Successfully generated pdf/Fpdf_TextRenderingMode.pdf
}

// TestIssue0316 addresses issue 316 in which AddUTF8FromBytes modifies its argument
// utf8bytes resulting in a panic if you generate two PDFs with the "same" font bytes.
func TestIssue0316(t *testing.T) {
	pdf := gofpdf.New(gofpdf.OrientationPortrait, "mm", "A4", "")
	pdf.AddPage()
	fontBytes, _ := ioutil.ReadFile(example.FontFile("DejaVuSansCondensed.ttf"))
	ofontBytes := append([]byte{}, fontBytes...)
	pdf.AddUTF8FontFromBytes("dejavu", "", fontBytes)
	pdf.SetFont("dejavu", "", 16)
	pdf.Cell(40, 10, "Hello World!")
	fileStr := example.Filename("TestIssue0316")
	err := pdf.OutputFileAndClose(fileStr)
	example.Summary(err, fileStr)
	pdf.AddPage()
	if !bytes.Equal(fontBytes, ofontBytes) {
		t.Fatal("Font data changed during pdf generation")
	}
}

func TestMultiCellUnsupportedChar(t *testing.T) {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddPage()
	fontBytes, _ := ioutil.ReadFile(example.FontFile("DejaVuSansCondensed.ttf"))
	pdf.AddUTF8FontFromBytes("dejavu", "", fontBytes)
	pdf.SetFont("dejavu", "", 16)

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("unexpected panic: %v", r)
		}
	}()

	pdf.MultiCell(0, 5, "😀", "", "", false)

	fileStr := example.Filename("TestMultiCellUnsupportedChar")
	pdf.OutputFileAndClose(fileStr)
}

// ExampleFpdf_SetTextRenderingMode demonstrates embedding files in PDFs,
// at the top-level.
func ExampleFpdf_SetAttachments() {
	pdf := gofpdf.New("P", "mm", "A4", "")

	// Global attachments
	file, err := ioutil.ReadFile("grid.go")
	if err != nil {
		pdf.SetError(err)
	}
	a1 := gofpdf.Attachment{Content: file, Filename: "grid.go"}
	file, err = ioutil.ReadFile("LICENSE")
	if err != nil {
		pdf.SetError(err)
	}
	a2 := gofpdf.Attachment{Content: file, Filename: "License"}
	pdf.SetAttachments([]gofpdf.Attachment{a1, a2})

	fileStr := example.Filename("Fpdf_EmbeddedFiles")
	err = pdf.OutputFileAndClose(fileStr)
	example.Summary(err, fileStr)
	// Output:
	// Successfully generated pdf/Fpdf_EmbeddedFiles.pdf
}

func ExampleFpdf_AddAttachmentAnnotation() {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetFont("Arial", "", 12)
	pdf.AddPage()

	// Per page attachment
	file, err := ioutil.ReadFile("grid.go")
	if err != nil {
		pdf.SetError(err)
	}
	a := gofpdf.Attachment{Content: file, Filename: "grid.go", Description: "Some amazing code !"}

	pdf.SetXY(5, 10)
	pdf.Rect(2, 10, 50, 15, "D")
	pdf.AddAttachmentAnnotation(&a, 2, 10, 50, 15)
	pdf.Cell(50, 15, "A first link")

	pdf.SetXY(5, 80)
	pdf.Rect(2, 80, 50, 15, "D")
	pdf.AddAttachmentAnnotation(&a, 2, 80, 50, 15)
	pdf.Cell(50, 15, "A second link (no copy)")

	fileStr := example.Filename("Fpdf_FileAnnotations")
	err = pdf.OutputFileAndClose(fileStr)
	example.Summary(err, fileStr)
	// Output:
	// Successfully generated pdf/Fpdf_FileAnnotations.pdf
}

func ExampleFpdf_SetModificationDate() {
	// pdfinfo (from http://www.xpdfreader.com) reports the following for this example :
	// ~ pdfinfo -box pdf/Fpdf_PageBox.pdf
	// Producer:       FPDF 1.7
	// CreationDate:   Sat Jan  1 00:00:00 2000
	// ModDate:        Sun Jan  2 10:22:30 2000
	pdf := gofpdf.New("", "", "", "")
	pdf.AddPage()
	pdf.SetModificationDate(time.Date(2000, 1, 2, 10, 22, 30, 0, time.UTC))
	fileStr := example.Filename("Fpdf_SetModificationDate")
	err := pdf.OutputFileAndClose(fileStr)
	example.Summary(err, fileStr)
	// Output:
	// Successfully generated pdf/Fpdf_SetModificationDate.pdf
}
