// Copyright (c) 2019, The Emergent Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

//go:generate core generate

import (
	"fmt"
	"image"
	"log"
	"math/rand"
	"os"

	"cogentcore.org/core/abilities"
	"cogentcore.org/core/colors"
	"cogentcore.org/core/colors/colormap"
	"cogentcore.org/core/core"
	"cogentcore.org/core/events"
	"cogentcore.org/core/icons"
	"cogentcore.org/core/iox/images"
	"cogentcore.org/core/math32"
	"cogentcore.org/core/styles"
	"cogentcore.org/core/svg"
	"cogentcore.org/core/views"
	"cogentcore.org/core/xyz"
	"cogentcore.org/core/xyzv"
	"github.com/emer/eve/v2/eve"
	"github.com/emer/eve/v2/eve2d"
	"github.com/emer/eve/v2/evev"
)

var NoGUI bool

func main() {
	if len(os.Args) > 1 && os.Args[1] == "-nogui" {
		NoGUI = true
	}
	ev := &Env{}
	ev.Defaults()
	if NoGUI {
		ev.NoGUIRun()
		return
	}
	// core.RenderTrace = true
	b := ev.ConfigGUI()
	b.NewWindow().Run().Wait()
}

// Env encapsulates the virtual environment
type Env struct {

	// height of emer
	EmerHt float32

	// how far to move every step
	MoveStep float32

	// how far to rotate every step
	RotStep float32

	// width of room
	Width float32

	// depth of room
	Depth float32

	// height of room
	Height float32

	// thickness of walls of room
	Thick float32

	// current depth map
	DepthVals []float32

	// offscreen render camera settings
	Camera evev.Camera

	// color map to use for rendering depth map
	DepthMap views.ColorMapName

	// world
	World *eve.Group `view:"-"`

	// 3D view of world
	View3D *evev.View

	// view of world
	View2D *eve2d.View

	// 3D visualization of the Scene
	SceneView *xyzv.SceneView

	// 2D visualization of the Scene
	Scene2D *core.SVG

	// emer group
	Emer *eve.Group `view:"-"`

	// Right eye of emer
	EyeR eve.Body `view:"-"`

	// contacts from last step, for body
	Contacts eve.Contacts `view:"-"`

	// snapshot bitmap view
	EyeRImg *core.Image `view:"-"`

	// depth map bitmap view
	DepthImage *core.Image `view:"-"`
}

func (ev *Env) Defaults() {
	ev.Width = 10
	ev.Depth = 15
	ev.Height = 2
	ev.Thick = 0.2
	ev.EmerHt = 1
	ev.MoveStep = ev.EmerHt * .2
	ev.RotStep = 15
	ev.DepthMap = views.ColorMapName("ColdHot")
	ev.Camera.Defaults()
	ev.Camera.FOV = 90
}

func (ev *Env) ConfigScene(se *xyz.Scene) {
	se.BackgroundColor = colors.FromRGB(230, 230, 255) // sky blue-ish
	xyz.NewAmbientLight(se, "ambient", 0.3, xyz.DirectSun)

	dir := xyz.NewDirLight(se, "dir", 1, xyz.DirectSun)
	dir.Pos.Set(0, 2, 1) // default: 0,1,1 = above and behind us (we are at 0,0,X)

	// grtx := xyz.NewTextureFileFS(assets.Content, se, "ground", "ground.png")
	// floorp := xyz.NewPlane(se, "floor-plane", 100, 100)
	// floor := xyz.NewSolid(se, "floor").SetMesh(floorp).
	// 	SetColor(colors.Tan).SetTexture(grtx).SetPos(0, -5, 0)
	// floor.Mat.Tiling.Repeat.Set(40, 40)
}

// MakeWorld constructs a new virtual physics world
func (ev *Env) MakeWorld() {
	ev.World = &eve.Group{}
	ev.World.InitName(ev.World, "RoomWorld")

	MakeRoom(ev.World, "room1", ev.Width, ev.Depth, ev.Height, ev.Thick)
	ev.Emer = MakeEmer(ev.World, ev.EmerHt)
	ev.EyeR = ev.Emer.ChildByName("head", 1).ChildByName("eye-r", 2).(eve.Body)

	ev.World.WorldInit()
}

// InitWorld does init on world and re-syncs
func (ev *Env) WorldInit() { //gti:add
	ev.World.WorldInit()
	if ev.View3D != nil {
		ev.View3D.Sync()
		ev.GrabEyeImg()
	}
	if ev.View2D != nil {
		ev.View2D.Sync()
	}
}

// ReMakeWorld rebuilds the world and re-syncs with gui
func (ev *Env) ReMakeWorld() { //gti:add
	ev.MakeWorld()
	ev.View3D.World = ev.World
	if ev.View3D != nil {
		ev.View3D.Sync()
		ev.GrabEyeImg()
	}
	if ev.View2D != nil {
		ev.View2D.Sync()
	}
}

