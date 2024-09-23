// Hierarchical Spatial Hash Grid: HSHG
// https://gist.github.com/kirbysayshi/1760774

package main

/*
#include <stdio.h>
#include <stdint.h>

typedef void (*queryCb)(int, int);
static void helper(queryCb cb, int objA, int objB){
	cb(objA, objB);
}

typedef uintptr_t gouintptr_t;
*/
import "C"

import (
	"fmt"
	"math"
	"sync"
	"unsafe"
)

const (
	MAX_OBJECT_CELL_DENSITY = 1.0 / 8.0
	INITIAL_GRID_LENGTH     = 256
	HIERARCHY_FACTOR        = 2
	HIERARCHY_FACTOR_SQRT   = 1.4142135623730951
)

type AABB struct {
	min    [2]float64
	max    [2]float64
	active bool
}

type Entity struct {
	id                   int
	min                  [2]float64
	max                  [2]float64
	active               bool
	globalObjectsIndex   int
	objectContainerIndex int
	allGridObjectsIndex  int
	hash                 int
	grid                 *Grid
}

type Cell struct {
	objectContainer     []*Entity
	neighborOffsetArray []int
	occupiedCellsIndex  int
	allCellsIndex       int
}

var metas = make(map[int]*Entity)
var metasCount int

var gridCount int = 0

// Check if two AABBs overlap
func testAABBOverlap(a, b *Entity) bool {
	if !a.active && !b.active {
		return false
	}
	return !(a.min[0] > b.max[0] || a.min[1] > b.max[1] || a.max[0] < b.min[0] || a.max[1] < b.min[1])
}

// Get the longest edge of an AABB
func getLongestAABBEdge(min, max [2]float64) float64 {
	return math.Max(math.Abs(max[0]-min[0]), math.Abs(max[1]-min[1]))
}

// Check if an object is in HSHG
func checkIfInHSHG(obj *Entity) bool {
	_, exists := metas[obj.id]
	return exists
}

type Grid struct {
	cellSize           float64
	inverseCellSize    float64
	rowColumnCount     int
	xyHashMask         int
	occupiedCells      []Cell
	allCells           []Cell
	allObjects         []*Entity
	sharedInnerOffsets []int
	id                 int
	mu                 sync.Mutex // For thread safety
}

func NewGrid(cellSize float64, cellCount int) *Grid {
	grid := &Grid{
		cellSize:        cellSize,
		inverseCellSize: 1.0 / cellSize,
		rowColumnCount:  int(math.Sqrt(float64(cellCount))),
		xyHashMask:      int(math.Sqrt(float64(cellCount))) - 1,
		id:              gridCount,
	}
	gridCount++
	grid.allCells = make([]Cell, grid.rowColumnCount*grid.rowColumnCount)
	return grid
}

func (g *Grid) init() {
	gridLength := len(g.allCells)
	wh := g.rowColumnCount
	innerOffsets := []int{wh - 1, wh, wh + 1, -1, 0, 1, -1 - wh, -wh, -wh + 1}
	g.sharedInnerOffsets = innerOffsets

	for i := 0; i < gridLength; i++ {
		cell := Cell{}
		y := i / wh
		x := i - y*wh

		isOnRightEdge := (x+1)%wh == 0
		isOnLeftEdge := x%wh == 0
		isOnTopEdge := (y+1)%wh == 0
		isOnBottomEdge := y%wh == 0

		if isOnRightEdge || isOnLeftEdge || isOnTopEdge || isOnBottomEdge {
			rightOffset := 0
			leftOffset := 0
			topOffset := 0
			bottomOffset := 0

			if isOnRightEdge {
				rightOffset = -wh + 1
			} else {
				rightOffset = 1
			}
			if isOnLeftEdge {
				leftOffset = wh - 1
			} else {
				leftOffset = -1
			}
			if isOnTopEdge {
				topOffset = -gridLength + wh
			} else {
				topOffset = wh
			}
			if isOnBottomEdge {
				bottomOffset = gridLength - wh
			} else {
				bottomOffset = -wh
			}

			cell.neighborOffsetArray = []int{
				leftOffset + topOffset,
				topOffset,
				rightOffset + topOffset,
				leftOffset,
				0,
				rightOffset,
				leftOffset + bottomOffset,
				bottomOffset,
				rightOffset + bottomOffset,
			}
		} else {
			cell.neighborOffsetArray = g.sharedInnerOffsets
		}

		cell.allCellsIndex = i
		g.allCells[i] = cell
	}
}

