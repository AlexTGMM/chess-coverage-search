package chess

import (
	"fmt"
	"strconv"
	"strings"
)

// BOARD_SIZE size of the board to attempt to solve
const BOARD_SIZE int = 8

// Board a fully inflated board to be worked on
type Board [BOARD_SIZE][BOARD_SIZE]*cell

// cell a cell for the working board
type cell struct {
	piece       Piece
	supports    pointSet
	supportedBy pointSet
}

// point This algorithm instantiates a lot of these while working, so use the smallest data type that makes sense.
// using this instead of a struct requires extra math, but it more than offsets the time that was spent allocating
// and comparing structs.  int8 will break if board size > 15
type point int8

// pointSet a map wrapper to make a set for storing points
type pointSet map[point]struct{}

var SENTINEL = struct{}{}

func (ps pointSet) put(p point)      { ps[p] = SENTINEL }
func (ps pointSet) has(p point) bool { _, ok := ps[p]; return ok }

// MinimalBoard the representation used to store boards that are not actively being worked on
type MinimalBoard struct {
	board     [BOARD_SIZE * BOARD_SIZE]Piece
	Heuristic float32
	IsSolved  bool
	Score     int
	Coverage  int
}

// MinimalBoardSet a map wrapper for tracking sets of boards
type MinimalBoardSet map[MinimalBoard]struct{}

func (m MinimalBoardSet) Put(board MinimalBoard)           { m[board] = SENTINEL }
func (m MinimalBoardSet) Contains(board MinimalBoard) bool { _, ok := m[board]; return ok }

// copy Does *NOT* copy support
func (c *cell) copy() *cell {
	result := &cell{piece: c.piece}
	return result
}

// addSupport is used to mark cells that this cell is being supported by
func (c *cell) addSupport(p point) {
	if c.supportedBy == nil {
		c.supportedBy = make(map[point]struct{}, BOARD_SIZE)
	}
	c.supportedBy.put(p)
}

// supportOther is used to mark this cell as supporting another cell
func (c *cell) supportOther(p point) {
	if c.supports == nil {
		c.supports = make(map[point]struct{}, BOARD_SIZE*2) // size of a rook/queen in the middle of the board
	}
	c.supports.put(p)
}

// clearSupport nils out the support sets.  Does not clear them because they may no longer be needed
func (c *cell) clearSupport() {
	c.supports = nil
	c.supportedBy = nil
}

// newPoint returns the new point and an indicator of whether it is valid.
// To be used when creating new points from pieces
func newPoint(x, y int) (point, bool) {
	return point((x * BOARD_SIZE) + y),
		!(x < 0 || x >= BOARD_SIZE || y < 0 || y >= BOARD_SIZE)
}

// newPointUnsafe returns a point without validating it.
// To be used when creating points during iteration
func newPointUnsafe(x, y int) point {
	return point((x * BOARD_SIZE) + y)
}

// add adds x,y offsets to points
func (p point) add(x, y int8) (point, bool) {
	newX := p.x() + x
	newY := p.y() + y
	return p + point((x*int8(BOARD_SIZE))+y),
		!(newX < 0 || newX >= int8(BOARD_SIZE) || newY < 0 || newY >= int8(BOARD_SIZE))
}

// x is the x value of the point
func (p point) x() int8 {
	return int8(p) / int8(BOARD_SIZE)
}

// y is the y value of the point
func (p point) y() int8 {
	return int8(p) % int8(BOARD_SIZE)
}

// getCell gets a cell from the board using a point
func (b *Board) getCell(p point) *cell {
	return b[p.x()][p.y()]
}

// isEmpty reports if a cell contains a piece
func (b *Board) isEmpty(p point) bool {
	return b.getCell(p).piece == NONE
}

// getAllCoverage this reports contextual coverage that each piece would provide on a
// given cell of a given board.  This takes into account board boundaries (knight and
// pawn) and blocked cells (rook, bishop, queen)
func (b *Board) getAllCoverage(p point) (map[Piece]pointSet, error) {
	result := make(map[Piece]pointSet, 5)
	coverage, err := getCoverage(b, p, PAWN)
	if err != nil {
		return nil, fmt.Errorf("failed to get pawn coverage: %w", err)
	}
	result[PAWN] = coverage
	coverage, err = getCoverage(b, p, KNIGHT)
	if err != nil {
		return nil, fmt.Errorf("failed to get knight coverage: %w", err)
	}
	result[KNIGHT] = coverage
	coverage, err = getCoverage(b, p, ROOK)
	if err != nil {
		return nil, fmt.Errorf("failed to get rook coverage: %w", err)
	}
	result[ROOK] = coverage
	coverage, err = getCoverage(b, p, BISHOP)
	if err != nil {
		return nil, fmt.Errorf("failed to get bishop coverage: %w", err)
	}
	result[BISHOP] = coverage
	coverage, err = getCoverage(b, p, QUEEN)
	if err != nil {
		return nil, fmt.Errorf("failed to get queen coverage: %w", err)
	}
	result[QUEEN] = coverage

	return result, nil
}

