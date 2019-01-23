package initialrenderer

import (
	"mapserver/app"
	"mapserver/coords"
	"strconv"
	"time"

	"github.com/sirupsen/logrus"
)

func getTileKey(tc *coords.TileCoords) string {
	return strconv.Itoa(tc.X) + "/" + strconv.Itoa(tc.Y) + "/" + strconv.Itoa(tc.Zoom)
}

func Job(ctx *app.App) {

	fields := logrus.Fields{}
	logrus.WithFields(fields).Info("Starting initial rendering")
	tilecount := 0

	totalLegacyCount, err := ctx.Blockdb.CountLegacyBlocks()
	if err != nil {
		panic(err)
	}

	rstate := ctx.Config.RenderState

	lastcoords := coords.NewMapBlockCoords(rstate.LastX, rstate.LastY, rstate.LastZ)

	for true {
		start := time.Now()

		result, err := ctx.BlockAccessor.FindLegacyMapBlocks(lastcoords, ctx.Config.InitialRenderingFetchLimit, ctx.Config.Layers)

		if err != nil {
			panic(err)
		}

		if len(result.List) == 0 && !result.HasMore {
			fields = logrus.Fields{
				"blocks": rstate.LegacyProcessed,
				"tiles":  tilecount,
			}
			logrus.WithFields(fields).Info("Initial rendering complete")
			rstate.InitialRun = false
			ctx.Config.Save()

			break
		}

		lastcoords = *result.LastPos

		tileRenderedMap := make(map[string]bool)

		for i := 12; i >= 1; i-- {
			for _, mb := range result.List {
				//13
				tc := coords.GetTileCoordsFromMapBlock(mb.Pos, ctx.Config.Layers)

				//12-1
				tc = tc.ZoomOut(13 - i)

				key := getTileKey(tc)

				if tileRenderedMap[key] {
					continue
				}

				tileRenderedMap[key] = true

				fields = logrus.Fields{
					"X":       tc.X,
					"Y":       tc.Y,
					"Zoom":    tc.Zoom,
					"LayerId": tc.LayerId,
				}
				logrus.WithFields(fields).Debug("Dispatching tile rendering (z11-1)")

				tilecount++
				ctx.Objectdb.RemoveTile(tc)
				_, err = ctx.Tilerenderer.Render(tc, 2)
				if err != nil {
					panic(err)
				}
			}
		}

		//Save current positions of initial run
		rstate.LastX = lastcoords.X
		rstate.LastY = lastcoords.Y
		rstate.LastZ = lastcoords.Z
		rstate.LegacyProcessed += result.UnfilteredCount
		ctx.Config.Save()

		t := time.Now()
		elapsed := t.Sub(start)

		progress := int(float64(rstate.LegacyProcessed) / float64(totalLegacyCount) * 100)

		fields = logrus.Fields{
			"count":     len(result.List),
			"processed": rstate.LegacyProcessed,
			"progress%": progress,
			"X":         lastcoords.X,
			"Y":         lastcoords.Y,
			"Z":         lastcoords.Z,
			"elapsed":   elapsed,
		}
		logrus.WithFields(fields).Info("Initial rendering")
	}
}
