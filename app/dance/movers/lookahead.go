package movers

import (
	"github.com/wieku/danser-go/app/beatmap/difficulty"
	"github.com/wieku/danser-go/app/beatmap/objects"
	"github.com/wieku/danser-go/app/settings"
	"github.com/wieku/danser-go/framework/math/curves"
	"github.com/wieku/danser-go/framework/math/math32"
	"github.com/wieku/danser-go/framework/math/mutils"
	"github.com/wieku/danser-go/framework/math/vector"
	"math"
)

type LookaheadMover struct {
	*basicMover

	curve     *curves.Bezier
	lastAngle float32
}

func NewLookaheadMover() MultiPointMover {
	return &LookaheadMover{basicMover: &basicMover{}}
}

func (mover *LookaheadMover) Reset(diff *difficulty.Difficulty, id int) {
	mover.basicMover.Reset(diff, id)
	mover.lastAngle = 0
}

func (mover *LookaheadMover) SetObjects(objs []objects.IHitObject) int {
	config := settings.CursorDance.MoverSettings.Lookahead[mover.id%len(settings.CursorDance.MoverSettings.Lookahead)]

	start, end := objs[0], objs[1]

	mover.startTime = start.GetEndTime()
	mover.endTime = end.GetStartTime()

	startPos := start.GetStackedEndPositionMod(mover.diff)
	endPos := end.GetStackedStartPositionMod(mover.diff)

	dst := startPos.Dst(endPos)
	scale := dst * float32(config.Scale)

	// Control point 1: exit angle from start.
	// Use the slider's exit angle if available, otherwise continue from last direction.
	var exitAngle float32
	if s, ok := start.(objects.ILongObject); ok {
		exitAngle = s.GetEndAngleMod(mover.diff)
	} else {
		exitAngle = mover.lastAngle + math.Pi
	}
	p1 := vector.NewVec2fRad(exitAngle, scale).Add(startPos)

	// Control point 2: entry angle into end.
	// Default: come straight from start direction.
	// With lookahead: if a next note exists, blend the entry angle toward it
	// so the cursor arrives at `end` already curving toward `next`.
	entryAngle := endPos.AngleRV(startPos) // straight back = no lookahead
	if len(objs) > 2 {
		nextPos := objs[2].GetStackedStartPositionMod(mover.diff)
		angleToNext := endPos.AngleRV(nextPos)

		// Blend: 0.0 = pure straight, 1.0 = fully aim at next note on arrival.
		strength := float32(config.LookaheadStrength)
		entryAngle = lerpAngle(entryAngle, angleToNext+math.Pi, strength)
	}

	// If end is a slider, use its entry angle instead.
	if s, ok := end.(objects.ILongObject); ok {
		entryAngle = s.GetStartAngleMod(mover.diff)
	}

	p2 := vector.NewVec2fRad(entryAngle, scale).Add(endPos)

	mover.curve = curves.NewBezierNA([]vector.Vector2f{startPos, p1, p2, endPos})

	// Remember exit angle for next segment.
	if dst > 1 {
		mover.lastAngle = p1.AngleRV(endPos)
	}

	return 2
}

func (mover *LookaheadMover) Update(time float64) vector.Vector2f {
	t := mutils.Clamp((time-mover.startTime)/(mover.endTime-mover.startTime), 0, 1)
	return mover.curve.PointAt(float32(t))
}

// lerpAngle interpolates between two angles (radians) by factor t, taking the shortest arc.
func lerpAngle(a, b, t float32) float32 {
	diff := math32.Mod(b-a+3*math32.Pi, 2*math32.Pi) - math32.Pi
	return a + diff*t
}