// getMinimalBoard returns a deflated copy of a Board
func (b *Board) getMinimalBoard(heuristic func(board *Board) (float32, error)) (MinimalBoard, error) {
	heuristicScore, err := heuristic(b)
	if err != nil {
		return MinimalBoard{}, fmt.Errorf("failed to calculate heuristic while minimizing: %w", err)
	}
	score, err := b.Score()
	if err != nil {
		return MinimalBoard{}, fmt.Errorf("failed to score board while minimizing: %w", err)
	}
	result := MinimalBoard{
		Heuristic: heuristicScore,
		IsSolved:  b.GetCoverageLevel() == BOARD_SIZE*BOARD_SIZE,
		Score:     score,
		Coverage:  b.GetCoverageLevel(),
	}
	for x, row := range b {
		for y, c := range row {
			result.board[(x*BOARD_SIZE)+y] = c.piece
		}
	}
	return result, nil
}

// GetCoverageLevel reports how many of the cells on the board are covered
func (b *Board) GetCoverageLevel() (result int) {
	for _, row := range b {
		for _, currCell := range row {
			if len(currCell.supportedBy) > 0 {
				result++
			}
		}
	}
	return
}

// Score reports the piece based score for a board
func (b *Board) Score() (int, error) {
	result := 0
	for _, row := range b {
		for _, currCell := range row {
			if currCell.piece != NONE {
				score, err := GetScore(currCell.piece)
				if err != nil {
					return result, fmt.Errorf("failed to score board: %w", err)
				}
				result += score
			}
		}
	}
	return result, nil
}

// copy Does *NOT* copy support
func (b *Board) copy() *Board {
	newBoard := &Board{}
	for x, row := range b {
		for y, currCell := range row {
			newBoard[x][y] = currCell.copy()
		}
	}
	return newBoard
}

// settleSupportGraph calculates the support graph for a given cell.  This is one of the
// most expensive calls in this algorithm, and overall performance could be significantly
// improved if this function was improved.
func (b *Board) settleSupportGraph() error {
	for _, row := range b {
		for _, currCell := range row {
			currCell.clearSupport()
		}
	}
	// find all the pieces on the board
	for x, row := range b {
		for y, currCell := range row {
			// when a piece is found, calculate its coverage and mark the board
			if currCell.piece != NONE {
				currPoint := newPointUnsafe(x, y)
				coverage, err := getCoverage(b, currPoint, currCell.piece)
				if err != nil {
					return fmt.Errorf("failed to get coverage of piece: %w", err)
				}
				currCell.supports = coverage
				for coveredPoint := range coverage {
					b.getCell(coveredPoint).addSupport(currPoint)
				}
			}
		}
	}
	return nil
}

// RebuildBoard re-inflates a MinimalBoard, and rebuilds the support graph
func (m MinimalBoard) RebuildBoard() (*Board, error) {
	board := &Board{}
	for i, piece := range m.board {
		board[i/BOARD_SIZE][i%BOARD_SIZE] = &cell{piece: piece}
	}
	err := board.settleSupportGraph()
	if err != nil {
		return nil, fmt.Errorf("failed to settle support graph: %w", err)
	}
	return board, nil
}

func (m MinimalBoard) String() string {
	result := strings.Builder{}
	for x := 0; x < BOARD_SIZE; x++ {
		for y := 0; y < BOARD_SIZE; y++ {
			result.WriteRune(m.board[(y*BOARD_SIZE)+x].GetRune())
		}
		result.WriteString("\n")
	}
	result.WriteString(
		fmt.Sprintf("Score: %d\tHeuristic: %f\tSolved: %t\tCoverage%d",
			m.Score, m.Heuristic, m.IsSolved, m.Coverage))
	return result.String()
}