// ConfigView3D makes the 3D view
func (ev *Env) ConfigView3D(sc *xyz.Scene) {
	// sc.MultiSample = 1 // we are using depth grab so we need this = 1
	wgp := xyz.NewGroup(sc, "world")
	ev.View3D = evev.NewView(ev.World, sc, wgp)
	ev.View3D.InitLibrary() // this makes a basic library based on body shapes, sizes
	// at this point the library can be updated to configure custom visualizations
	// for any of the named bodies.
	ev.View3D.Sync()
}

// ConfigView2D makes the 2D view
func (ev *Env) ConfigView2D(sc *svg.SVG) {
	wgp := svg.NewGroup(&sc.Root, "world")
	ev.View2D = eve2d.NewView(ev.World, sc, wgp)
	ev.View2D.InitLibrary() // this makes a basic library based on body shapes, sizes
	// at this point the library can be updated to configure custom visualizations
	// for any of the named bodies.
	ev.View2D.Sync()
}

// RenderEyeImg returns a snapshot from the perspective of Emer's right eye
func (ev *Env) RenderEyeImg() (*image.RGBA, error) {
	err := ev.View3D.RenderOffNode(ev.EyeR, &ev.Camera)
	if err != nil {
		log.Println(err)
		return nil, err
	}
	return ev.View3D.Image()
}

// GrabEyeImg takes a snapshot from the perspective of Emer's right eye
func (ev *Env) GrabEyeImg() { //gti:add
	img, err := ev.RenderEyeImg()
	if err == nil && img != nil {
		ev.EyeRImg.SetImage(img)
		ev.EyeRImg.NeedsRender()
	} else {
		log.Println(err)
	}

	depth, err := ev.View3D.DepthImage()
	if err == nil && depth != nil {
		ev.DepthVals = depth
		ev.ViewDepth(depth)
	}
}

// ViewDepth updates depth bitmap with depth data
func (ev *Env) ViewDepth(depth []float32) {
	cmap := colormap.AvailableMaps[string(ev.DepthMap)]
	ev.DepthImage.Image = image.NewRGBA(image.Rectangle{Max: ev.Camera.Size})
	evev.DepthImage(ev.DepthImage.Image, depth, cmap, &ev.Camera)
	ev.DepthImage.NeedsRender()
}

// UpdateViews updates the 2D and 3D views of the scene
func (ev *Env) UpdateViews() {
	if ev.SceneView.IsVisible() {
		ev.SceneView.NeedsRender()
	}
	if ev.Scene2D.IsVisible() {
		ev.Scene2D.NeedsRender()
	}
}

// WorldStep does one step of the world
func (ev *Env) WorldStep() {
	ev.World.WorldRelToAbs()
	cts := ev.World.WorldCollide(eve.DynsTopGps)
	ev.Contacts = nil
	for _, cl := range cts {
		if len(cl) > 1 {
			for _, c := range cl {
				if c.A.Name() == "body" {
					ev.Contacts = cl
				}
				fmt.Printf("A: %v  B: %v\n", c.A.Name(), c.B.Name())
			}
		}
	}
	if len(ev.Contacts) > 1 { // turn around
		fmt.Printf("hit wall: turn around!\n")
		rot := 100.0 + 90.0*rand.Float32()
		ev.Emer.Rel.RotateOnAxis(0, 1, 0, rot)
	}
	ev.View3D.UpdatePose()
	ev.View2D.UpdatePose()
	ev.GrabEyeImg()
	ev.UpdateViews()
}

// StepForward moves Emer forward in current facing direction one step, and takes GrabEyeImg
func (ev *Env) StepForward() { //gti:add
	ev.Emer.Rel.MoveOnAxis(0, 0, 1, -ev.MoveStep)
	ev.WorldStep()
}

// StepBackward moves Emer backward in current facing direction one step, and takes GrabEyeImg
func (ev *Env) StepBackward() { //gti:add
	ev.Emer.Rel.MoveOnAxis(0, 0, 1, ev.MoveStep)
	ev.WorldStep()
}

// RotBodyLeft rotates emer left and takes GrabEyeImg
func (ev *Env) RotBodyLeft() { //gti:add
	ev.Emer.Rel.RotateOnAxis(0, 1, 0, ev.RotStep)
	ev.WorldStep()
}

// RotBodyRight rotates emer right and takes GrabEyeImg
func (ev *Env) RotBodyRight() { //gti:add
	ev.Emer.Rel.RotateOnAxis(0, 1, 0, -ev.RotStep)
	ev.WorldStep()
}

