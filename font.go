// Copyright (c) 2018, The GoKi Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gi

import (
	"fmt"
	"image/color"
	"io/ioutil"
	"log"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/chewxy/math32"
	"github.com/goki/freetype/truetype"
	// "github.com/golang/freetype/truetype"
	"github.com/goki/gi/units"
	"github.com/goki/ki"
	"github.com/goki/ki/bitflag"
	"github.com/goki/ki/kit"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/font/sfnt"
)

// font.go contains all font and basic SVG-level text rendering styles, and the
// font library.  see text.go for rendering code

// FontStyle contains all font styling information, including everything that
// is used in SVG text rendering -- used in Paint and in Style -- see style.go
// -- most of font information is inherited
type FontStyle struct {
	Color    Color           `xml:"color" inherit:"true" desc:"text color -- also defines the currentColor variable value"`
	BgColor  ColorSpec       `xml:"background-color" desc:"background color -- not inherited, transparent by default"`
	Opacity  float32         `xml:"opacity" desc:"alpha value to apply to all elements"`
	Size     units.Value     `xml:"font-size" desc:"size of font to render -- convert to points when getting font to use"`
	Family   string          `xml:"font-family" inherit:"true" desc:"font family -- ordered list of names from more general to more specific to use -- use split on , to parse"`
	Style    FontStyles      `xml:"font-style" inherit:"true" desc:"style -- normal, italic, etc"`
	Weight   FontWeights     `xml:"font-weight" inherit:"true" desc:"weight: normal, bold, etc"`
	Stretch  FontStretch     `xml:"font-stretch" inherit:"true" desc:"font stretch / condense options"`
	Variant  FontVariants    `xml:"font-variant" inherit:"true" desc:"normal or small caps"`
	Deco     TextDecorations `xml:"text-decoration" desc:"underline, line-through, etc -- not inherited"`
	Shift    BaselineShifts  `xml:"baseline-shift" desc:"super / sub script -- not inherited"`
	Face     font.Face       `view:"-" desc:"actual font codes for drawing text -- just a pointer into FontLibrary of loaded fonts"`
	Height   float32         `desc:"reference 1.0 spacing line height of font in dots -- computed from font as ascent + descent + lineGap, where lineGap is specified by the font as the recommended line spacing"`
	Em       float32         `desc:"Em size of font -- this is NOT actually the width of the letter M, but rather the specified point size of the font (in actual display dots, not points) -- it does NOT include the descender and will not fit the entire height of the font"`
	Ex       float32         `desc:"Ex size of font -- this is the actual height of the letter x in the font"`
	Ch       float32         `desc:"Ch size of font -- this is the actual width of the 0 glyph in the font"`
	Rem      float32         `desc:"Rem size of font -- 12pt converted to same effective DPI as above measurements"`
	FaceName string          `desc:"full name of font face as loaded -- computed based on Family, Style, Weight, etc"`
	// todo: kerning
	// todo: stretch -- css 3 -- not supported
}

func (fs *FontStyle) Defaults() {
	fs.Color.SetColor(color.Black)
	fs.Opacity = 1.0
	fs.FaceName = "Arial"
	fs.Size = units.NewValue(12, units.Pt)
}

// SetStylePost does any updates after generic xml-tag property setting -- use
// for anything that also has non-standard values that might not be processed
// properly by default
func (fs *FontStyle) SetStylePost(props ki.Props) {
	if pfs, ok := props["font-size"]; ok {
		if fsz, ok := pfs.(string); ok {
			if psz, ok := FontSizePoints[fsz]; ok {
				fs.Size = units.NewValue(psz, units.Pt)
			}
		}
	}
	if tds, ok := props["text-decoration"]; ok {
		if td, ok := tds.(string); ok {
			if td == "none" {
				fs.Deco = DecoNone // otherwise get a bit flag set
			}
		}
	}
}

// SetDeco sets decoration (underline, etc), which uses bitflag to allow multiple combinations
func (fs *FontStyle) SetDeco(deco TextDecorations) {
	bitflag.Set32((*int32)(&fs.Deco), int(deco))
}

// ClearDeco clears decoration (underline, etc), which uses bitflag to allow multiple combinations
func (fs *FontStyle) ClearDeco(deco TextDecorations) {
	bitflag.Clear32((*int32)(&fs.Deco), int(deco))
}

