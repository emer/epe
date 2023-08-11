// Copyright (c) 2019, The Emergent Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package eve2d

import (
	"fmt"
	"image"

	"github.com/emer/eve/eve"
	"github.com/goki/gi/svg"
	"github.com/goki/ki/ki"
	"github.com/goki/ki/kit"
	"github.com/goki/mat32"
)

// View connects a Virtual World with a 2D SVG Scene to visualize the world
type View struct {

	// width of lines for shape rendering, in normalized units
	LineWidth float32 `desc:"width of lines for shape rendering, in normalized units"`

	// projection matrix for converting 3D to 2D -- resulting X, Y coordinates are used from Vec3
	Prjn mat32.Mat4 `desc:"projection matrix for converting 3D to 2D -- resulting X, Y coordinates are used from Vec3"`

	// the root Group node of the virtual world
	World *eve.Group `desc:"the root Group node of the virtual world"`

	// the SVG rendering canvas for visualizing in 2D
	Scene *svg.SVG `desc:"the SVG rendering canvas for visualizing in 2D"`

	// the root Group node in the Scene under which the world is rendered
	Root *svg.Group `desc:"the root Group node in the Scene under which the world is rendered"`

	// library of shapes for bodies -- name matches Body.Vis
	Library map[string]*svg.Group `desc:"library of shapes for bodies -- name matches Body.Vis"`
}

var KiT_View = kit.Types.AddType(&View{}, nil)

// NewView returns a new View that links given world with given scene and root group
func NewView(world *eve.Group, sc *svg.SVG, root *svg.Group) *View {
	vw := &View{World: world, Scene: sc, Root: root}
	vw.Library = make(map[string]*svg.Group)
	vw.ProjectXZ() // more typical
	vw.LineWidth = 0.05
	return vw
}

// InitLibrary initializes Scene library with basic shapes
// based on bodies in the virtual world.  More complex visualizations
// can be configured after this.
func (vw *View) InitLibrary() {
	vw.InitLibraryBody(vw.World)
}

// Sync synchronizes the view to the world
func (vw *View) Sync() bool {
	rval := vw.SyncNode(vw.World, vw.Root)
	return rval
}

// UpdatePose updates the view pose values only from world tree.
// Essential that both trees are already synchronized.
func (vw *View) UpdatePose() {
	vw.UpdatePoseNode(vw.World, vw.Root)
}

// UpdateBodyView updates the display properties of given body name
// recurses the tree until this body name is found.
func (vw *View) UpdateBodyView(bodyNames []string) {
	vw.UpdateBodyViewNode(bodyNames, vw.World, vw.Root)
}

// Image returns the current rendered image
func (vw *View) Image() (*image.RGBA, error) {
	img := vw.Scene.Pixels
	if img == nil {
		return nil, fmt.Errorf("eve2d.View Image: is nil")
	}
	return img, nil
}

// ProjectXY sets 2D projection to reflect 3D X,Y coords
func (vw *View) ProjectXY() {
	vw.Prjn.SetIdentity()
}

// ProjectXZ sets 2D projection to reflect 3D X,Z coords
func (vw *View) ProjectXZ() {
	vw.Prjn.SetIdentity()
	vw.Prjn[5] = 0 // Y->Y
	vw.Prjn[9] = 1 // Z->Y
}

// todo: more projections

// Prjn2D projects position from 3D to 2D
func (vw *View) Prjn2D(pos mat32.Vec3) mat32.Vec2 {
	v2 := pos.MulMat4(&vw.Prjn)
	return mat32.NewVec2(v2.X, v2.Y)
}

// XForm2D returns the full 2D transform matrix for a given position and quat rotation in 3D
func (vw *View) XForm2D(phys *eve.Phys) mat32.Mat2 {
	pos2 := phys.Pos.MulMat4(&vw.Prjn)
	xyaxis := mat32.Vec3{1, 1, 0}
	xyaxis.SetNormal()
	inv := vw.Prjn.Transpose()
	axis := xyaxis.MulMat4(inv)
	axis.SetNormal()
	rot := axis.MulQuat(phys.Quat)
	rot.SetNormal()
	xyrot := rot.MulMat4(&vw.Prjn)
	xyrot.Z = 0
	xyrot.SetNormal()
	ang := xyrot.AngleTo(xyaxis)
	xf2 := mat32.Translate2D(pos2.X, pos2.Y).Rotate(ang)
	return xf2
}