// RotHeadLeft rotates emer left and takes GrabEyeImg
func (ev *Env) RotHeadLeft() { //gti:add
	hd := ev.Emer.ChildByName("head", 1).(*eve.Group)
	hd.Rel.RotateOnAxis(0, 1, 0, ev.RotStep)
	ev.WorldStep()
}

// RotHeadRight rotates emer right and takes GrabEyeImg
func (ev *Env) RotHeadRight() { //gti:add
	hd := ev.Emer.ChildByName("head", 1).(*eve.Group)
	hd.Rel.RotateOnAxis(0, 1, 0, -ev.RotStep)
	ev.WorldStep()
}

// MakeRoom constructs a new room in given parent group with given params
func MakeRoom(par *eve.Group, name string, width, depth, height, thick float32) *eve.Group {
	rm := eve.NewGroup(par, name)
	eve.NewBox(rm, "floor").SetSize(math32.Vec3(width, thick, depth)).
		SetColor("grey").SetInitPos(math32.Vec3(0, -thick/2, 0))

	eve.NewBox(rm, "back-wall").SetSize(math32.Vec3(width, height, thick)).
		SetColor("blue").SetInitPos(math32.Vec3(0, height/2, -depth/2))
	eve.NewBox(rm, "left-wall").SetSize(math32.Vec3(thick, height, depth)).
		SetColor("red").SetInitPos(math32.Vec3(-width/2, height/2, 0))
	eve.NewBox(rm, "right-wall").SetSize(math32.Vec3(thick, height, depth)).
		SetColor("green").SetInitPos(math32.Vec3(width/2, height/2, 0))
	eve.NewBox(rm, "front-wall").SetSize(math32.Vec3(width, height, thick)).
		SetColor("yellow").SetInitPos(math32.Vec3(0, height/2, depth/2))
	return rm
}

// MakeEmer constructs a new Emer virtual robot of given height (e.g., 1)
func MakeEmer(par *eve.Group, height float32) *eve.Group {
	emr := eve.NewGroup(par, "emer")
	width := height * .4
	depth := height * .15

	eve.NewBox(emr, "body").SetSize(math32.Vec3(width, height, depth)).
		SetColor("purple").SetDynamic().
		SetInitPos(math32.Vec3(0, height/2, 0))
	// body := eve.NewCapsule(emr, "body", math32.Vec3(0, height / 2, 0), height, width/2)
	// body := eve.NewCylinder(emr, "body", math32.Vec3(0, height / 2, 0), height, width/2)

	headsz := depth * 1.5
	hhsz := .5 * headsz
	hgp := eve.NewGroup(emr, "head").SetInitPos(math32.Vec3(0, height+hhsz, 0))

	eve.NewBox(hgp, "head").SetSize(math32.Vec3(headsz, headsz, headsz)).
		SetColor("tan").SetDynamic().SetInitPos(math32.Vec3(0, 0, 0))

	eyesz := headsz * .2
	eve.NewBox(hgp, "eye-l").SetSize(math32.Vec3(eyesz, eyesz*.5, eyesz*.2)).
		SetColor("green").SetDynamic().
		SetInitPos(math32.Vec3(-hhsz*.6, headsz*.1, -(hhsz + eyesz*.3)))

	eve.NewBox(hgp, "eye-r").SetSize(math32.Vec3(eyesz, eyesz*.5, eyesz*.2)).
		SetColor("green").SetDynamic().
		SetInitPos(math32.Vec3(hhsz*.6, headsz*.1, -(hhsz + eyesz*.3)))

	return emr
}