// ProposeBoards is used to calculate all the potential boards that could be reached from a given board.  It
// is where the algorithm spends most of its time, and any additional early pruning techniques would benefit
// it greatly
func (b *Board) ProposeBoards(heuristic func(board *Board) (float32, error)) (MinimalBoardSet, error) {
	result := MinimalBoardSet{}
	// check each cell
	for x, row := range b {
		for y, currCell := range row {
			// if the cell is occupied, skip it
			if currCell.piece != NONE {
				continue
			}
			// calculate coverages for each possible piece at this point
			currCellPoint, _ := newPoint(x, y)
			coverages, err := b.getAllCoverage(currCellPoint)
			if err != nil {
				return nil, fmt.Errorf("failed to get coverages: %w", err)
			}
			// check each pieces coverages
			for piece, coverage := range coverages {
				var coveredNew bool
				// check if the coverage would cover any new cells
				for currThreatenedPoint := range coverage {
					if len(b.getCell(currThreatenedPoint).supportedBy) == 0 {
						coveredNew = true
						break
					}
				}
				// if the piece would change the state of the board, create a new
				// board with that modification
				if coveredNew {
					// NB: all work here is done on the *copy*, not modifying the original board
					newBoard := b.copy()
					newBoard[currCellPoint.x()][currCellPoint.y()].piece = piece
					err = newBoard.settleSupportGraph()
					if err != nil {
						return nil, fmt.Errorf("failed to settle cloned board: %w", err)
					}
					// once we have the new board, calculate its reductions
					reducedBoards, err := newBoard.reduce()
					if err != nil {
						return nil, fmt.Errorf("failed to reduce cloned board: %w", err)
					}
					for _, reducedBoard := range reducedBoards {
						minimalBoard, err := reducedBoard.getMinimalBoard(heuristic)
						if err != nil {
							return nil, fmt.Errorf("failed to minimize cloned board: %w", err)
						}
						// and finally add the reduced boards to the possible next boards
						result.Put(minimalBoard)
					}
				}
			}
		}
	}
	return result, nil
}

// reduce is used to see if a board has any pieces that can be removed without effecting the coverage.  If
// there are any, it will return all possible permutations that don't affect the coverage.
func (b *Board) reduce() ([]*Board, error) {
	result := []*Board{}
	// check each cell to see if it's contributing
	for x, row := range b {
	cellLoop:
		for y, currCell := range row {
			if currCell.piece == NONE {
				continue
			}
			// a cell is not contributing, if it doesn't support any cells that
			// are not also supported by another cell
			for currPoint := range currCell.supports {
				if len(b.getCell(currPoint).supportedBy) == 1 {
					continue cellLoop
				}
			}
			// if a piece is found to be not contributing, copy the board, remove the piece,
			// and see if the new board reduces further
			newBoard := b.copy()
			newBoard.getCell(newPointUnsafe(x, y)).piece = NONE
			err := newBoard.settleSupportGraph()
			if err != nil {
				return nil, fmt.Errorf("failed to settle board while reducing: %w", err)
			}
			// recursively reduce each solution.  This can reach depth up to BOARD_SIZE*BOARD_SIZE, which means
			// that BOARD_SIZE would have to be significantly higher than anything this algorithm is close to
			// capable of before we have to worry about blowing out the stack
			reduceResult, err := newBoard.reduce()
			if err != nil {
				return nil, fmt.Errorf("failed to reduce board while reducing: %w", err)
			}
			result = append(result, reduceResult...)
		}
	}
	// if this board did not reduce, return only itself in the result set
	if len(result) == 0 {
		result = append(result, b)
	}
	return result, nil
}

// String this draws the board in negative x, y space
func (b *Board) String(heuristic func(board *Board) (float32, error)) string {
	result := strings.Builder{}
	for _, row := range b {
		for _, currCell := range row {
			if currCell.piece != NONE {
				result.WriteRune(currCell.piece.GetRune())
			} else {
				result.WriteString(strconv.Itoa(len(currCell.supportedBy)))
			}
		}
		result.WriteString("\n")
	}
	score, err := b.Score()
	if err != nil {
		return fmt.Sprintf("failed to calculate score while buildind string: %v", err)
	}
	heuristicScore, err := heuristic(b)
	if err != nil {
		return fmt.Sprintf("failed to calculate heuristic while buildind string: %v", err)
	}
	solved := b.GetCoverageLevel() == BOARD_SIZE*BOARD_SIZE
	coverage := b.GetCoverageLevel()
	result.WriteString(fmt.Sprintf("Score: %d\tHeuristic: %f\tSolved: %t\tCoverage: %d",
		score, heuristicScore, solved, coverage))
	return result.String()
}