// FaceNm returns the full FaceName to use for the current FontStyle spec
func (fs *FontStyle) FaceNm() string {
	fnm := Prefs.FontFamily
	if fnm == "" {
		fnm = "Arial"
	}
	nms := strings.Split(fs.Family, ",")
	for _, fn := range nms {
		fn = strings.TrimSpace(fn)
		if FontLibrary.FontAvail(fn) {
			fnm = fn
			break
		}
		switch fn {
		case "times":
			fnm = "Times New Roman"
			break
		case "serif":
			fnm = "Times New Roman"
			break
		case "sans-serif":
			fnm = "Arial"
			break
		case "courier":
			fnm = "Courier New" // this is the tt name
			break
		case "monospace":
			if FontLibrary.FontAvail("Andale Mono") {
				fnm = "Andale Mono"
			} else {
				fnm = "Courier New"
			}
			break
		case "cursive":
			if FontLibrary.FontAvail("Comic Sans") {
				fnm = "Comic Sans"
			} else if FontLibrary.FontAvail("Comic Sans MS") {
				fnm = "Comic Sans MS"
			}
			break
		case "fantasy":
			if FontLibrary.FontAvail("Impact") {
				fnm = "Impact"
			} else if FontLibrary.FontAvail("Impac") {
				fnm = "Impac"
			}
			break
		}
	}
	mods := ""
	if fs.Style == FontItalic && fs.Weight == WeightBold {
		if !strings.Contains(fnm, "Bold") {
			mods += "Bold "
		}
		if !strings.Contains(fnm, "Italic") {
			mods += "Italic"
		}
	} else if fs.Style == FontOblique && fs.Weight == WeightBold {
		if !strings.Contains(fnm, "Bold") {
			mods += "Bold "
		}
		if !strings.Contains(fnm, "Oblique") {
			mods += "Oblique"
		}
	} else if fs.Style == FontItalic {
		if !strings.Contains(fnm, "Italic") {
			mods += "Italic"
		}
	} else if fs.Style == FontOblique {
		if !strings.Contains(fnm, "Oblique") {
			mods += "Obqlique"
		}
	} else if fs.Weight == WeightBold {
		if !strings.Contains(fnm, "Bold") {
			mods += "Bold "
		}
	}
	if mods != "" {
		fmod := fnm + " " + strings.TrimSpace(mods)
		if FontLibrary.FontAvail(fmod) {
			fnm = fmod
		} else {
			log.Printf("could not find modified font name: %v\n", fmod)
		}
	}
	return fnm
}

func (fs *FontStyle) LoadFont(ctxt *units.Context, fallback string) {
	fs.FaceName = fs.FaceNm()
	intDots := math.Round(float64(fs.Size.Dots))
	if intDots == 0 {
		intDots = 12
	}
	face, err := FontLibrary.Font(fs.FaceName, intDots)
	if err != nil {
		log.Printf("%v\n", err)
		if fs.Face == nil {
			if fallback != "" {
				fs.FaceName = fallback
				fs.LoadFont(ctxt, "") // try again
			} else {
				//				log.Printf("FontStyle LoadFont() -- Falling back on basicfont\n")
				fs.Face = basicfont.Face7x13
			}
		}
	} else {
		fs.Face = face
	}
	fs.ComputeMetrics(ctxt)
	fs.SetUnitContext(ctxt)
}

func (fs *FontStyle) ComputeMetrics(ctxt *units.Context) {
	if fs.Face == nil {
		return
	}
	intDots := float32(math.Round(float64(fs.Size.Dots)))
	if intDots == 0 {
		intDots = 12
	}
	// apd := fs.Face.Metrics().Ascent + fs.Face.Metrics().Descent
	fs.Height = math32.Ceil(FixedToFloat32(fs.Face.Metrics().Height))
	fs.Em = intDots // conventional definition
	xb, _, ok := fs.Face.GlyphBounds('x')
	if ok {
		fs.Ex = FixedToFloat32(xb.Max.Y - xb.Min.Y)
	} else {
		fs.Ex = 0.5 * fs.Em
	}
	xb, _, ok = fs.Face.GlyphBounds('0')
	if ok {
		fs.Ch = FixedToFloat32(xb.Max.X - xb.Min.X)
	} else {
		fs.Ch = 0.5 * fs.Em
	}
	fs.Rem = ctxt.ToDots(12, units.Pt)
	// fmt.Printf("fs: %v sz: %v\t\tHt: %v\tEm: %v\tEx: %v\tCh: %v\n", fs.FaceName, intDots, fs.Height, fs.Em, fs.Ex, fs.Ch)
}

