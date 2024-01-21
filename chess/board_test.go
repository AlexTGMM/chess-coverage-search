package chess

import "testing"

// TODO: add more testing.  This is just the testing that came up during debugging

func TestBoard_settleSupportGraph(t *testing.T) {
	board, err := MinimalBoard{}.RebuildBoard()
	if err != nil {
		t.Logf("unexpected error rebuilding board")
		t.FailNow()
	}
	for x, row := range board {
		for y, currCell := range row {
			if len(currCell.supportedBy) > 0 {
				t.Logf("cell is unexpectedly supported: %d, %d", x, y)
				t.FailNow()
			}
		}
	}
	board.getCell(newPointUnsafe(0, 0)).piece = QUEEN
	if board.getCell(newPointUnsafe(0, 0)).piece != QUEEN {
		t.Logf("queen failed to stay set")
		t.FailNow()
	}
	err = board.settleSupportGraph()
	if err != nil {
		t.Logf("unexpected error settling support graph")
		t.FailNow()
	}

	for i := 1; i < BOARD_SIZE; i++ {
		if !board.getCell(newPointUnsafe(i, 0)).supportedBy.has(newPointUnsafe(0, 0)) {
			t.Logf("failed to set coverage at %d,0", i)
			t.FailNow()
		}
		if !board.getCell(newPointUnsafe(0, i)).supportedBy.has(newPointUnsafe(0, 0)) {
			t.Logf("failed to set coverage at 0,%d", i)
			t.FailNow()
		}
		if !board.getCell(newPointUnsafe(i, i)).supportedBy.has(newPointUnsafe(0, 0)) {
			t.Logf("failed to set coverage at %d,%d", i, i)
			t.FailNow()
		}
	}
}

func TestBoard_GetCoverageLevel(t *testing.T) {
	for _, boardFunc := range getAllBasicCompleteBoards() {
		minimalBoard, expectedScore, name := boardFunc()
		t.Run(name, func(t *testing.T) {
			board, err := minimalBoard.RebuildBoard()
			if err != nil {
				t.Errorf("failed to rebuild board: %v", err)
			}
			if board.GetCoverageLevel() != BOARD_SIZE*BOARD_SIZE {
				t.Errorf("solved board not marked fully covered: %d", board.GetCoverageLevel())
			}
			score, err := board.Score()
			if err != nil {
				t.Errorf("failed to score board: %v", err)
			}
			if expectedScore != score {
				t.Errorf("board did not score as expected.  wanted %d but got %d", expectedScore, score)
			}
		})
	}
}

// these are all complete boards, but in no way optimal
func getAllBasicCompleteBoards() []func() (MinimalBoard, int, string) {
	return []func() (MinimalBoard, int, string){
		getBasicCompletePawnBoard,
		getBasicCompleteBishopBoard,
		getBasicCompleteKnightBoard,
		getBasicCompleteRookBoard,
		getBasicCompleteQueenBoard,
	}
}

// one row of rooks
func getBasicCompleteRookBoard() (MinimalBoard, int, string) {
	board := MinimalBoard{}
	for i := 0; i < BOARD_SIZE; i++ {
		board.board[i] = ROOK
	}
	score, _ := GetScore(ROOK)
	return board, score * BOARD_SIZE, "rook board"
}

// half of the rows full of bishops
func getBasicCompleteBishopBoard() (MinimalBoard, int, string) {
	board := MinimalBoard{}
	for i := 0; i < BOARD_SIZE*4; i++ {
		board.board[i] = BISHOP
	}
	score, _ := GetScore(BISHOP)
	return board, score * BOARD_SIZE * 4, "bishop board"
}

// one row of rooks, rest of board full of pawns
func getBasicCompletePawnBoard() (MinimalBoard, int, string) {
	board := MinimalBoard{}
	for i := 0; i < BOARD_SIZE; i++ {
		board.board[i] = ROOK
	}
	for i := BOARD_SIZE; i < BOARD_SIZE*BOARD_SIZE; i++ {
		board.board[i] = PAWN
	}
	rookScore, _ := GetScore(ROOK)
	pawnScore, _ := GetScore(PAWN)
	return board, (rookScore * BOARD_SIZE) + (pawnScore * BOARD_SIZE * (BOARD_SIZE - 1)), "pawn board"
}

// one row of queens
func getBasicCompleteQueenBoard() (MinimalBoard, int, string) {
	board := MinimalBoard{}
	for i := 0; i < BOARD_SIZE; i++ {
		board.board[i] = QUEEN
	}
	score, _ := GetScore(QUEEN)
	return board, score * BOARD_SIZE, "queen board"
}

// full board of knights
func getBasicCompleteKnightBoard() (MinimalBoard, int, string) {
	board := MinimalBoard{}
	for i := 0; i < BOARD_SIZE*BOARD_SIZE; i++ {
		board.board[i] = KNIGHT
	}
	score, _ := GetScore(KNIGHT)
	return board, score * BOARD_SIZE * BOARD_SIZE, "knight board"
}
