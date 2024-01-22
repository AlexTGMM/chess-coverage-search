# Chess Coverage
This repo attempts to solve the chess riddle of how many pieces are required to threaten every square on a chess board, given scoring for each piece.  The piece scoring used in this algorithm is:
- Pawn - 1
- Knight - 3
- Bishop - 3
- Rook - 5
- Queen - 9

The problem is borrowed from [this puzzling.stackexchange post](https://puzzling.stackexchange.com/questions/2907/how-many-chess-pieces-are-needed-to-control-every-square-on-the-board-no-piece). 

## Algorithm
This implementation uses a search based strategy.  It would be A*, but the heuristic is not admissible.  Each expansion of the edge set is handled by individual workers and the edge set itself is maintained by an orchestrator.

## What's actually here
First let's lay out the goals and non-goals
### Goals
- ✅ Keep Goroutine design and implementation fresh
- ✅ Get some practice profiling and optimizing threaded Go code
- ✅ Keep the algorithm knowledge and implementation fresh
- ✅ Have fun
### Non-Goals
- ❌ Find the best algorithm to solve this problem
- ❌ Write a re-usable puzzle solving framework

So, back to what's actually here.  All three threads (worker, orchestrator, drawing) are implemented and work.  There is a heuristic that sorts the edge set in a reasonable way, but it is _not_ admissible, and it _definitely_ has issues getting stuck and large local minima.   There are also command line flags to enable and disable both memory and CPU profiling.  There are some minimal tests to prove out that the pieces, coverage, and score calculations work, although there are no tests for the threads.
### Results by Board Size
5. Quickly find the optimal solution.  This is because the space is small enough that the search is exhaustive.
6. Find a very good solution within a minute or so.  Unknown if this solution is optimal or not.
7. Find a bad solution over the course of several minutes.  Given a few hours, it will continue to improve on the solution, but again, unsure if this solution is optimal or not.
8. Over the course of several hours, it will find an optimal (according to the referenced SO question).  **_But_** it does this given the optimal score as an initial input condition.  This prevents it from ever getting stuck in a local minima.  The hope is that it would be able to find a better solution, but before it got there, the edge set got too large and it ran out of memory.
9. Untested, but highly unlikely it would do well, given 8...
## Next
- With some further thought and knowledge engineering,  there are probably ways to keep a smaller edge set.  At least one of these is marked in the code as a todo, but won't help when the expected minimum score is known ahead of time.
- Add tests to the board proposal/step functions and see if there are any ways to speed them up.  The hypothesis is that there is some more early pruning that could be done at this step.
- The working hypothesis is that there is not a useful admissible heuristic for this problem, but finding one would allow this to be solved much faster.