func (g *Grid) insert(obj *Entity, hash int) {
	if hash == -1 {
		hash = g.toHash(obj.min[0], obj.min[1])
	}

	g.mu.Lock()

	obj.grid = g

	targetCell := &g.allCells[hash]
	if len(targetCell.objectContainer) == 0 {
		targetCell.occupiedCellsIndex = len(g.occupiedCells)
		g.occupiedCells = append(g.occupiedCells, *targetCell)
	}
	obj.objectContainerIndex = len(targetCell.objectContainer)
	obj.hash = hash
	obj.grid = g
	obj.allGridObjectsIndex = len(g.allObjects)
	targetCell.objectContainer = append(targetCell.objectContainer, obj)
	g.allObjects = append(g.allObjects, obj)

	g.mu.Unlock()

	if float64(len(g.allObjects))/float64(len(g.allCells)) > MAX_OBJECT_CELL_DENSITY {
		g.expandGrid()
	}
}

func (g *Grid) toHash(x, y float64) int {
	var xHash, yHash int
	var i float64
	if x < 0 {
		i = -x * g.inverseCellSize
		xHash = g.rowColumnCount - 1 - int(i)&g.xyHashMask
	} else {
		i = x * g.inverseCellSize
		xHash = int(i) & g.xyHashMask
	}

	if y < 0 {
		i = -y * g.inverseCellSize
		yHash = g.rowColumnCount - 1 - int(i)&g.xyHashMask
	} else {
		i = y * g.inverseCellSize
		yHash = int(i) & g.xyHashMask
	}
	return xHash + yHash*g.rowColumnCount
}

func (g *Grid) remove(obj *Entity) {
	g.mu.Lock()
	defer g.mu.Unlock()

	hash := obj.hash
	containerIndex := obj.objectContainerIndex
	allGridObjectsIndex := obj.allGridObjectsIndex

	cell := &g.allCells[hash]

	if len(cell.objectContainer) == 1 {
		cell.objectContainer = cell.objectContainer[:0]
		if cell.occupiedCellsIndex == len(g.occupiedCells)-1 {
			g.occupiedCells = g.occupiedCells[:len(g.occupiedCells)-1]
		} else {
			replacementCell := g.occupiedCells[len(g.occupiedCells)-1]
			g.occupiedCells = g.occupiedCells[:len(g.occupiedCells)-1]
			replacementCell.occupiedCellsIndex = cell.occupiedCellsIndex
			g.occupiedCells[cell.occupiedCellsIndex] = replacementCell
		}
		cell.occupiedCellsIndex = -1
	} else {
		if containerIndex == len(cell.objectContainer)-1 {
			cell.objectContainer = cell.objectContainer[:len(cell.objectContainer)-1]
		} else {
			replacementObj := cell.objectContainer[len(cell.objectContainer)-1]
			cell.objectContainer = cell.objectContainer[:len(cell.objectContainer)-1]
			replacementObj.objectContainerIndex = containerIndex
			cell.objectContainer[containerIndex] = replacementObj
		}
	}

	if allGridObjectsIndex == len(g.allObjects)-1 {
		g.allObjects = g.allObjects[:len(g.allObjects)-1]
	} else {
		replacementObj := g.allObjects[len(g.allObjects)-1]
		g.allObjects = g.allObjects[:len(g.allObjects)-1]
		replacementObj.allGridObjectsIndex = allGridObjectsIndex
		g.allObjects[allGridObjectsIndex] = replacementObj
	}
}

