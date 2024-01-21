package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/AlexTGMM/chess-coverage-search/chess"
	"golang.org/x/sync/errgroup"
	"log"
	"os"
	"runtime"
	"runtime/pprof"
	"sort"
	"sync/atomic"
	"time"
)

const (
	WORK_QUEUE_SIZE_FACTOR = 8
	// NEW_BOARD_QUEUE_SIZE_FACTOR 5 pieces + 1 reduction per space
	NEW_BOARD_QUEUE_SIZE_FACTOR = chess.BOARD_SIZE * (5 + 1)
)

// command line flags to control profiling
var cpuProfile = flag.String("cpuprofile", "", "write cpu profile to file")
var memProfile = flag.String("memprofile", "", "write memory profile to `file`")
var timeout = flag.Int("timeout", 5, "profiling shutdown timeout in seconds")

func main() {
	flag.Parse()
	// set up cpu the profiler
	if *cpuProfile != "" {
		f, err := os.Create(*cpuProfile)
		if err != nil {
			log.Fatal(err)
		}
		err = pprof.StartCPUProfile(f)
		if err != nil {
			log.Fatal(err)
		}
		defer pprof.StopCPUProfile()
	}
	// grab a memory profile at the end
	defer func() {
		if *memProfile != "" {
			f, err := os.Create(*memProfile)
			if err != nil {
				log.Fatal("could not create memory profile: ", err)
			}
			defer func() {
				err = f.Close()
				if err != nil {
					log.Fatal(err)
				}
			}()
			runtime.GC() // get up-to-date statistics
			if err := pprof.WriteHeapProfile(f); err != nil {
				log.Fatal("could not write memory profile: ", err)
			}
		}
	}()

	cores := runtime.NumCPU()
	// make sure Go actually uses the extra cores
	runtime.GOMAXPROCS(cores)
	// run the solver
	err := run(cores)
	if err != nil {
		log.Fatal(err)
	}
}

// how many boards the workers have handled
var processed = atomic.Int64{}

// how many boards were presented back to the orchestrator that it had already seen
var duplicates = atomic.Int64{}

// the best solution score
var currBestScore = atomic.Int32{}

// how many boards are the workers currently handling.  Used for safe shutdown
var outstandingJobs = atomic.Int32{}

// the following two data structures account for the vast majority of memory used by the algorithm
// keep track of the unique boards the orchestrator has seen.  This grows monotonically
var seenBoards = chess.MinimalBoardSet{}

// the orchestrators edge set of boards yet to be sent back to the workers.  This
// grows much faster than it shrinks
var edgeSet []chess.MinimalBoard

func run(cores int) error {
	// this question makes the assertion that 28 is the best possible score for board size 8,
	// so let's constrain our solution to that or better
	// https://puzzling.stackexchange.com/questions/2907/how-many-chess-pieces-are-needed-to-control-every-square-on-the-board-no-piece?lq=1
	currBestScore.Store(28)

	// create an empty board to use as the solution root
	baseBoard := chess.MinimalBoard{}
	seenBoards.Put(baseBoard)
	edgeSet = append(edgeSet, baseBoard)

	// hoping that this will end up with one core running the orchestrator, the rest
	// of the cores running a worker, and the drawing thread bouncing between threads
	// as available
	// follow up:  profiling has confirmed this hunch is roughly what happens
	workers := cores - 1
	workQueueSize := workers * WORK_QUEUE_SIZE_FACTOR
	// set up the threading components
	eg, egctx := errgroup.WithContext(context.Background())
	workQueue := make(chan chess.MinimalBoard, workQueueSize)
	newBoardQueue := make(chan chess.MinimalBoard, workers*NEW_BOARD_QUEUE_SIZE_FACTOR)
	drawingQueue := make(chan chess.MinimalBoard)

	// start the threads
	for i := 0; i < workers; i++ {
		worker := makeWorker(egctx, workQueue, newBoardQueue)
		eg.Go(worker)
	}
	eg.Go(makeOrchestrator(egctx, workQueueSize, workQueue, newBoardQueue, drawingQueue))
	eg.Go(makeBoardDrawer(egctx, workQueue, newBoardQueue, drawingQueue))

	return eg.Wait()
}

