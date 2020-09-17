package view

import (
	"fmt"
	"github.com/buildpacks/pack"
	"github.com/jroimartin/gocui"
	"github.com/wagoodman/dive/dive/filetree"
	"github.com/wagoodman/dive/dive/image"
)

type ImageDetails interface {
	OnLayoutChange() error
	Name() string
	Setup(*gocui.View,*gocui.View) error
	SetCurrentLayer(layer *image.Layer)
	Renderer
}

type Views struct {
	Tree    *FileTree
	Layer   *Layer
	Status  *Status
	Filter  *Filter
	Details ImageDetails
	Debug   *Debug
}

func NewViews(g *gocui.Gui, analysis *image.AnalysisResult, cache filetree.Comparer) (*Views, error) {
	Layer, err := newLayerView(g, analysis.Layers)
	if err != nil {
		return nil, err
	}

	treeStack := analysis.RefTrees[0]
	Tree, err := newFileTreeView(g, treeStack, analysis.RefTrees, cache)
	if err != nil {
		return nil, err
	}

	Status := newStatusView(g)

	// set the layer view as the first selected view
	Status.SetCurrentView(Layer)

	Filter := newFilterView(g)

	// TODO add switches here so that this is in an if condition
	//Details := newDetailsView(g, analysis.Efficiency, analysis.Inefficiencies, analysis.SizeBytes)

	// this call should be factored out and only used once....
	client, err := pack.NewClient()
	if err != nil {
		return nil, fmt.Errorf("unable to create pack client: %s", err)
	}

	imgInfo, err := client.InspectImage("java-test", true)
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve %s image info: %s", "my-test", err)
	}


	Details := newCNBDetailsView(g, imgInfo, analysis.SizeBytes)

	Debug := newDebugView(g)

	return &Views{
		Tree:    Tree,
		Layer:   Layer,
		Status:  Status,
		Filter:  Filter,
		Details: Details,
		Debug:   Debug,
	}, nil
}

func (views *Views) All() []Renderer {
	return []Renderer{
		views.Tree,
		views.Layer,
		views.Status,
		views.Filter,
		views.Details,
	}
}