func (g *Grid) expandGrid() {
	g.mu.Lock()

	// Save old state before expanding
	oldCells := g.allCells
	oldOccupiedCells := g.occupiedCells

	oldCellCount := len(oldCells)
	newCellCount := oldCellCount * 4 // Quadrupling the grid size
	newCells := make([]Cell, newCellCount)

	g.allCells = newCells
	g.occupiedCells = nil // Reset occupied cells

	// Update the cellSize and rehash all objects
	g.cellSize *= 2

	for _, cell := range oldOccupiedCells {
		for _, obj := range cell.objectContainer {
			// Reinsert object into the new grid
			newHash := g.toHash(obj.min[0], obj.min[1])

			targetCell := &g.allCells[newHash]
			if len(targetCell.objectContainer) == 0 {
				targetCell.occupiedCellsIndex = len(g.occupiedCells)
				g.occupiedCells = append(g.occupiedCells, *targetCell)
			}

			obj.objectContainerIndex = len(targetCell.objectContainer)
			obj.hash = newHash
			obj.allGridObjectsIndex = len(g.allObjects)
			targetCell.objectContainer = append(targetCell.objectContainer, obj)
			g.allObjects = append(g.allObjects, obj)
		}
	}

	g.mu.Unlock()
}

type HSHG struct {
	grids         []*Grid
	globalObjects []*Entity
}

func NewHSHG() *HSHG {
	return &HSHG{
		grids:         make([]*Grid, 0),
		globalObjects: make([]*Entity, 0),
	}
}

// Add an object to HSHG
func (h *HSHG) insert(min, max [2]float64, active bool) int {
	objSize := getLongestAABBEdge(min, max)

	obj := &Entity{
		id:                   metasCount,
		min:                  min,
		max:                  max,
		active:               active,
		globalObjectsIndex:   len(h.globalObjects),
		objectContainerIndex: 0,
		allGridObjectsIndex:  0,
		hash:                 0,
		grid:                 nil,
	}

	metas[metasCount] = obj
	metasCount++
	h.globalObjects = append(h.globalObjects, obj)

	if len(h.grids) == 0 {
		cellSize := objSize * HIERARCHY_FACTOR_SQRT
		newGrid := NewGrid(cellSize, INITIAL_GRID_LENGTH)
		newGrid.init()
		newGrid.insert(obj, -1)
		h.grids = append(h.grids, newGrid)
	} else {
		x := 0.0
		for i := 0; i < len(h.grids); i++ {
			oneGrid := h.grids[i]
			x = oneGrid.cellSize
			if objSize < x {
				x /= HIERARCHY_FACTOR
				if objSize < x {
					for objSize < x {
						x /= HIERARCHY_FACTOR
					}
					newGrid := NewGrid(x*HIERARCHY_FACTOR, INITIAL_GRID_LENGTH)
					newGrid.init()
					newGrid.insert(obj, -1)
					h.grids = append(h.grids[:i], append([]*Grid{newGrid}, h.grids[i:]...)...)
					return obj.id
				} else {
					oneGrid.insert(obj, -1)
					return obj.id
				}
			}
		}
		for objSize >= x {
			x *= HIERARCHY_FACTOR
		}
		newGrid := NewGrid(x, INITIAL_GRID_LENGTH)
		newGrid.init()
		newGrid.insert(obj, -1)
		h.grids = append(h.grids, newGrid)
	}

	return obj.id
}

// Function to remove an object from HSHG
func (h *HSHG) remove(obj *Entity) {
	if !checkIfInHSHG(obj) {
		fmt.Println("Object is not in HSHG.")
		return
	}

	globalObjectsIndex := obj.globalObjectsIndex
	if globalObjectsIndex == len(h.globalObjects)-1 {
		h.globalObjects = h.globalObjects[:len(h.globalObjects)-1]
	} else {
		replacementObj := h.globalObjects[len(h.globalObjects)-1]
		h.globalObjects = h.globalObjects[:len(h.globalObjects)-1]
		replacementObj.globalObjectsIndex = globalObjectsIndex
		h.globalObjects[globalObjectsIndex] = replacementObj
	}

	obj.grid.remove(obj)
	delete(metas, obj.id)
}