// heuristic is a heuristic based on board coverage slightly biased towards piece efficiency
// NB: it is not admissible, so this isn't true A*
func heuristic(board *chess.Board) (float32, error) {
	score, err := board.Score()
	if err != nil {
		return 0, fmt.Errorf("failed to calculate score during heuristic: %w", err)
	}
	coverage := float32(board.GetCoverageLevel())
	return (coverage / float32(score)) + coverage, nil
}

func makeWorker(ctx context.Context, workQueue, newBoardQueue chan chess.MinimalBoard) func() error {
	return func() error {
		for {
			// pull a board from the work queue
			var minimalBoard chess.MinimalBoard
			select {
			case b, ok := <-workQueue:
				if !ok {
					return nil
				}
				// wrap board work in a function, so we can defer reporting the work done
				err := func() error {
					defer outstandingJobs.Add(-1)
					minimalBoard = b
					// reconstitute the board to begin working on it
					board, err := minimalBoard.RebuildBoard()
					if err != nil {
						return err
					}
					// gather boards that could be derived from this board within one game step
					proposedBoards, err := board.ProposeBoards(heuristic)
					if err != nil {
						return fmt.Errorf("failed to propose new boards: %w", err)
					}
					// add any boards that don't have too high of a score back to the work queue
					// this is only best effort, so when a new best score is found, some boards with too
					// high of a score may slip through.  This isn't an issue; they will be caught
					// later by the orchestrator
					for proposedBoard := range proposedBoards {
						if proposedBoard.Score <= int(currBestScore.Load()) {
							select {
							case newBoardQueue <- proposedBoard:
							case <-ctx.Done():
								return fmt.Errorf("context was closed")
							}
						}
					}
					outstandingJobs.Add(-1)
					return nil
				}()
				if err != nil {
					return nil
				}
			case <-ctx.Done():
				return fmt.Errorf("context was closed")
			}
		}
	}
}

