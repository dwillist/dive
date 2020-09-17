package ui

import (
	"fmt"
	"github.com/spf13/viper"
	"github.com/wagoodman/dive/dive/image"
	"github.com/wagoodman/dive/runtime/ui/key"
	"github.com/wagoodman/dive/runtime/ui/layout"
	"github.com/wagoodman/dive/runtime/ui/layout/compound"
	"github.com/wagoodman/dive/runtime/ui/view"
	"github.com/wagoodman/dive/runtime/ui/viewmodel"
	"sync"

	"github.com/jroimartin/gocui"
	"github.com/sirupsen/logrus"
	"github.com/wagoodman/dive/dive/filetree"
)

const debug = false

// type global
type app struct {
	gui         *gocui.Gui
	controllers *Controller
	layout      *layout.Manager
}

var (
	once         sync.Once
	appSingleton *app
)

type detailsModel struct {
	efficiency float64
	inefficiencies filetree.EfficiencySlice
	sizeBytes uint64
}

type diveModels struct {
	filetree *viewmodel.FileTree
	compareMode *viewmodel.LayerCompareMode
	layerSelection *viewmodel.LayerSelection
	layerSetState *viewmodel.LayerSetState
	// TODO: move me to the models folder
	details detailsModel
}

func initializeModels(gui *gocui.Gui, analysis *image.AnalysisResult, cache filetree.Comparer) (diveModels, error) {
	trees := analysis.RefTrees
	layers := analysis.Layers

	// TODO: do we need to check this??
	firstTree := analysis.RefTrees[0]
	firstLayer := layers[0]

	// fileTreeModel initialization
	fileTreeModel, err := viewmodel.NewFileTreeViewModel(firstTree, trees, cache)
	if err != nil {
		return diveModels{}, err
	}

	// compareMode
	var compareMode viewmodel.LayerCompareMode
	switch mode := viper.GetBool("layer.show-aggregated-changes"); mode {
	case true:
		compareMode = viewmodel.CompareAllLayers
	case false:
		compareMode = viewmodel.CompareSingleLayer
	default:
		return diveModels{}, fmt.Errorf("unknown layer.show-aggregated-changes value: %v", mode)
	}

	layerSelection := viewmodel.LayerSelection{
		Layer:           firstLayer,
		BottomTreeStart: 0,
		BottomTreeStop:  0,
		TopTreeStart:    0,
		TopTreeStop:     0,
	}

	layerSetState := viewmodel.NewLayerSetState(layers, compareMode)
	details := detailsModel{
		efficiency: analysis.Efficiency,
		inefficiencies: analysis.Inefficiencies,
		sizeBytes: analysis.SizeBytes,
	}
	return diveModels{
		filetree: fileTreeModel,
		compareMode: &compareMode,
		layerSelection: &layerSelection,
		layerSetState: layerSetState,
		details: details,
	}, nil
}

func initializeViews(gui * gocui.Gui, m diveModels) (result view.Views, err error) {
	layerView, err := view.NewLayerView(gui, m.layerSetState)
	if err != nil {
		return result, err
	}

	fileTreeView, err := view.NewFileTreeView(gui, m.filetree)

	statusView := view.NewStatusView(gui)
	statusView.SetCurrentView(layerView)

	filterView := view.NewFilterView(gui)

	detailsView := view.NewDetailsView(gui, m.details.efficiency, m.details.inefficiencies, m.details.sizeBytes)

	return view.Views{
		Tree: fileTreeView,
		Layer: layerView,
		Status: statusView,
		Filter: filterView,
		Details: detailsView,
	}, nil
}

func initializeController(g *gocui.Gui, views *view.Views) (*Controller, error) {
	controller := &Controller{
		gui:   g,
		views: views,
	}

	controller.views.Layer.AddLayerChangeListener(controller.onLayerChange)

	// update the status pane when a filetree option is changed by the user
	controller.views.Tree.AddViewOptionChangeListener(controller.onFileTreeViewOptionChange)

	// update the tree view while the user types into the filter view
	controller.views.Filter.AddFilterEditListener(controller.onFilterEdit)

	err := controller.onLayerChange(viewmodel.LayerSelection{
		Layer:           controller.views.Layer.CurrentLayer(),
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

// TODO: app should be built from the bottom up, this really makes components re-usable, and adheres to DIP
// right now we have a top down initialization structure which is 'messy'
func newApp(gui *gocui.Gui, analysis *image.AnalysisResult, cache filetree.Comparer) (*app, error) {
	var err error
	once.Do(func() {
		var globalHelpKeys []*key.Binding
		// create models
		m, err := initializeModels(gui, analysis , cache )
		if err != nil {
			return
		}

		// create views
		v, err := initializeViews(gui,m)
		if err != nil {
			return
		}

		controller, err := initializeController(gui, &v)

		lm := layout.NewManager()
		lm.Add(controller.views.Status, layout.LocationFooter)
		lm.Add(controller.views.Filter, layout.LocationFooter)
		lm.Add(compound.NewLayerDetailsCompoundLayout(controller.views.Layer, controller.views.Details), layout.LocationColumn)
		lm.Add(controller.views.Tree, layout.LocationColumn)

		if debug {
			lm.Add(controller.views.Debug, layout.LocationColumn)
		}
		gui.Cursor = false
		gui.SetManagerFunc(lm.Layout)

		appSingleton = &app{
			gui:         gui,
			controllers: controller,
			layout:      lm,
		}

		var infos = []key.BindingInfo{
			{
				ConfigKeys: []string{"keybinding.quit"},
				OnAction:   appSingleton.quit,
				Display:    "Quit",
			},
			{
				ConfigKeys: []string{"keybinding.toggle-view"},
				OnAction:   controller.ToggleView,
				Display:    "Switch view",
			},
			{
				ConfigKeys: []string{"keybinding.filter-files"},
				OnAction:   controller.ToggleFilterView,
				IsSelected: controller.views.Filter.IsVisible,
				Display:    "Filter",
			},
		}

		globalHelpKeys, err = key.GenerateBindings(gui, "", infos)
		if err != nil {
			return
		}

		controller.views.Status.AddHelpKeys(globalHelpKeys...)

		// perform the first update and render now that all resources have been loaded
		err = controller.UpdateAndRender()
		if err != nil {
			return
		}
	})

	return appSingleton, err
}

// var profileObj = profile.Start(profile.MemProfile, profile.ProfilePath("."), profile.NoShutdownHook)
// var onExit func()

// debugPrint writes the given string to the debug pane (if the debug pane is enabled)
// func debugPrint(s string) {
// 	if controllers.Tree != nil && controllers.Tree.gui != nil {
// 		v, _ := controllers.Tree.gui.View("debug")
// 		if v != nil {
// 			if len(v.BufferLines()) > 20 {
// 				v.Clear()
// 			}
// 			_, _ = fmt.Fprintln(v, s)
// 		}
// 	}
// }

// quit is the gocui callback invoked when the user hits Ctrl+C
func (a *app) quit() error {

	// profileObj.Stop()
	// onExit()

	return gocui.ErrQuit
}

// Run is the UI entrypoint.
func Run(analysis *image.AnalysisResult, treeStack filetree.Comparer) error {
	var err error

	g, err := gocui.NewGui(gocui.OutputNormal)
	if err != nil {
		return err
	}
	defer g.Close()

	_, err = newApp(g, analysis, treeStack)
	if err != nil {
		return err
	}

	if err := g.MainLoop(); err != nil && err != gocui.ErrQuit {
		logrus.Error("main loop error: ", err)
		return err
	}
	return nil
}
