package chess

import "fmt"

type Piece byte

// names for all the pieces
const (
	NONE Piece = iota
	PAWN
	KNIGHT
	BISHOP
	ROOK
	QUEEN
)

// scores for all the pieces
var scores = map[Piece]int{
	PAWN:   1,
	KNIGHT: 3,
	BISHOP: 3,
	ROOK:   5,
	QUEEN:  9,
}

// printable runes for all the pieces
var runes = map[Piece]rune{
	// powershell is missing these characters
	//NONE:   '_',
	//PAWN:   '♟',
	//KNIGHT: '♞',
	//BISHOP: '♝',
	//ROOK:   '♜',
	//QUEEN:  '♛',
	NONE:   '_',
	PAWN:   'P',
	KNIGHT: 'K',
	BISHOP: 'B',
	ROOK:   'R',
	QUEEN:  'Q',
}

func GetScore(piece Piece) (int, error) {
	score, ok := scores[piece]
	if !ok {
		panic(fmt.Sprintf("tried to get score for unknown piece: %d", piece))
	}
	return score, nil
}

func (p Piece) GetRune() rune {
	return runes[p]
}

// getCoverage returns the coverage for all the pieces, given a point and a Board
func getCoverage(board *Board, p point, piece Piece) (pointSet, error) {
	switch piece {
	case PAWN:
		return pawnCoverage(p), nil
	case KNIGHT:
		return knightCoverage(p), nil
	case BISHOP:
		return bishopCoverage(board, p), nil
	case ROOK:
		return rookCoverage(board, p), nil
	case QUEEN:
		return queenCoverage(board, p), nil
	default:
		return nil, fmt.Errorf("attempted to get coverage for unknown piece: %d", piece)
	}
}

func pawnCoverage(p point) pointSet {
	var result pointSet = make(map[point]struct{})
	if possiblePoint, valid := p.add(1, 1); valid {
		result.put(possiblePoint)
	}
	if possiblePoint, valid := p.add(1, -1); valid {
		result.put(possiblePoint)
	}
	return result
}

func knightCoverage(p point) pointSet {
	var result pointSet = make(map[point]struct{})
	if possiblePoint, valid := p.add(1, 2); valid {
		result.put(possiblePoint)
	}
	if possiblePoint, valid := p.add(2, 1); valid {
		result.put(possiblePoint)
	}
	if possiblePoint, valid := p.add(-1, 2); valid {
		result.put(possiblePoint)
	}
	if possiblePoint, valid := p.add(-2, 1); valid {
		result.put(possiblePoint)
	}
	if possiblePoint, valid := p.add(1, -2); valid {
		result.put(possiblePoint)
	}
	if possiblePoint, valid := p.add(2, -1); valid {
		result.put(possiblePoint)
	}
	if possiblePoint, valid := p.add(-1, -2); valid {
		result.put(possiblePoint)
	}
	if possiblePoint, valid := p.add(-2, -1); valid {
		result.put(possiblePoint)
	}
	return result
}

func bishopCoverage(board *Board, p point) pointSet {
	var result pointSet = make(map[point]struct{})
	var next point
	var valid bool
	for next, valid = p.add(1, 1); valid && board.isEmpty(next); next, valid = next.add(1, 1) {
		result.put(next)
	}
	if valid {
		result.put(next)
	}
	for next, valid = p.add(-1, 1); valid && board.isEmpty(next); next, valid = next.add(-1, 1) {
		result.put(next)
	}
	if valid {
		result.put(next)
	}
	for next, valid = p.add(1, -1); valid && board.isEmpty(next); next, valid = next.add(1, -1) {
		result.put(next)
	}

	if valid {
		result.put(next)
	}
	for next, valid = p.add(-1, -1); valid && board.isEmpty(next); next, valid = next.add(-1, -1) {
		result.put(next)
	}
	if valid {
		result.put(next)
	}
	return result
}

func rookCoverage(board *Board, p point) pointSet {
	var result pointSet = make(map[point]struct{})
	var next point
	var valid bool
	for next, valid = p.add(1, 0); valid && board.isEmpty(next); next, valid = next.add(1, 0) {
		result.put(next)
	}
	if valid {
		result.put(next)
	}
	for next, valid = p.add(0, 1); valid && board.isEmpty(next); next, valid = next.add(0, 1) {
		result.put(next)
	}
	if valid {
		result.put(next)
	}
	for next, valid = p.add(-1, 0); valid && board.isEmpty(next); next, valid = next.add(-1, 0) {
		result.put(next)
	}
	if valid {
		result.put(next)
	}
	for next, valid = p.add(0, -1); valid && board.isEmpty(next); next, valid = next.add(0, -1) {
		result.put(next)
	}
	if valid {
		result.put(next)
	}
	return result
}

func queenCoverage(board *Board, p point) pointSet {
	result := bishopCoverage(board, p)
	for newP := range rookCoverage(board, p) {
		result.put(newP)
	}
	return result
}