func makeOrchestrator(ctx context.Context, workQueueSize int, workQueue, newBoardQueue, drawingQueue chan chess.MinimalBoard) func() error {
	return func() error {
		var scoreIsDirty bool
		now := time.Now()
		for {
			// if there is work to be done, add a board to the work queue
			if len(edgeSet) > 0 {
				// discard best boards from the edge set until the best board has an acceptable score
				tailIndex := len(edgeSet) - 1
				for edgeSet[tailIndex].Score > int(currBestScore.Load()) {
					edgeSet = edgeSet[:tailIndex]
					tailIndex--
				}
				// if there are any boards left, add try to add one to the work queue
				if len(edgeSet) > 0 {
					select {
					case <-ctx.Done():
						return fmt.Errorf("context expired on orchestrator")
					case workQueue <- edgeSet[tailIndex]:
						// iff the drawing queue is waiting, have it draw a board
						select {
						case drawingQueue <- edgeSet[tailIndex]:
						default:
						}
						// pop the board that was added
						edgeSet = edgeSet[:tailIndex]
						outstandingJobs.Add(1)
						processed.Add(1)
					default:
						// if the input queue isn't ready, just move on immediately
					}
				}
			}
			// tracks the number of boards added in one pass
			var newBoards int
			// this pulls boards from the work queue until the queue is empty.  This is done because
			// each worker is relatively slow and usually produces far more output than it consumes in input.
			// follow up: profiled and verified empirically that this hunch was correct and that workers are
			// spending effectively no time waiting for input, even though the producer spends very little time
			// producing it
		newBoardLoop:
			for {
				select {
				case <-ctx.Done():
					return fmt.Errorf("context expired on orchestrator")
				case newBoard, ok := <-newBoardQueue:
					if !ok {
						return fmt.Errorf("new board channel was unexpectedly closed")
					}
					// if the new board is already solved, update the score and print it
					if newBoard.IsSolved {
						if newBoard.IsSolved && newBoard.Score < int(currBestScore.Load()) {
							currBestScore.Store(int32(newBoard.Score))
							scoreIsDirty = true
						}
						// when printing solved boards, wait for the drawing thread to be ready, so
						// we don't miss any solutions
						select {
						case <-ctx.Done():
							return fmt.Errorf("context expired on orchestrator while drawing solution")
						case drawingQueue <- newBoard:
						}
					} else {
						// if the new board isn't solved, add it to the edge set to be sorted
						insertBoard(newBoard)
					}
					newBoards++
				default:
					// as soon as there new boards left in the queue, stop pulling
					break newBoardLoop
				}
				// this is the termination condition.  We terminate if we can't find any more boards to check
				// or if the profiling timout has expired
				if ((*cpuProfile != "" || *memProfile != "") && now.Add(time.Duration(*timeout)*time.Second).Before(time.Now())) ||
					(len(edgeSet) == 0 &&
						len(workQueue) == 0 &&
						len(newBoardQueue) == 0 &&
						outstandingJobs.Load() == 0) {
					close(workQueue)
					close(drawingQueue)
					// hack to make sure the workers stop if we're ending early to get the dump.  Without this,
					// workers can end up hung, waiting to write back to the result queue, trigger a panic and
					// prevent the profiling from being written.  The other option would be to busy wait on outstandingJobs
					if *cpuProfile != "" || *memProfile != "" {
					drain:
						for {
							select {
							case <-newBoardQueue:
							case <-time.NewTicker(50 * time.Millisecond).C:
								break drain
							}
						}
					}
					return nil
				}
			}
			// only sort the boards we may plan to use, unless the score has changed.  If
			// the score has changed, sort them all since we don't know how many may get discarded
			// TODO: might it be better to actually discard the boards that are no long in bounds,
			// and still only sort the tip of the edge set?  Probably.  Try this next
			offset := len(edgeSet) - (newBoards + workQueueSize)
			if offset < 0 || scoreIsDirty {
				offset = 0
				scoreIsDirty = false
			}
			sort.Slice(edgeSet[offset:], func(i, j int) bool {
				return edgeSet[offset+i].Heuristic < edgeSet[offset+j].Heuristic
			})
		}
	}
}

// insertBoard handles the bookkeeping for adding to the edge set
func insertBoard(minimalBoard chess.MinimalBoard) bool {
	if !seenBoards.Contains(minimalBoard) {
		seenBoards.Put(minimalBoard)
		edgeSet = append(edgeSet, minimalBoard)
		return true
	}
	duplicates.Add(1)
	return false
}

// an unbuffered drawing thread that draws on a best effort basis.  Useful for debugging and algorithm grokking
func makeBoardDrawer(ctx context.Context, workQueue, newBoardQueue, boardDrawerQueue chan chess.MinimalBoard) func() error {
	return func() error {
		var foundAnswer bool
		for {
			select {
			case <-ctx.Done():
				return fmt.Errorf("context expired on board drawer")
			case newBoard, ok := <-boardDrawerQueue:
				if newBoard.IsSolved {
					foundAnswer = true
				}
				if !ok {
					log.Printf("drawer thread completed")
					return nil
				}
				if !foundAnswer || newBoard.IsSolved {
					rebuiltBoard, err := newBoard.RebuildBoard()
					if err != nil {
						log.Printf("failed to rebuild board while drawing: %v", err)
					}
					log.Printf("\n%s\nseen: %d\tduplicates: %d\tcurrent: %d\tqueued: %d\tprospects: %d\tprocessed: %d",
						rebuiltBoard.String(heuristic),
						len(seenBoards), duplicates.Load(), len(edgeSet), len(workQueue), len(newBoardQueue), processed.Load())
				}
			}
		}
	}
}