func (fs *FontStyle) SetUnitContext(ctxt *units.Context) {
	if fs.Face != nil {
		ctxt.SetFont(fs.Em, fs.Ex, fs.Ch, fs.Rem)
	}
}

// Style CSS looks for "tag" name props in cssAgg props, and applies those to
// style if found, and returns true -- false if no such tag found
func (fs *FontStyle) StyleCSS(tag string, cssAgg ki.Props, ctxt *units.Context) bool {
	if cssAgg == nil {
		return false
	}
	tp, ok := cssAgg[tag]
	if !ok {
		return false
	}
	pmap, ok := tp.(ki.Props) // must be a props map
	if !ok {
		return false
	}
	fs.SetStyleProps(nil, pmap)
	fs.LoadFont(ctxt, "")
	return true
}

// SetStyleProps sets font style values based on given property map (name:
// value pairs), inheriting elements as appropriate from parent, and also
// having a default style for the "initial" setting
func (fs *FontStyle) SetStyleProps(parent *FontStyle, props ki.Props) {
	// direct font styling is used only for special cases -- don't do this:
	// if !fs.StyleSet && parent != nil { // first time
	// 	FontStyleFields.Inherit(fs, parent)
	// }
	FontStyleFields.Style(fs, parent, props)
	fs.SetStylePost(props)
}

// ToDots calls ToDots on all units.Value fields in the style (recursively)
func (fs *FontStyle) ToDots(ctxt *units.Context) {
	FontStyleFields.ToDots(fs, ctxt)
}

// FontStyleDefault is default style can be used when property specifies "default"
var FontStyleDefault FontStyle

// FontStyleFields contain the StyledFields for FontStyle type
var FontStyleFields = initFontStyle()

func initFontStyle() *StyledFields {
	FontStyleDefault.Defaults()
	sf := &StyledFields{}
	sf.Init(&FontStyleDefault)
	return sf
}

//////////////////////////////////////////////////////////////////////////////////
// Font Style enums

// FontSizePoints maps standard font names to standard point sizes -- we use
// dpi zoom scaling instead of rescaling "medium" font size, so generally use
// these values as-is.  smaller and larger relative scaling can move in 2pt increments
var FontSizePoints = map[string]float32{
	"xx-small": 7,
	"x-small":  7.5,
	"small":    10, // small is also "smaller"
	"smallf":   10, // smallf = small font size..
	"medium":   12,
	"large":    14,
	"x-large":  18,
	"xx-large": 24,
}

// FontStyles styles of font: normal, italic, etc
type FontStyles int32

const (
	FontNormal FontStyles = iota
	FontItalic
	FontOblique
	FontStylesN
)

//go:generate stringer -type=FontStyles

var KiT_FontStyles = kit.Enums.AddEnumAltLower(FontStylesN, false, StylePropProps, "Font")

func (ev FontStyles) MarshalJSON() ([]byte, error)  { return kit.EnumMarshalJSON(ev) }
func (ev *FontStyles) UnmarshalJSON(b []byte) error { return kit.EnumUnmarshalJSON(ev, b) }

// FontWeights styles of font: normal, italic, etc
type FontWeights int32

const (
	WeightNormal FontWeights = iota
	WeightBold
	WeightBolder
	WeightLighter
	Weight100
	Weight200
	Weight300
	Weight400 // normal
	Weight500
	Weight600
	Weight700
	Weight800
	Weight900 // bold
	FontWeightsN
)

//go:generate stringer -type=FontWeights

var KiT_FontWeights = kit.Enums.AddEnumAltLower(FontWeightsN, false, StylePropProps, "Weight")

func (ev FontWeights) MarshalJSON() ([]byte, error)  { return kit.EnumMarshalJSON(ev) }
func (ev *FontWeights) UnmarshalJSON(b []byte) error { return kit.EnumUnmarshalJSON(ev, b) }

// FontVariants is just normal vs. small caps. todo: not currently supported
type FontVariants int32

const (
	FontVarNormal FontVariants = iota
	FontVarSmallCaps
	FontVariantsN
)

//go:generate stringer -type=FontVariants

