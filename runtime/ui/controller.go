package ui

import (
	"github.com/jroimartin/gocui"
	"github.com/sirupsen/logrus"
	"github.com/wagoodman/dive/dive/filetree"
	"github.com/wagoodman/dive/dive/image"
	"github.com/wagoodman/dive/runtime/ui/view"
	"github.com/wagoodman/dive/runtime/ui/viewmodel"
	"regexp"
)

type Controller struct {
	Gui   *gocui.Gui
	Views *view.Views
}

func NewCollection(g *gocui.Gui, analysis *image.AnalysisResult, cache filetree.Comparer) (*Controller, error) {
	views, err := view.NewViews(g, analysis, cache)
	if err != nil {
		return nil, err
	}

	controller := &Controller{
		Gui:   g,
		Views: views,
	}

	// layer view cursor down event should trigger an update in the file tree
	views.Layer.AddLayerChangeListener(controller.OnLayerChange)

	// update the status pane when a filetree option is changed by the user
	views.Tree.AddViewOptionChangeListener(controller.OnFileTreeViewOptionChange)

	// update the tree view while the user types into the filter view
	views.Filter.AddFilterEditListener(controller.OnFilterEdit)

	// propagate initial conditions to necessary views
	err = controller.OnLayerChange(viewmodel.LayerSelection{
		Layer:           controller.Views.Layer.CurrentLayer(),
		BottomTreeStart: 0,
		BottomTreeStop:  0,
		TopTreeStart:    0,
		TopTreeStop:     0,
	})

	if err != nil {
		return nil, err
	}

	return controller, nil
}

func (c *Controller) OnFileTreeViewOptionChange() error {
	err := c.Views.Status.Update()
	if err != nil {
		return err
	}
	return c.Views.Status.Render()
}

func (c *Controller) OnFilterEdit(filter string) error {
	var filterRegex *regexp.Regexp
	var err error

	if len(filter) > 0 {
		filterRegex, err = regexp.Compile(filter)
		if err != nil {
			return err
		}
	}

	c.Views.Tree.SetFilterRegex(filterRegex)

	err = c.Views.Tree.Update()
	if err != nil {
		return err
	}

	return c.Views.Tree.Render()
}

func (c *Controller) OnLayerChange(selection viewmodel.LayerSelection) error {
	// update the details
	c.Views.Details.SetCurrentLayer(selection.Layer)

	// update the filetree
	err := c.Views.Tree.SetTree(selection.BottomTreeStart, selection.BottomTreeStop, selection.TopTreeStart, selection.TopTreeStop)
	if err != nil {
		return err
	}

	if c.Views.Layer.CompareMode() == viewmodel.CompareAllLayers {
		c.Views.Tree.SetTitle("Aggregated Layer Contents")
	} else {
		c.Views.Tree.SetTitle("Current Layer Contents")
	}

	// update details and filetree panes
	return c.UpdateAndRender()
}

func (c *Controller) UpdateAndRender() error {
	err := c.Update()
	if err != nil {
		logrus.Debug("failed update: ", err)
		return err
	}

	err = c.Render()
	if err != nil {
		logrus.Debug("failed render: ", err)
		return err
	}

	return nil
}

// Update refreshes the state objects for future rendering.
func (c *Controller) Update() error {
	// TODO: this seems like a break down in concerns
	// we really want to be updating a MODEL here,
	// updating model should push data into Views.
	for _, controller := range c.Views.All() {
		err := controller.Update()
		if err != nil {
			logrus.Debug("unable to update controller: ")
			return err
		}
	}
	return nil
}

// Render flushes the state objects to the screen.
func (c *Controller) Render() error {
	for _, controller := range c.Views.All() {
		if controller.IsVisible() {
			err := controller.Render()
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// ToggleView switches between the file view and the layer view and re-renders the screen.
func (c *Controller) ToggleView() (err error) {
	v := c.Gui.CurrentView()
	if v == nil || v.Name() == c.Views.Layer.Name() {
		_, err = c.Gui.SetCurrentView(c.Views.Tree.Name())
		c.Views.Status.SetCurrentView(c.Views.Tree)
	} else {
		_, err = c.Gui.SetCurrentView(c.Views.Layer.Name())
		c.Views.Status.SetCurrentView(c.Views.Layer)
	}

	if err != nil {
		logrus.Error("unable to toggle view: ", err)
		return err
	}

	return c.UpdateAndRender()
}

func (c *Controller) ToggleFilterView() error {
	// delete all user input from the tree view
	err := c.Views.Filter.ToggleVisible()
	if err != nil {
		logrus.Error("unable to toggle filter visibility: ", err)
		return err
	}

	// we have just hidden the filter view...
	if !c.Views.Filter.IsVisible() {
		// ...remove any filter from the tree
		c.Views.Tree.SetFilterRegex(nil)

		// ...adjust focus to a valid (visible) view
		err = c.ToggleView()
		if err != nil {
			logrus.Error("unable to toggle filter view (back): ", err)
			return err
		}
	}

	return c.UpdateAndRender()
}