func (ev *Env) ConfigGUI() *core.Body {
	// vgpu.Debug = true

	b := core.NewBody("virtroom").SetTitle("Emergent Virtual Engine")

	ev.MakeWorld()

	split := core.NewSplits(b, "split")

	tv := views.NewTreeView(core.NewFrame(split), "tv").SyncTree(ev.World)
	sv := views.NewStructView(split, "sv").SetStruct(ev)
	imfr := core.NewFrame(split)
	tbvw := core.NewTabs(split)

	scfr := tbvw.NewTab("3D View")
	twofr := tbvw.NewTab("2D View")

	split.SetSplits(.1, .2, .2, .5)

	tv.OnSelect(func(e events.Event) {
		if len(tv.SelectedNodes) > 0 {
			sv.SetStruct(tv.SelectedNodes[0].AsTreeView().SyncNode)
		}
	})

	//////////////////////////////////////////
	//    3D Scene

	ev.SceneView = xyzv.NewSceneView(scfr, "sceneview")
	ev.SceneView.Config()
	se := ev.SceneView.SceneXYZ()
	ev.ConfigScene(se)
	ev.ConfigView3D(se)

	se.Camera.Pose.Pos = math32.Vec3(0, 40, 3.5)
	se.Camera.LookAt(math32.Vec3(0, 5, 0), math32.Vec3(0, 1, 0))
	se.SaveCamera("3")

	se.Camera.Pose.Pos = math32.Vec3(0, 20, 30)
	se.Camera.LookAt(math32.Vec3(0, 5, 0), math32.Vec3(0, 1, 0))
	se.SaveCamera("2")

	se.Camera.Pose.Pos = math32.Vec3(-.86, .97, 2.7)
	se.Camera.LookAt(math32.Vec3(0, .8, 0), math32.Vec3(0, 1, 0))
	se.SaveCamera("1")
	se.SaveCamera("default")

	//////////////////////////////////////////
	//    Image

	imfr.Style(func(s *styles.Style) {
		s.Direction = styles.Column
	})
	core.NewLabel(imfr).SetText("Right Eye Image:")
	ev.EyeRImg = core.NewImage(imfr, "eye-r-img")
	ev.EyeRImg.Image = image.NewRGBA(image.Rectangle{Max: ev.Camera.Size})

	core.NewLabel(imfr).SetText("Right Eye Depth:")
	ev.DepthImage = core.NewImage(imfr, "depth-img")
	ev.DepthImage.Image = image.NewRGBA(image.Rectangle{Max: ev.Camera.Size})

	//////////////////////////////////////////
	//    2D Scene

	twov := core.NewSVG(twofr, "sceneview")
	ev.Scene2D = twov
	twov.Style(func(s *styles.Style) {
		s.Grow.Set(1, 1)
		twov.SVG.Root.ViewBox.Size.Set(ev.Width+4, ev.Depth+4)
		twov.SVG.Root.ViewBox.Min.Set(-0.5*(ev.Width+4), -0.5*(ev.Depth+4))
		twov.SetReadOnly(false)
	})

	ev.ConfigView2D(twov.SVG)

	//////////////////////////////////////////
	//    Toolbar

	b.AddAppBar(func(tb *core.Toolbar) {
		core.NewButton(tb).SetText("Edit Env").SetIcon(icons.Edit).
			SetTooltip("Edit the settings for the environment").
			OnClick(func(e events.Event) {
				sv.SetStruct(ev)
			})
		views.NewFuncButton(tb, ev.WorldInit).SetText("Init").SetIcon(icons.Update)
		views.NewFuncButton(tb, ev.ReMakeWorld).SetText("Make").SetIcon(icons.Update)
		views.NewFuncButton(tb, ev.GrabEyeImg).SetText("Grab Image").SetIcon(icons.Image)
		core.NewSeparator(tb)

		views.NewFuncButton(tb, ev.StepForward).SetText("Fwd").SetIcon(icons.SkipNext).
			Style(func(s *styles.Style) {
				s.SetAbilities(true, abilities.RepeatClickable)
			})
		views.NewFuncButton(tb, ev.StepBackward).SetText("Bkw").SetIcon(icons.SkipPrevious).
			Style(func(s *styles.Style) {
				s.SetAbilities(true, abilities.RepeatClickable)
			})
		views.NewFuncButton(tb, ev.RotBodyLeft).SetText("Body Left").SetIcon(icons.KeyboardArrowLeft).
			Style(func(s *styles.Style) {
				s.SetAbilities(true, abilities.RepeatClickable)
			})
		views.NewFuncButton(tb, ev.RotBodyRight).SetText("Body Right").SetIcon(icons.KeyboardArrowRight).
			Style(func(s *styles.Style) {
				s.SetAbilities(true, abilities.RepeatClickable)
			})
		views.NewFuncButton(tb, ev.RotHeadLeft).SetText("Head Left").SetIcon(icons.KeyboardArrowLeft).
			Style(func(s *styles.Style) {
				s.SetAbilities(true, abilities.RepeatClickable)
			})
		views.NewFuncButton(tb, ev.RotHeadRight).SetText("Head Right").SetIcon(icons.KeyboardArrowRight).
			Style(func(s *styles.Style) {
				s.SetAbilities(true, abilities.RepeatClickable)
			})
		core.NewSeparator(tb)

		core.NewButton(tb).SetText("README").SetIcon(icons.FileMarkdown).
			SetTooltip("Open browser on README.").
			OnClick(func(e events.Event) {
				core.TheApp.OpenURL("https://github.com/emer/eve/blob/master/examples/virtroom/README.md")
			})
	})
	return b
}

func (ev *Env) NoGUIRun() {
	gp, dev, err := evev.NoDisplayGPU("virtroom")
	if err != nil {
		panic(err)
	}
	se := evev.NoDisplayScene("virtroom", gp, dev)
	ev.ConfigScene(se)
	ev.MakeWorld()
	ev.ConfigView3D(se)

	se.Config()

	img, err := ev.RenderEyeImg()
	if err == nil {
		images.Save(img, "eyer_0.png")
	} else {
		panic(err)
	}
}