var KiT_FontVariants = kit.Enums.AddEnumAltLower(FontVariantsN, false, StylePropProps, "FontVar")

func (ev FontVariants) MarshalJSON() ([]byte, error)  { return kit.EnumMarshalJSON(ev) }
func (ev *FontVariants) UnmarshalJSON(b []byte) error { return kit.EnumUnmarshalJSON(ev, b) }

// FontStretch are different stretch levels of font.  todo: not currently supported
type FontStretch int32

const (
	FontStrNormal FontStretch = iota
	FontStrWider
	FontStrNarrower
	FontStrUltraCondensed
	FontStrExtraCondensed
	FontStrCondensed
	FontStrSemiCondensed
	FontStrSemiExpanded
	FontStrExpanded
	FontStrExtraExpanded
	FontStrUltraExpanded
	FontStretchN
)

//go:generate stringer -type=FontStretch

var KiT_FontStretch = kit.Enums.AddEnumAltLower(FontStretchN, false, StylePropProps, "FontStr")

func (ev FontStretch) MarshalJSON() ([]byte, error)  { return kit.EnumMarshalJSON(ev) }
func (ev *FontStretch) UnmarshalJSON(b []byte) error { return kit.EnumUnmarshalJSON(ev, b) }

// TextDecorations are underline, line-through, etc -- operates as bit flags
// -- also used for additional layout hints for RuneRender
type TextDecorations int32

const (
	DecoNone TextDecorations = iota
	DecoUnderline
	DecoOverline
	DecoLineThrough
	// Blink is not currently supported (and probably a bad idea generally ;)
	DecoBlink

	// DottedUnderline is used for abbr tag -- otherwise not a standard text-decoration option afaik
	DecoDottedUnderline

	// following are special case layout hints in RuneRender, to pass
	// information from a styling pass to a subsequent layout pass -- they are
	// NOT processed during final rendering

	// DecoParaStart at start of a SpanRender indicates that it should be
	// styled as the start of a new paragraph and not just the start of a new
	// line
	DecoParaStart
	// DecoSuper indicates super-scripted text
	DecoSuper
	// DecoSub indicates sub-scripted text
	DecoSub
	// DecoBgColor indicates that a bg color has been set -- for use in optimizing rendering
	DecoBgColor
	TextDecorationsN
)

//go:generate stringer -type=TextDecorations

var KiT_TextDecorations = kit.Enums.AddEnumAltLower(TextDecorationsN, true, StylePropProps, "Deco") // true = bit flag

func (ev TextDecorations) MarshalJSON() ([]byte, error)  { return kit.EnumMarshalJSON(ev) }
func (ev *TextDecorations) UnmarshalJSON(b []byte) error { return kit.EnumUnmarshalJSON(ev, b) }

// BaselineShifts are for super / sub script
type BaselineShifts int32

const (
	ShiftBaseline BaselineShifts = iota
	ShiftSuper
	ShiftSub
	BaselineShiftsN
)

//go:generate stringer -type=BaselineShifts

var KiT_BaselineShifts = kit.Enums.AddEnumAltLower(BaselineShiftsN, false, StylePropProps, "Shift")

func (ev BaselineShifts) MarshalJSON() ([]byte, error)  { return kit.EnumMarshalJSON(ev) }
func (ev *BaselineShifts) UnmarshalJSON(b []byte) error { return kit.EnumUnmarshalJSON(ev, b) }

//////////////////////////////////////////////////////////////////////////////////
// Font library

type FontInfo struct {
	Name    string      `desc:"name of font"`
	Style   FontStyles  `xml:"style" inherit:"true" desc:"style -- normal, italic, etc"`
	Weight  FontWeights `xml:"weight" inherit:"true" desc:"weight: normal, bold, etc"`
	Example string      `desc:"example text -- styled according to font params in chooser"`
}

type FontLib struct {
	FontPaths  []string
	FontsAvail map[string]string `desc:"map of font name to path to file"`
	FontInfo   []FontInfo        `desc:"information about each font"`
	Faces      map[string]map[float64]font.Face
	initMu     sync.Mutex
	loadMu     sync.Mutex
}

// FontLibrary is the gi font library, initialized from fonts available on font paths
var FontLibrary FontLib

