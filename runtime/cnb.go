package runtime

import (
	"fmt"
	"github.com/buildpacks/pack"
	"github.com/wagoodman/dive/dive/filetree"
	"github.com/wagoodman/dive/dive/image"
)

func AnalyzeCNB(analysis *image.AnalysisResult, imageName string) (*image.AnalysisResult,error) {
	// first lets get the cnb info that we need
	client, err := pack.NewClient()
	if err != nil {
		return nil, fmt.Errorf("unable to create pack client: %s", err)
	}

	img, err := client.InspectImage(imageName, true)
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve %s image info: %s", imageName, err)
	}

	newLayers := []*image.Layer{}
	newRefTree := []*filetree.FileTree{}

	if len(analysis.Layers) != len(analysis.RefTrees) {
		return nil, fmt.Errorf("mismatched lengths %s vs %s", len(analysis.Layers), len(analysis.RefTrees))
	}

	var curLayer *image.Layer = nil
	var curRefTree *filetree.FileTree = nil
	var isStack bool = true
	for layerIdx,layer := range analysis.Layers {
		rTree := analysis.RefTrees[layerIdx]
		if curLayer == nil {
			curLayer = layer
			curRefTree = rTree
			continue
		}
		if isStack { // in stack still
			curLayer.Size += layer.Size
			_, err = curRefTree.Stack(rTree)
			if err != nil {
				return nil, fmt.Errorf("error to stacking trees")
			}
		}
		if layer.Digest == img.Base.TopLayer { // end of stack
			newLayers = append(newLayers, curLayer)
			newRefTree = append(newRefTree, curRefTree)
			isStack = false
		}
		if !isStack {
			newLayers = append(newLayers, layer)
			newRefTree = append(newRefTree, rTree)
		}
	}
	analysis.RefTrees = newRefTree
	analysis.Layers = newLayers

	return analysis, nil



}