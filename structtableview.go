// Copyright (c) 2018, The GoKi Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gi

import (
	"fmt"
	"log"
	"reflect"

	"github.com/goki/gi/units"
	"github.com/goki/ki"
	"github.com/goki/ki/kit"
)

////////////////////////////////////////////////////////////////////////////////////////
//  StructTableView

// StructTableView represents a slice of a struct as a table, where the fields are the columns, within an overall frame with an optional title, and a button box at the bottom where methods can be invoked
type StructTableView struct {
	Frame
	Slice   interface{}   `desc:"the slice that we are a view onto"`
	Title   string        `desc:"title / prompt to show above the editor fields"`
	Values  [][]ValueView `json:"-" xml:"-" desc:"ValueView representations of the slice field values -- outer dimension is fields, inner is rows (generally more rows than fields, so this minimizes number of slices allocated)"`
	TmpSave ValueView     `json:"-" xml:"-" desc:"value view that needs to have SaveTmp called on it whenever a change is made to one of the underlying values -- pass this down to any sub-views created from a parent"`
	ViewSig ki.Signal     `json:"-" xml:"-" desc:"signal for valueview -- only one signal sent when a value has been set -- all related value views interconnect with each other to update when others update"`
}

var KiT_StructTableView = kit.Types.AddType(&StructTableView{}, StructTableViewProps)

// Note: the overall strategy here is similar to Dialog, where we provide lots
// of flexible configuration elements that can be easily extended and modified

// SetSlice sets the source slice that we are viewing -- rebuilds the children
// to represent this slice
func (sv *StructTableView) SetSlice(sl interface{}, tmpSave ValueView) {
	updt := false
	if sv.Slice != sl {
		struTyp := reflect.TypeOf(sl).Elem()
		if struTyp.Kind() != reflect.Struct {
			log.Printf("StructTableView requires that you pass a slice of struct elements -- type is not a Struct: %v\n", struTyp.String())
			return
		}
		updt = sv.UpdateStart()
		sv.Slice = sl
	}
	sv.TmpSave = tmpSave
	sv.UpdateFromSlice()
	sv.UpdateEnd(updt)
}

var StructTableViewProps = ki.Props{
	"background-color": &Prefs.BackgroundColor,
	"#title": ki.Props{
		// todo: add "bigger" font
		"max-width":      units.NewValue(-1, units.Px),
		"text-align":     AlignCenter,
		"vertical-align": AlignTop,
	},
}

// SetFrame configures view as a frame
func (sv *StructTableView) SetFrame() {
	sv.Lay = LayoutCol
}

// StdFrameConfig returns a TypeAndNameList for configuring a standard Frame
// -- can modify as desired before calling ConfigChildren on Frame using this
func (sv *StructTableView) StdFrameConfig() kit.TypeAndNameList {
	config := kit.TypeAndNameList{} // note: slice is already a pointer
	// config.Add(KiT_Label, "title")
	// config.Add(KiT_Space, "title-space")
	config.Add(KiT_Layout, "slice-grid")
	config.Add(KiT_Space, "grid-space")
	config.Add(KiT_Layout, "buttons")
	return config
}

// StdConfig configures a standard setup of the overall Frame -- returns mods,
// updt from ConfigChildren and does NOT call UpdateEnd
func (sv *StructTableView) StdConfig() (mods, updt bool) {
	sv.SetFrame()
	config := sv.StdFrameConfig()
	mods, updt = sv.ConfigChildren(config, false)
	return
}

// SetTitle sets the title and updates the Title label
func (sv *StructTableView) SetTitle(title string) {
	sv.Title = title
	lab, _ := sv.TitleWidget()
	if lab != nil {
		lab.Text = title
	}
}

// Title returns the title label widget, and its index, within frame -- nil, -1 if not found
func (sv *StructTableView) TitleWidget() (*Label, int) {
	idx := sv.ChildIndexByName("title", 0)
	if idx < 0 {
		return nil, -1
	}
	return sv.Child(idx).(*Label), idx
}

// SliceGrid returns the SliceGrid grid layout widget, which contains all the fields and values, and its index, within frame -- nil, -1 if not found
func (sv *StructTableView) SliceGrid() (*Layout, int) {
	idx := sv.ChildIndexByName("slice-grid", 0)
	if idx < 0 {
		return nil, -1
	}
	return sv.Child(idx).(*Layout), idx
}