// AltFontMap is an alternative font map that maps file names to more standard
// full names (e.g., Times -> Times New Roman) -- also looks for b,i suffixes
// for these cases -- some are added here just to pick up those suffixes
var AltFontMap = map[string]string{
	"arial":   "Arial",
	"ariblk":  "Arial Black",
	"candara": "Candara",
	"calibri": "Calibri",
	"cambria": "Cambria",
	"cour":    "Courier New",
	"constan": "Constantia",
	"consola": "Console",
	"comic":   "Comic Sans MS",
	"corbel":  "Corbel",
	"framd":   "Franklin Gothic Medium",
	"georgia": "Georgia",
	"gadugi":  "Gadugi",
	"malgun":  "Malgun Gothic",
	"mmrtex":  "Myanmar Text",
	"pala":    "Palatino",
	"segoepr": "Segoe Print",
	"segoesc": "Segoe Script",
	"segoeui": "Segoe UI",
	"segui":   "Segoe UI Historic",
	"tahoma":  "Tahoma",
	"taile":   "Traditional Arabic",
	"times":   "Times New Roman",
	"trebuc":  "Trebuchet",
	"verdana": "Verdana",
}

func (fl *FontLib) Init() {
	fl.initMu.Lock()
	if fl.FontPaths == nil {
		// fmt.Printf("Initializing font lib\n")
		fl.FontPaths = make([]string, 0, 1000)
		fl.FontsAvail = make(map[string]string)
		fl.FontInfo = make([]FontInfo, 0, 1000)
		fl.Faces = make(map[string]map[float64]font.Face)
	} else if len(fl.FontsAvail) == 0 {
		fmt.Printf("updating fonts avail in %v\n", fl.FontPaths)
		fl.UpdateFontsAvail()
	}
	fl.initMu.Unlock()
}

// InitFontPaths initializes font paths to system defaults, only if no paths
// have yet been set
func (fl *FontLib) InitFontPaths(paths ...string) {
	if len(fl.FontPaths) > 0 {
		return
	}
	fl.AddFontPaths(paths...)
}

func (fl *FontLib) AddFontPaths(paths ...string) bool {
	fl.Init()
	for _, p := range paths {
		fl.FontPaths = append(fl.FontPaths, p)
	}
	return fl.UpdateFontsAvail()
}

// UpdateFontsAvail scans for all fonts we can use on the FontPaths
func (fl *FontLib) UpdateFontsAvail() bool {
	if len(fl.FontPaths) == 0 {
		log.Print("gi.FontLib: no font paths -- need to add some\n")
		return false
	}
	if len(fl.FontsAvail) > 0 {
		fl.FontsAvail = make(map[string]string)
	}
	for _, p := range fl.FontPaths {
		fl.FontsAvailFromPath(p)
	}
	sort.Slice(fl.FontInfo, func(i, j int) bool {
		return fl.FontInfo[i].Name < fl.FontInfo[j].Name
	})

	return len(fl.FontsAvail) > 0
}

// FontMods are standard font modifiers
var FontMods = [...]string{"Bold", "Italic", "Oblique"}

// SpaceFontMods ensures that standard font modifiers have a space in front of them
func SpaceFontMods(fn string) string {
	for _, mod := range FontMods {
		if bi := strings.Index(fn, mod); bi > 0 {
			if fn[bi-1] != ' ' {
				fn = strings.Replace(fn, mod, " "+mod, 1)
			}
		}
	}
	return fn
}

var FontExts = map[string]bool{
	".ttf": true,
	".ttc": true,
	//	".otf": true,  // not yet supported
}