// Function to update the HSHG structure
func (h *HSHG) update() {
	for i := 0; i < len(h.globalObjects); i++ {
		obj := h.globalObjects[i]
		grid := obj.grid
		if grid == nil {
			fmt.Printf("Object %d has no grid assigned.\n", obj.id)
			continue
		}
		newObjHash := grid.toHash(obj.min[0], obj.min[1])

		if newObjHash != obj.hash {
			grid.remove(obj)
			grid.insert(obj, newObjHash)
		}
	}
}

func (h *HSHG) updateAABB(obj *Entity, min, max [2]float64, active bool) {
	if !checkIfInHSHG(obj) {
		fmt.Println("Object is not in HSHG.")
		return
	}

	obj.min = min
	obj.max = max
	obj.active = active
}

// Query for collision pairs
func (h *HSHG) query() [][2]Entity {
	var possibleCollisions [][2]Entity

	for _, grid := range h.grids {
		for _, cell := range grid.occupiedCells {
			for k := 0; k < len(cell.objectContainer); k++ {
				objA := cell.objectContainer[k]
				if !objA.active {
					continue
				}
				for l := k + 1; l < len(cell.objectContainer); l++ {
					objB := cell.objectContainer[l]
					if !objB.active {
						continue
					}
					if testAABBOverlap(objA, objB) {
						possibleCollisions = append(possibleCollisions, [2]Entity{*objA, *objB})
					}
				}
			}

			for _, offset := range cell.neighborOffsetArray {
				adjacentCell := grid.allCells[cell.allCellsIndex+offset]
				for k := 0; k < len(adjacentCell.objectContainer); k++ {
					objA := adjacentCell.objectContainer[k]
					if !objA.active {
						continue
					}
					for l := k + 1; l < len(adjacentCell.objectContainer); l++ {
						objB := adjacentCell.objectContainer[l]
						if !objB.active {
							continue
						}
						if testAABBOverlap(objA, objB) {
							possibleCollisions = append(possibleCollisions, [2]Entity{*objA, *objB})
						}
					}
				}
			}
		}
	}

	return possibleCollisions
}

// END HSHG

var hshg = NewHSHG()

//export insertEntity
func insertEntity(minX, minY, maxX, maxY C.double, active C.int) C.int {
	min := [2]float64{float64(minX), float64(minY)}
	max := [2]float64{float64(maxX), float64(maxY)}
	id := hshg.insert(min, max, active != 0)
	return C.int(id)
}

//export removeEntity
func removeEntity(id C.int) {
	entity := metas[int(id)]

	hshg.remove(entity)
}

//export updateEntity
func updateEntity(id C.int, minX, minY, maxX, maxY C.double, active C.int) {
	min := [2]float64{float64(minX), float64(minY)}
	max := [2]float64{float64(maxX), float64(maxY)}

	entity := metas[int(id)]

	hshg.updateAABB(entity, min, max, active != 0)
}

//export updateHSHG
func updateHSHG() {
	hshg.update()
}

//export queryHSHG
func queryHSHG() *C.int {
	collisions := hshg.query()
	count := len(collisions)

	if count == 0 {
		return nil
	}

	pairArray := C.malloc(C.size_t(count * 2 * int(C.size_t(unsafe.Sizeof(C.int(0))))))
	pairs := (*[1 << 30]C.int)(pairArray)[: count*2 : count*2]

	for i, pair := range collisions {
		pairs[i*2] = C.int(pair[0].id)
		pairs[i*2+1] = C.int(pair[1].id)
	}

	return (*C.int)(pairArray)
}

//export getCollisionCount
func getCollisionCount() C.int {
	return C.int(len(hshg.query()))
}

func main() {
	println("[GO-HSHG] Started.")
	select {}
}