// ButtonBox returns the ButtonBox layout widget, and its index, within frame -- nil, -1 if not found
func (sv *StructTableView) ButtonBox() (*Layout, int) {
	idx := sv.ChildIndexByName("buttons", 0)
	if idx < 0 {
		return nil, -1
	}
	return sv.Child(idx).(*Layout), idx
}

// ConfigSliceGrid configures the SliceGrid for the current slice
func (sv *StructTableView) ConfigSliceGrid() {
	if kit.IfaceIsNil(sv.Slice) {
		return
	}
	mv := reflect.ValueOf(sv.Slice)
	mvnp := kit.NonPtrValue(mv)
	sz := mvnp.Len()

	// this is the type of element within slice -- already checked that it is a struct
	struTyp := reflect.TypeOf(sv.Slice).Elem()
	nfld := struTyp.NumField()

	// always start fresh!
	sv.Values = make([][]ValueView, nfld)
	for fli := 0; fli < nfld; fli++ {
		sv.Values[fli] = make([]ValueView, sz)
	}

	sg, _ := sv.SliceGrid()
	if sg == nil {
		return
	}
	sg.Lay = LayoutGrid
	sg.SetProp("columns", nfld+1)
	config := kit.TypeAndNameList{} // note: slice is already a pointer

	for i := 0; i < sz; i++ {
		val := kit.OnePtrValue(mvnp.Index(i)) // deal with pointer lists
		stru := val.Interface()
		idxtxt := fmt.Sprintf("%05d", i)
		labnm := fmt.Sprintf("index-%v", idxtxt)
		config.Add(KiT_Label, labnm)
		for fli := 0; fli < nfld; fli++ {
			fval := val.Elem().Field(fli)
			vv := ToValueView(fval.Interface())
			if vv == nil { // shouldn't happen
				continue
			}
			field := struTyp.Field(fli)
			vv.SetStructValue(fval.Addr(), stru, &field, sv.TmpSave)
			vtyp := vv.WidgetType()
			valnm := fmt.Sprintf("value-%v.%v", fli, idxtxt)
			config.Add(vtyp, valnm)
			sv.Values[fli][i] = vv
		}
		// addnm := fmt.Sprintf("add-%v", idxtxt)
		// delnm := fmt.Sprintf("del-%v", idxtxt)
		// config.Add(KiT_Action, addnm)
		// config.Add(KiT_Action, delnm)
	}
	mods, updt := sg.ConfigChildren(config, false)
	if mods {
		sv.SetFullReRender()
	} else {
		updt = sg.UpdateStart()
	}
	nWidgPerRow := nfld + 1
	for i := 0; i < sz; i++ {
		for fli := 0; fli < nfld; fli++ {
			vv := sv.Values[fli][i]
			// vvb := vv.AsValueViewBase()
			// vvb.ViewSig.ConnectOnly(sv.This, func(recv, send ki.Ki, sig int64, data interface{}) {
			// 	svv, _ := recv.EmbeddedStruct(KiT_StructTableView).(*StructTableView)
			// 	svv.UpdateSig()
			// 	svv.ViewSig.Emit(svv.This, 0, nil)
			// })
			lbl := sg.Child(i * nWidgPerRow).(*Label)
			lbl.SetProp("vertical-align", AlignMiddle)
			idxtxt := fmt.Sprintf("%05d", i)
			lbl.Text = idxtxt
			widg := sg.Child((i * nWidgPerRow) + 1 + fli).(Node2D)
			widg.SetProp("vertical-align", AlignMiddle)
			vv.ConfigWidget(widg)
			// addact := sg.Child(i*4 + 2).(*Action)
			// addact.SetProp("vertical-align", AlignMiddle)
			// addact.Text = " + "
			// addact.Data = i
			// addact.ActionSig.ConnectOnly(sv.This, func(recv, send ki.Ki, sig int64, data interface{}) {
			// 	act := send.(*Action)
			// 	svv := recv.EmbeddedStruct(KiT_StructTableView).(*StructTableView)
			// 	svv.SliceNewAt(act.Data.(int) + 1)
			// })
			// delact := sg.Child(i*4 + 3).(*Action)
			// delact.SetProp("vertical-align", AlignMiddle)
			// delact.Text = "  --"
			// delact.Data = i
			// delact.ActionSig.ConnectOnly(sv.This, func(recv, send ki.Ki, sig int64, data interface{}) {
			// 	act := send.(*Action)
			// 	svv := recv.EmbeddedStruct(KiT_StructTableView).(*StructTableView)
			// 	svv.SliceDelete(act.Data.(int))
			// })
		}
	}
	sg.UpdateEnd(updt)
}