///////////////////////////////////////////////////////////////
// Sync, Config

// NewInLibrary adds a new item of given name in library
func (vw *View) NewInLibrary(nm string) *svg.Group {
	if vw.Library == nil {
		vw.Library = make(map[string]*svg.Group)
	}
	gp := &svg.Group{}
	gp.InitName(gp, nm)
	vw.Library[nm] = gp
	return gp
}

// AddFmLibrary adds shape from library to given group
func (vw *View) AddFmLibrary(nm string, gp *svg.Group) {
	lgp, has := vw.Library[nm]
	if !has {
		return
	}
	gp.AddChild(lgp.Clone())
}

// InitLibraryBody initializes Scene library with basic shapes
// based on bodies in the virtual world.  More complex visualizations
// can be configured after this.
func (vw *View) InitLibraryBody(wn eve.Node) {
	bod := wn.AsBody()
	if bod != nil {
		vw.InitLibShape(bod)
	}
	for idx := range *wn.Children() {
		wk := wn.Child(idx).(eve.Node)
		vw.InitLibraryBody(wk)
	}
}

// InitLibShape initializes Scene library with basic shape for given body
func (vw *View) InitLibShape(bod eve.Body) {
	nm := bod.Name()
	bb := bod.AsBodyBase()
	if bb.Vis == "" {
		bb.Vis = nm
	}
	if _, has := vw.Library[nm]; has {
		return
	}
	lgp := vw.NewInLibrary(nm)
	wt := kit.ShortTypeName(ki.Type(bod.This()))
	switch wt {
	case "eve.Box":
		mnm := "eveBox"
		svg.AddNewRect(lgp, mnm, 0, 0, 1, 1)
	case "eve.Cylinder":
		mnm := "eveCylinder"
		svg.AddNewCircle(lgp, mnm, 0, 0, 1)
	case "eve.Capsule":
		mnm := "eveCapsule"
		svg.AddNewCircle(lgp, mnm, 0, 0, 1)
	case "eve.Sphere":
		mnm := "eveSphere"
		svg.AddNewCircle(lgp, mnm, 0, 0, 1)
	}
}

// ConfigBodyShape configures a shape for a body with current values
func (vw *View) ConfigBodyShape(bod eve.Body, shp svg.NodeSVG) {
	wt := kit.ShortTypeName(ki.Type(bod.This()))
	sb := shp.AsSVGNode()
	switch wt {
	case "eve.Box":
		bx := bod.(*eve.Box)
		sz := vw.Prjn2D(bx.Size)
		shp.SetSize(sz)
		sb.Pnt.XForm = mat32.Translate2D(-sz.X/2, -sz.Y/2)
		shp.SetProp("transform", sb.Pnt.XForm.String())
		shp.SetProp("stroke-width", vw.LineWidth)
		shp.SetProp("fill", "none")
		if bx.Color != "" {
			shp.SetProp("stroke", bx.Color)
		}
	case "eve.Cylinder":
		cy := bod.(*eve.Cylinder)
		sz3 := mat32.NewVec3(cy.BotRad*2, cy.Height, cy.TopRad*2)
		sz := vw.Prjn2D(sz3)
		shp.SetSize(sz)
		sb.Pnt.XForm = mat32.Translate2D(-sz.X/2, -sz.Y/2)
		shp.SetProp("transform", sb.Pnt.XForm.String())
		shp.SetProp("stroke-width", vw.LineWidth)
		shp.SetProp("fill", "none")
		if cy.Color != "" {
			shp.SetProp("stroke", cy.Color)
		}
	case "eve.Capsule":
		cp := bod.(*eve.Capsule)
		sz3 := mat32.NewVec3(cp.BotRad*2, cp.Height, cp.TopRad*2)
		sz := vw.Prjn2D(sz3)
		shp.SetSize(sz)
		sb.Pnt.XForm = mat32.Translate2D(-sz.X/2, -sz.Y/2)
		shp.SetProp("transform", sb.Pnt.XForm.String())
		shp.SetProp("stroke-width", vw.LineWidth)
		shp.SetProp("fill", "none")
		if cp.Color != "" {
			shp.SetProp("stroke", cp.Color)
		}
	case "eve.Sphere":
		sp := bod.(*eve.Sphere)
		sz3 := mat32.NewVec3(sp.Radius*2, sp.Radius*2, sp.Radius*2)
		sz := vw.Prjn2D(sz3)
		shp.SetSize(sz)
		sb.Pnt.XForm = mat32.Translate2D(-sz.X/2, -sz.Y/2)
		shp.SetProp("transform", sb.Pnt.XForm.String())
		shp.SetProp("stroke-width", vw.LineWidth)
		shp.SetProp("fill", "none")
		if sp.Color != "" {
			shp.SetProp("stroke", sp.Color)
		}
	}
}