// FontsAvailFromPath scans for all fonts we can use on a given path,
// gathering info into FontsAvail and FontInfo.
func (fl *FontLib) FontsAvailFromPath(path string) error {

	err := filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			fmt.Printf("gi.FontLib: error accessing path %q: %v\n", path, err)
			return err
		}
		ext := strings.ToLower(filepath.Ext(path))
		_, ok := FontExts[ext]
		if !ok {
			return nil
		}
		_, fn := filepath.Split(path)
		fn = fn[:len(fn)-len(ext)]
		bfn := fn
		bfn = strings.TrimSuffix(fn, "bd")
		bfn = strings.TrimSuffix(bfn, "bi")
		bfn = strings.TrimSuffix(bfn, "z")
		bfn = strings.TrimSuffix(bfn, "b")
		if bfn != "calibri" && bfn != "gadugui" && bfn != "segoeui" && bfn != "segui" {
			bfn = strings.TrimSuffix(bfn, "i")
		}
		if afn, ok := AltFontMap[bfn]; ok {
			sfx := ""
			if strings.HasSuffix(fn, "bd") || strings.HasSuffix(fn, "b") {
				sfx = " Bold"
			} else if strings.HasSuffix(fn, "bi") || strings.HasSuffix(fn, "z") {
				sfx = " Bold Italic"
			} else if strings.HasSuffix(fn, "i") {
				sfx = " Italic"
			}
			fn = afn + sfx
		} else {
			fn = strings.Replace(fn, "_", " ", -1)
			fn = strings.Replace(fn, "-", " ", -1)
			// fn = strings.Title(fn)
		}
		fn = strings.TrimSuffix(fn, " Regular")
		// all std modifiers should have space before them
		fn = SpaceFontMods(fn)
		basefn := strings.ToLower(fn)
		if _, ok := fl.FontsAvail[basefn]; !ok {
			fl.FontsAvail[basefn] = path
			fi := FontInfo{Name: fn, Style: FontNormal, Weight: WeightNormal, Example: FontInfoExample}
			if strings.Contains(basefn, "bold") {
				fi.Weight = WeightBold
			}
			if strings.Contains(basefn, "italic") {
				fi.Style = FontItalic
			} else if strings.Contains(basefn, "oblique") {
				fi.Style = FontOblique
			}
			fl.FontInfo = append(fl.FontInfo, fi)
			// fmt.Printf("added font: %v at path %q\n", basefn, path)

		}
		return nil
	})
	if err != nil {
		log.Printf("gi.FontLib: error walking the path %q: %v\n", path, err)
	}
	return err
}

// Font gets a particular font
func (fl *FontLib) Font(fontnm string, points float64) (font.Face, error) {
	fontnm = strings.ToLower(fontnm)
	fl.Init()
	if facemap := fl.Faces[fontnm]; facemap != nil {
		if face := facemap[points]; face != nil {
			// fmt.Printf("Got font face from cache: %v %v\n", fontnm, points)
			return face, nil
		}
	}
	if path := fl.FontsAvail[fontnm]; path != "" {
		face, err := LoadFontFace(path, points)
		if err != nil {
			log.Printf("gi.FontLib: error loading font %v, removed from list\n", fontnm)
			delete(fl.FontsAvail, fontnm)
			return nil, err
		}
		fl.loadMu.Lock()
		facemap := fl.Faces[fontnm]
		if facemap == nil {
			facemap = make(map[float64]font.Face)
			fl.Faces[fontnm] = facemap
		}
		facemap[points] = face
		// fmt.Printf("Loaded font face: %v %v\n", fontnm, points)
		fl.loadMu.Unlock()
		return face, nil
	}
	return nil, fmt.Errorf("gi.FontLib: Font named: %v not found in list of available fonts, try adding to FontPaths in gi.FontLibrary, searched paths: %v\n", fontnm, fl.FontPaths)
}

func LoadFontFace(path string, points float64) (font.Face, error) {
	fontBytes, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".otf" {
		// note: this compiles but otf fonts are NOT yet supported apparently
		f, err := sfnt.Parse(fontBytes)
		if err != nil {
			return nil, err
		}
		face, err := opentype.NewFace(f, &opentype.FaceOptions{
			Size: points,
			// Hinting: font.HintingFull,
		})
		return face, err
	} else {
		f, err := truetype.Parse(fontBytes)
		if err != nil {
			return nil, err
		}
		face := truetype.NewFace(f, &truetype.Options{
			Size: points,
			// Hinting: font.HintingFull,
			GlyphCacheEntries: 1024, // default is 512 -- todo benchmark
			// Stroke:            1, // todo: cool stroking from tdewolff -- add to svg options
		})
		return face, nil
	}
}

// FontAvail determines if a given font name is available (case insensitive)
func (fl *FontLib) FontAvail(fontnm string) bool {
	fontnm = strings.ToLower(fontnm)
	_, ok := FontLibrary.FontsAvail[fontnm]
	return ok
}

// FontInfoExample is example text to demonstrate fonts -- from Inkscape plus extra
var FontInfoExample = "AaBbCcIiPpQq12369$€¢?.:/()àáâãäåæç日本中国⇧⌘"

// todo: https://blog.golang.org/go-fonts