// SliceNewAt inserts a new blank element at given index in the slice -- -1 means the end
func (sv *StructTableView) SliceNewAt(idx int) {
	updt := sv.UpdateStart()
	svl := reflect.ValueOf(sv.Slice)
	svnp := kit.NonPtrValue(svl)
	svtyp := svnp.Type()
	nval := reflect.New(svtyp.Elem())
	sz := svnp.Len()
	svnp = reflect.Append(svnp, nval.Elem())
	if idx >= 0 && idx < sz-1 {
		reflect.Copy(svnp.Slice(idx+1, sz+1), svnp.Slice(idx, sz))
		svnp.Index(idx).Set(nval.Elem())
	}
	svl.Elem().Set(svnp)
	if sv.TmpSave != nil {
		sv.TmpSave.SaveTmp()
	}
	sv.SetFullReRender()
	sv.UpdateEnd(updt)
	sv.ViewSig.Emit(sv.This, 0, nil)
}

// SliceDelete deletes element at given index from slice
func (sv *StructTableView) SliceDelete(idx int) {
	updt := sv.UpdateStart()
	svl := reflect.ValueOf(sv.Slice)
	svnp := kit.NonPtrValue(svl)
	svtyp := svnp.Type()
	nval := reflect.New(svtyp.Elem())
	sz := svnp.Len()
	reflect.Copy(svnp.Slice(idx, sz-1), svnp.Slice(idx+1, sz))
	svnp.Index(sz - 1).Set(nval.Elem())
	svl.Elem().Set(svnp.Slice(0, sz-1))
	if sv.TmpSave != nil {
		sv.TmpSave.SaveTmp()
	}
	sv.SetFullReRender()
	sv.UpdateEnd(updt)
	sv.ViewSig.Emit(sv.This, 0, nil)
}

// ConfigSliceButtons configures the buttons for map functions
func (sv *StructTableView) ConfigSliceButtons() {
	if kit.IfaceIsNil(sv.Slice) {
		return
	}
	bb, _ := sv.ButtonBox()
	config := kit.TypeAndNameList{} // note: slice is already a pointer
	config.Add(KiT_Button, "Add")
	mods, updt := bb.ConfigChildren(config, false)
	addb := bb.ChildByName("Add", 0).EmbeddedStruct(KiT_Button).(*Button)
	addb.SetText("Add")
	addb.ButtonSig.ConnectOnly(sv.This, func(recv, send ki.Ki, sig int64, data interface{}) {
		if sig == int64(ButtonClicked) {
			svv := recv.EmbeddedStruct(KiT_StructTableView).(*StructTableView)
			svv.SliceNewAt(-1)
		}
	})
	if mods {
		bb.UpdateEnd(updt)
	}
}

func (sv *StructTableView) UpdateFromSlice() {
	mods, updt := sv.StdConfig()
	// typ := kit.NonPtrType(reflect.TypeOf(sv.Slice))
	// sv.SetTitle(fmt.Sprintf("%v Values", typ.Name()))
	sv.ConfigSliceGrid()
	sv.ConfigSliceButtons()
	if mods {
		sv.UpdateEnd(updt)
	}
}

func (sv *StructTableView) UpdateValues() {
	updt := sv.UpdateStart()
	for _, vv := range sv.Values {
		for _, vvf := range vv {
			vvf.UpdateWidget()
		}
	}
	sv.UpdateEnd(updt)
}

// needs full rebuild and this is where we do it:
func (sv *StructTableView) Style2D() {
	sv.ConfigSliceGrid()
	sv.Frame.Style2D()
}

func (sv *StructTableView) Render2D() {
	sv.ClearFullReRender()
	sv.Frame.Render2D()
}

func (sv *StructTableView) ReRender2D() (node Node2D, layout bool) {
	if sv.NeedsFullReRender() {
		node = nil
		layout = false
	} else {
		node = sv.This.(Node2D)
		layout = true
	}
	return
}