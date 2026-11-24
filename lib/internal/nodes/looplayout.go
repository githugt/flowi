// SPDX-License-Identifier: Apache-2.0

package nodes

// The layouts of a loop's body on the canvas.
const (
	// LoopLayoutVertical stacks the steps of a loop's body top to bottom in a box under the loop.
	LoopLayoutVertical = "vertical"
	// LoopLayoutHorizontal puts them in one row in that box.
	LoopLayoutHorizontal = "horizontal"
)

// loopLayoutParam is the layout field both loops (batch and run_till) have. It only changes how the canvas draws
// the steps inside the loop: the run is the same either way.
func loopLayoutParam() Param {
	return Param{
		Name: "layout", Kind: ParamEnum, Enum: []string{LoopLayoutVertical, LoopLayoutHorizontal}, Default: LoopLayoutVertical,
		Doc: "How the canvas draws the steps in the body: vertical (default) stacks them top to bottom in a box under the loop, horizontal puts them in one row. It changes only the picture, not the run.",
	}
}
