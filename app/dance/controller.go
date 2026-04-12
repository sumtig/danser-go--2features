package dance

import (
	"github.com/wieku/danser-go/app/beatmap"
	"github.com/wieku/danser-go/app/beatmap/objects"
	"github.com/wieku/danser-go/app/dance/movers"
	"github.com/wieku/danser-go/app/dance/schedulers"
	"github.com/wieku/danser-go/app/dance/spinners"
	"github.com/wieku/danser-go/app/graphics"
	"github.com/wieku/danser-go/app/settings"
	"github.com/wieku/danser-go/framework/math/vector"
	"math"
	"sort"
	"strings"
)

type Controller interface {
	SetBeatMap(beatMap *beatmap.BeatMap)
	InitCursors()
	Update(time float64, delta float64)
	GetCursors() []*graphics.Cursor
}

type GenericController struct {
	bMap       *beatmap.BeatMap
	cursors    []*graphics.Cursor
	schedulers []schedulers.Scheduler
}

func NewGenericController() Controller {
	return &GenericController{}
}

func (controller *GenericController) SetBeatMap(beatMap *beatmap.BeatMap) {
	controller.bMap = beatMap
}

func (controller *GenericController) InitCursors() {
	controller.cursors = make([]*graphics.Cursor, settings.TAG)
	controller.schedulers = make([]schedulers.Scheduler, settings.TAG)

	counter := make(map[string]int)

	for i := range controller.cursors {
		controller.cursors[i] = graphics.NewCursor()

		mover := "flower"
		if len(settings.CursorDance.Movers) > 0 {
			mover = strings.ToLower(settings.CursorDance.Movers[i%len(settings.CursorDance.Movers)].Mover)
		}

		moverCtor, mName := movers.GetMoverCtorByName(mover)

		controller.schedulers[i] = schedulers.NewGenericScheduler(moverCtor, i, counter[mName])

		counter[mName]++
	}

	type Queue struct {
		hitObjects []objects.IHitObject
	}

	queues := make([]Queue, settings.TAG)

	queue := controller.bMap.GetObjectsCopy()

	for i := 0; i < len(queue); i++ {
		if s, ok := queue[i].(*objects.Slider); ok && s.IsRetarded() {
			queue = schedulers.PreprocessQueue(i, queue, true)
		}
	}

	if !settings.CursorDance.ComboTag && !settings.CursorDance.Battle &&
		settings.CursorDance.TAGSliderDance && settings.TAG > 1 {
		for i := 0; i < len(queue); i++ {
			queue = schedulers.PreprocessQueue(i, queue, true)
		}
	}

	for i := 0; i < len(queue); i++ {
		if s, ok := queue[i].(*objects.Slider); ok {
			found := false

			for j := i - 1; j >= 0; j-- {
				if o := queue[i-1]; o.GetEndTime() >= s.GetStartTime() {
					queue = schedulers.PreprocessQueue(i, queue, true)
					found = true
					break
				}
			}

			if !found && i+1 < len(queue) {
				if o := queue[i+1]; o.GetStartTime() <= s.GetEndTime() {
					queue = schedulers.PreprocessQueue(i, queue, true)
				}
			}
		}
	}

	for i := 0; i < len(queue); i++ {
		if s, ok := queue[i].(*objects.Spinner); ok {
			var subSpinners []objects.IHitObject

			startTime := s.GetStartTime()

			for j := i + 1; j < len(queue); j++ {
				o := queue[j]

				if o.GetStartTime() >= s.GetEndTime() {
					break
				}

				if endTime := o.GetStartTime() - 30; endTime > startTime {
					subSpinners = append(subSpinners, objects.NewDummySpinner(startTime, endTime))
				}

				startTime = o.GetEndTime() + 30
			}

			if subSpinners != nil && len(subSpinners) > 0 {
				if s.GetEndTime() > startTime {
					subSpinners = append(subSpinners, objects.NewDummySpinner(startTime, s.GetEndTime()))
				}

				queue = append(queue[:i], append(subSpinners, queue[i+1:]...)...)
				sort.SliceStable(queue, func(i, j int) bool { return queue[i].GetStartTime() < queue[j].GetStartTime() })
			}
		}
	}

	cursorLastPos := make([]vector.Vector2f, settings.TAG)
	for i := range cursorLastPos {
		cursorLastPos[i] = vector.NewVec2f(256, 192)
	}
	comboAssignment := make(map[int64]int)

	for j, o := range queue {
		_, isSpinner := o.(*objects.Spinner)

		if (isSpinner && settings.CursorDance.DoSpinnersTogether) || settings.CursorDance.Battle {
			for i := range queues {
				queues[i].hitObjects = append(queues[i].hitObjects, o)
			}
		} else if settings.CursorDance.ComboTag {
			comboSet := o.GetComboSet()

			if _, assigned := comboAssignment[comboSet]; !assigned {
				if settings.CursorDance.SmartCursorAssignment {
					notePos := o.GetStartPosition()
					bestCursor := 0
					bestDist := float32(math.MaxFloat32)

					for i := 0; i < settings.TAG; i++ {
						dx := cursorLastPos[i].X - notePos.X
						dy := cursorLastPos[i].Y - notePos.Y
						dist := dx*dx + dy*dy
						if dist < bestDist {
							bestDist = dist
							bestCursor = i
						}
					}

					comboAssignment[comboSet] = bestCursor
				} else {
					comboAssignment[comboSet] = int(comboSet) % settings.TAG
				}
			}

			i := comboAssignment[comboSet]
			queues[i].hitObjects = append(queues[i].hitObjects, o)
			cursorLastPos[i] = o.GetEndPosition()
		} else {
			i := j % settings.TAG
			queues[i].hitObjects = append(queues[i].hitObjects, o)
		}
	}

	for i := range controller.cursors {
		spinMover := "circle"
		if len(settings.CursorDance.Spinners) > 0 {
			spinMover = settings.CursorDance.Spinners[i%len(settings.CursorDance.Spinners)].Mover
		}

		controller.schedulers[i].Init(queues[i].hitObjects, controller.bMap.Diff, controller.cursors[i], spinners.GetMoverCtorByName(spinMover), true)
	}
}

func (controller *GenericController) Update(time float64, delta float64) {
	for i := range controller.cursors {
		controller.schedulers[i].Update(time)
		controller.cursors[i].Update(delta)

		controller.cursors[i].LeftButton = controller.cursors[i].LeftKey || controller.cursors[i].LeftMouse
		controller.cursors[i].RightButton = controller.cursors[i].RightKey || controller.cursors[i].RightMouse
	}
}

func (controller *GenericController) GetCursors() []*graphics.Cursor {
	return controller.cursors
}