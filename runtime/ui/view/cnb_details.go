package view

import (
	"encoding/json"
	"fmt"
	"github.com/buildpacks/lifecycle"
	"github.com/buildpacks/pack"
	"github.com/sirupsen/logrus"
	"github.com/wagoodman/dive/dive/image"
	"github.com/wagoodman/dive/runtime/ui/format"
	"github.com/wagoodman/dive/runtime/ui/key"
	"strings"

	"github.com/jroimartin/gocui"
)

// Details holds the UI objects and data models for populating the lower-left pane. Specifically the pane that
// shows the layer details and image statistics.
type CNBDetails struct {
	name           string
	gui            *gocui.Gui
	view           *gocui.View
	header         *gocui.View
	imageSize      uint64
	imageInfo      *pack.ImageInfo
	currentLayer *image.Layer
}

// newDetailsView creates a new view object attached the the global [gocui] screen object.
func newCNBDetailsView(gui *gocui.Gui, imageInfo *pack.ImageInfo, imageSize uint64) (controller *CNBDetails) {
	controller = new(CNBDetails)
	controller.imageInfo = imageInfo
	// populate main fields
	controller.name = "details"
	controller.gui = gui
	controller.imageSize = imageSize

	return controller
}

func (v *CNBDetails) Name() string {
	return v.name
}

// Setup initializes the UI concerns within the context of a global [gocui] view object.
func (v *CNBDetails) Setup(view *gocui.View, header *gocui.View) error {
	logrus.Tracef("view.Setup() %s", v.Name())

	// set controller options
	v.view = view
	v.view.Editable = false
	v.view.Wrap = true
	v.view.Highlight = false
	v.view.Frame = false

	v.header = header
	v.header.Editable = false
	v.header.Wrap = false
	v.header.Frame = false

	var infos = []key.BindingInfo{
		{
			Key:      gocui.KeyArrowDown,
			Modifier: gocui.ModNone,
			OnAction: v.CursorDown,
		},
		{
			Key:      gocui.KeyArrowUp,
			Modifier: gocui.ModNone,
			OnAction: v.CursorUp,
		},
	}

	_, err := key.GenerateBindings(v.gui, v.name, infos)
	if err != nil {
		return err
	}

	return v.Render()
}

// IsVisible indicates if the details view pane is currently initialized.
func (v *CNBDetails) IsVisible() bool {
	return v != nil
}

// CursorDown moves the cursor down in the details pane (currently indicates nothing).
func (v *CNBDetails) CursorDown() error {
	return CursorDown(v.gui, v.view)
}

// CursorUp moves the cursor up in the details pane (currently indicates nothing).
func (v *CNBDetails) CursorUp() error {
	return CursorUp(v.gui, v.view)
}

// OnLayoutChange is called whenever the screen dimensions are changed
func (v *CNBDetails) OnLayoutChange() error {
	err := v.Update()
	if err != nil {
		return err
	}
	return v.Render()
}

// Update refreshes the state objects for future rendering.
func (v *CNBDetails) Update() error {
	return nil
}

func (v *CNBDetails) SetCurrentLayer(layer *image.Layer) {
	v.currentLayer = layer
}

// Render flushes the state objects to the screen. The details pane reports:
// 1. the current selected layer's command string
// 2. the image efficiency score
// 3. the estimated wasted image space
// 4. a list of inefficient file allocations
func (v *CNBDetails) Render() error {
	logrus.Tracef("view.Render() %s", v.Name())

	if v.currentLayer == nil {
		return fmt.Errorf("no layer selected")
	}

	v.gui.Update(func(g *gocui.Gui) error {
		// update header
		v.header.Clear()
		width, _ := v.view.Size()

		layerHeaderStr := format.RenderHeader("Layer CNB Details", width, false)
		imageHeaderStr := format.RenderHeader("Image CNB Details", width, false)

		_, err := fmt.Fprintln(v.header, layerHeaderStr)
		if err != nil {
			return err
		}

		// update contents
		v.view.Clear()

		var lines = make([]string, 0)
		if v.currentLayer.Names != nil && len(v.currentLayer.Names) > 0 {
			lines = append(lines, format.Header("Tags:   ")+strings.Join(v.currentLayer.Names, ", "))
		} else {
			lines = append(lines, format.Header("Tags:   ")+"(none)")
		}
		lines = append(lines, format.Header("Id:     ")+v.currentLayer.Id)
		lines = append(lines, format.Header("Digest: ")+v.currentLayer.Digest)
		lines = append(lines, format.Header("Buildpack: ") + "some-buildpack")
		lines = append(lines, "\n"+imageHeaderStr)
		lines = append(lines, renderBOM(v.currentLayer.Digest, v.imageInfo))

		_, err = fmt.Fprintln(v.view, strings.Join(lines, "\n"))
		if err != nil {
			logrus.Debug("unable to write to buffer: ", err)
		}
		return err
	})
	return nil
}

// KeyHelp indicates all the possible actions a user can take while the current pane is selected (currently does nothing).
func (v *CNBDetails) KeyHelp() string {
	return "TBD"
}

// Needs a lot of changes, really first need to solve the problem of piping a mapping
// from layer digetss to BOMS
func renderBOM(currentDigest string, info *pack.ImageInfo) string {
	// find the correct layer
	result := "No corresponding buildpack found"
	var buildpackEntry lifecycle.Buildpack
	for _, metadata := range info.LayersMetadata {
		for _, layer := range metadata.BuildpackLayers {
			if layer.SHA == currentDigest {
				buildpackEntry = lifecycle.Buildpack{
					ID: metadata.Key,
					Version: metadata.Version,
				}
			}
		}
	}

	// it is empty
	if buildpackEntry.ID == "" {
		var result string
		result = fmt.Sprintf("Layer is not generated by a buildpack %s\n", currentDigest)
		return result
	}

	var bomEntry lifecycle.BOMEntry
	for _, entry := range info.BOM {
		if entry.Buildpack.ID == buildpackEntry.ID && entry.Buildpack.Version == buildpackEntry.Version {
			bomEntry = entry
		}
	}

	// If it is not empty
	if bomEntry.Name != "" {
		bomOutput := "Bill of materials Layer entry:\n"
		bomBuildpack := format.Header("Providing Buildpack:\n") + fmt.Sprintf("%s@%s", bomEntry.Buildpack.ID, bomEntry.Buildpack.Version)

		bomRequires := format.Header("Requirements:\n") + fmt.Sprintf("  %v@%s", bomEntry.Require.Name, bomEntry.Require.Version)
		bomProvides := format.Header("Provides:\n") + fmt.Sprintf("  %s@%s", bomEntry.Name, bomEntry.Version)
		metadataString, err := json.MarshalIndent(bomEntry.Metadata,"  ","  ")
		if err != nil {
			metadataString = []byte("invalid metadata object")
		}
		metadata :=  format.Header("Metadata:\n") + string(metadataString)
		result = strings.Join([]string{
			bomOutput,
			bomBuildpack,
			bomRequires,
			bomProvides,
			metadata,
		}, "\n")
	} else {
		return "no bom entry found"
	}
	return result
}
