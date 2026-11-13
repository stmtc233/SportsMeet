package service

import (
	"context"
	"strconv"

	"github.com/stmtc233/sportsmeet/internal/domain"
	"github.com/stmtc233/sportsmeet/internal/report"
)

// recordHeightColumns is how many heights a 成绩记录表 leaves room for: an
// event does not fix its heights, the judges write them in.
const recordHeightColumns = 10

// RenderRecordSheet returns the 成绩记录表 as a printable HTML document: one
// blank sheet per heat for the judges to write marks on, chosen and ordered
// as the 检录单 is. Its columns follow the event's kind and configuration
// (recordColumns); marks already entered are not printed.
func (s *Service) RenderRecordSheet(ctx context.Context, req CheckInRequest) (string, error) {
	prefs, err := s.Preferences(ctx)
	if err != nil {
		return "", err
	}
	d, err := s.loadCheckIn(ctx, req.MeetID)
	if err != nil {
		return "", err
	}
	list, err := d.sheetHeats(req)
	if err != nil {
		return "", err
	}
	book := report.RecordBook{Meet: d.meet.Name, PrintedAt: domain.RecordedText(s.store.Now(), s.zone)}
	breaks := sheetBreaks(list, prefs.PrintRecordBreak)
	for i, t := range list {
		e := t.event
		lines, label := d.sheetLines(e, t.heat)
		sheet := report.RecordSheet{Break: breaks[i], Event: e.Name, Round: t.round.Label(), Heat: domain.HeatLabel(t.heat.Ordinal, t.heat.Name),
			Time: domain.ScheduledText(t.at, ""), PositionLabel: e.Rules().PositionLabel, Team: e.IsTeam(), NameLabel: label, Lines: lines}
		recordColumns(e, &sheet)
		book.Sheets = append(book.Sheets, sheet)
	}
	return report.RecordSheets(book)
}

// recordColumns sets what the judges fill in for an event:
//   - track and relay: the mark; individual races started in lanes also the
//     heat's wind
//   - distance: one column per attempt, jumps with a wind row under each
//     athlete's marks, and the best mark
//   - height: recordHeightColumns columns headed by hand with the heights,
//     and the best mark
//   - count and score: the mark, after one column per attempt when the
//     event takes several
func recordColumns(e domain.Event, sheet *report.RecordSheet) {
	sheet.MarkLabel = e.Rules().MarkLabel
	attempts := 0
	switch {
	case e.Kind == domain.KindTrack:
		sheet.HeatWind = e.StartMode == domain.StartLanes
	case e.Kind == domain.KindDistance:
		sheet.MarkLabel, attempts = "最好成绩", e.AttemptCount
		sheet.AttemptWind = e.DistanceMode == domain.DistanceJump
	case e.Kind == domain.KindHeight:
		sheet.MarkLabel, sheet.ColumnsTitle = "最好成绩", "高度"
		sheet.Columns = make([]string, recordHeightColumns)
	case e.MultipleAttempts():
		attempts = e.AttemptCount
	}
	for i := range attempts {
		sheet.Columns = append(sheet.Columns, strconv.Itoa(i+1))
	}
}