// ConfigView configures the view node to properly display world node
func (vw *View) ConfigView(wn eve.Node, vn svg.NodeSVG) {
	wb := wn.AsNodeBase()
	vb := vn.(*svg.Group)
	vb.Pnt.XForm = vw.XForm2D(&wb.Rel)
	vb.SetProp("transform", vb.Pnt.XForm.String())
	bod := wn.AsBody()
	if bod == nil {
		return
	}
	if !vb.HasChildren() {
		vw.AddFmLibrary(bod.AsBodyBase().Vis, vb)
	}
	bgp := vb.Child(0)
	if bgp.HasChildren() {
		shp, has := bgp.Child(0).(svg.NodeSVG)
		if has {
			vw.ConfigBodyShape(bod, shp)
		}
	}
}

// SyncNode updates the view tree to match the world tree, using
// ConfigChildren to maximally preserve existing tree elements
// returns true if view tree was modified (elements added / removed etc)
func (vw *View) SyncNode(wn eve.Node, vn svg.NodeSVG) bool {
	nm := wn.Name()
	vn.SetName(nm) // guaranteed to be unique
	skids := *wn.Children()
	tnl := make(kit.TypeAndNameList, 0, len(skids))
	for _, skid := range skids {
		tnl.Add(svg.KiT_Group, skid.Name())
	}
	mod, updt := vn.ConfigChildren(tnl)
	modall := mod
	for idx := range skids {
		wk := wn.Child(idx).(eve.Node)
		vk := vn.Child(idx).(svg.NodeSVG)
		vw.ConfigView(wk, vk)
		if wk.HasChildren() {
			kmod := vw.SyncNode(wk, vk)
			if kmod {
				modall = true
			}
		}
	}
	vn.UpdateEnd(updt)
	return modall
}

///////////////////////////////////////////////////////////////
// UpdatePose

// UpdatePoseNode updates the view pose values only from world tree.
// Essential that both trees are already synchronized.
func (vw *View) UpdatePoseNode(wn eve.Node, vn svg.NodeSVG) {
	skids := *wn.Children()
	for idx := range skids {
		wk := wn.Child(idx).(eve.Node)
		vk := vn.Child(idx).(svg.NodeSVG).(*svg.Group)
		wb := wk.AsNodeBase()
		vk.Pnt.XForm = vw.XForm2D(&wb.Rel)
		vk.SetProp("transform", vk.Pnt.XForm.String())
		// fmt.Printf("wk: %s  pos: %v  vk: %s\n", wk.Name(), ps, vk.Child(0).Name())
		vw.UpdatePoseNode(wk, vk)
	}
}

// UpdateBodyViewNode updates the body view info for given name(s)
// Essential that both trees are already synchronized.
func (vw *View) UpdateBodyViewNode(bodyNames []string, wn eve.Node, vn svg.NodeSVG) {
	skids := *wn.Children()
	for idx := range skids {
		wk := wn.Child(idx).(eve.Node)
		vk := vn.Child(idx).(svg.NodeSVG)
		match := false
		if _, isBod := wk.(eve.Body); isBod {
			for _, nm := range bodyNames {
				if wk.Name() == nm {
					match = true
					break
				}
			}
		}
		if match {
			bgp := vk.Child(0)
			if bgp.HasChildren() {
				shp, has := bgp.Child(0).(svg.NodeSVG)
				if has {
					vw.ConfigBodyShape(wk.AsBody(), shp)
				}
			}
		}
		vw.UpdateBodyViewNode(bodyNames, wk, vk)
	}
}
