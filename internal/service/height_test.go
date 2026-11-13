package service

import (
	"strings"
	"testing"

	"github.com/stmtc233/sportsmeet/internal/domain"
	"github.com/stmtc233/sportsmeet/internal/interchange"
)

func TestHeightProgressAndFinish(t *testing.T) {
	f := newFixture(t, 2, EventInput{Gender: "男", Discipline: "跳高", Kind: "height", Rounds: []RoundInput{{Stage: "semifinal", AdvanceByPlace: 1}}})
	f.addHeat(0, f.athletes...)
	h := f.detail().Rounds[0].Heats[0].ID
	inputs := []ResultInput{{Position: 1, Bars: []BarInput{{"1.50", "O"}}}, {Position: 2, Bars: []BarInput{{"1.50", "X"}}}}
	must(t, f.svc.SaveResults(f.ctx, ui, h, inputs))
	d := f.detail()
	if d.Rounds[0].Complete || !d.Rounds[0].Heats[0].Slots[0].Result.HeightPending || d.Rounds[0].Heats[0].Slots[0].Result.Qualify != "" {
		t.Fatal("progress treated as final")
	}
	if d.Rounds[0].Standings[1].MarkText != "未过杆" {
		t.Fatal("partial failures shown as final NM")
	}
	for i := range inputs {
		inputs[i].HeightFinished = true
	}
	must(t, f.svc.SaveResults(f.ctx, ui, h, inputs))
	d = f.detail()
	if !d.Rounds[0].Complete || d.Rounds[0].Heats[0].Slots[0].Result.HeightPending || d.Rounds[0].Heats[0].Slots[0].Result.Qualify != "Q" {
		t.Fatal("finish not persisted or qualified")
	}
	inputs[0].HeightFinished = false
	inputs[0].Bars = append(inputs[0].Bars, BarInput{"1.55", "O"})
	must(t, f.svc.SaveResults(f.ctx, ui, h, inputs[:1]))
	d = f.detail()
	if d.Rounds[0].Complete || d.Rounds[0].Heats[0].Slots[0].Result.Qualify != "" {
		t.Fatal("edit did not reopen")
	}
	file, err := f.svc.ExportMeet(f.ctx, f.meet)
	must(t, err)
	data, err := interchange.Encode(file)
	must(t, err)
	file, err = interchange.Decode(data)
	must(t, err)
	found := false
	for _, e := range file.Events {
		if e.ID == f.event {
			found = e.Rounds[0].Heats[0].Slots[0].Result.HeightPending
		}
	}
	if !found {
		t.Fatal("progress lost on export")
	}
	p, err := f.svc.PreviewImport(f.ctx, data, "")
	must(t, err)
	if len(p.Errors) > 0 {
		t.Fatalf("import preview: %v", p.Errors)
	}
	done, err := f.svc.ImportMeet(f.ctx, ui, data, "", p.Hash)
	must(t, err)
	imported, err := f.svc.ListEvents(f.ctx, done.MeetID)
	must(t, err)
	again, err := f.svc.GetEvent(f.ctx, imported[0].Event.ID)
	must(t, err)
	if again.Rounds[0].Complete || !again.Rounds[0].Heats[0].Slots[0].Result.HeightPending {
		t.Fatal("import finalized a partial record")
	}
	// A failed batch must preserve both the old record and completion state.
	bad := []ResultInput{{Position: 1, Bars: []BarInput{{"1.50", "OX"}}}, {Position: 2, Bars: []BarInput{{"1.50", "O"}}}}
	if err := f.svc.SaveResults(f.ctx, ui, h, bad); err == nil {
		t.Fatal("invalid batch accepted")
	}
	if f.detail().Rounds[0].Heats[0].Slots[1].Result.Mark != nil {
		t.Fatal("failed batch partially saved")
	}
}

func TestHeightPreview(t *testing.T) {
	inputs := []ResultInput{{Position: 1, Bars: []BarInput{{"1.50", "o"}, {"1.55", "xo"}, {"1.60", ""}}}, {Position: 2, Bars: []BarInput{{"1.50", "XX-"}, {"1.55", "X"}, {"1.60", ""}}}}
	p := PreviewHeight(inputs)
	if !p.Valid || !p.CanFinish || p.Rows[0].MarkText != "1.55" || *p.Rows[0].Rank != 1 || p.Rows[0].Cells[1].CanSuccess || p.Rows[1].Cells[2].CanSuccess {
		t.Fatalf("preview: %+v", p)
	}
	inputs[0].Bars[1].Trials = "1"
	p = PreviewHeight(inputs)
	if p.Valid || p.CanFinish || p.Rows[0].Cells[1].Error == "" {
		t.Fatal("missing cell validation")
	}
	if _, err := buildResultEvent(domain.Event{Kind: domain.KindHeight}, ResultInput{Bars: []BarInput{{"1.50", "XXX"}, {"1.55", "O"}}}); err == nil {
		t.Fatal("eliminated athlete continued")
	}
}

func TestHeightRotation(t *testing.T) {
	bars := func(trials ...string) []BarInput {
		out := []BarInput{}
		for i, tr := range trials {
			out = append(out, BarInput{[]string{"1.40", "1.45", "1.50"}[i], tr})
		}
		return out
	}
	states := func(row HeightRowPreview) string {
		out := []string{}
		for _, c := range row.Cells {
			out = append(out, c.State)
		}
		return strings.Join(out, " ")
	}
	cases := []struct {
		name   string
		rows   [][]string
		status []string
		next   *HeightNext
		cells  []string
	}{
		{"first trials in position order", [][]string{{"", "", ""}, {"", "", ""}}, nil, &HeightNext{1, "1.40", 1}, []string{"open open open", "open open open"}},
		{"second trials after everyone's first", [][]string{{"X", "", ""}, {"", "", ""}, {"O", "", ""}}, nil, &HeightNext{2, "1.40", 1}, nil},
		{"failed athletes jump again in order", [][]string{{"X", "", ""}, {"O", "", ""}, {"X", "", ""}}, nil, &HeightNext{1, "1.40", 2}, nil},
		{"pass and clearance move on to the next height", [][]string{{"-", "", ""}, {"O", "", ""}}, nil, &HeightNext{1, "1.45", 1}, []string{"done open open", "done open open"}},
		{"third failure ends the athlete", [][]string{{"XXX", "", ""}, {"O", "XO", ""}}, nil, &HeightNext{2, "1.50", 1}, []string{"done out out", "done done open"}},
		{"failures carried over a pass", [][]string{{"XX-", "X", ""}, {"O", "O", "O"}}, nil, nil, []string{"done done out", "done done done"}},
		{"empty cell below a later height is skipped", [][]string{{"", "O", ""}}, nil, &HeightNext{1, "1.50", 1}, []string{"skipped done open"}},
		{"status rows sit out", [][]string{{"", "", ""}, {"O", "", ""}}, []string{"DNS", ""}, &HeightNext{2, "1.45", 1}, []string{"off off off", "done open open"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			inputs := []ResultInput{}
			for i, row := range c.rows {
				in := ResultInput{Position: i + 1, Bars: bars(row...)}
				if c.status != nil {
					in.Status = c.status[i]
				}
				inputs = append(inputs, in)
			}
			p := PreviewHeight(inputs)
			if (p.Next == nil) != (c.next == nil) || p.Next != nil && *p.Next != *c.next {
				t.Fatalf("next = %+v, want %+v", p.Next, c.next)
			}
			for i, want := range c.cells {
				if got := states(p.Rows[i]); got != want {
					t.Fatalf("row %d states = %q, want %q", i+1, got, want)
				}
			}
			if p.NextHeight != "1.55" {
				t.Fatalf("next height = %q", p.NextHeight)
			}
		})
	}
	if p := PreviewHeight([]ResultInput{{Position: 1, Bars: []BarInput{{"1.20", ""}}}}); p.NextHeight != "1.25" {
		t.Fatalf("single height suggestion = %q", p.NextHeight)
	}
}

func TestHeightMCPFinishRequiresConfirmation(t *testing.T) {
	f := newFixture(t, 1, EventInput{Gender: "男", Discipline: "跳高", Kind: "height"})
	f.addHeat(0, f.athletes...)
	h := f.detail().Rounds[0].Heats[0].ID
	op := Mutation{Action: "set_results", HeatID: h, Results: []ResultInput{{Position: 1, Bars: []BarInput{{"1.50", "O"}}}}}
	_, err := f.svc.ApplyMutation(f.ctx, op, "", false)
	must(t, err)
	if f.detail().Rounds[0].Complete {
		t.Fatal("MCP progress finalized")
	}
	op.Results[0].HeightFinished = true
	if _, err := f.svc.ApplyMutation(f.ctx, op, "", false); err == nil {
		t.Fatal("MCP finalized without confirmation")
	}
	p, err := f.svc.PreviewMutation(f.ctx, op)
	must(t, err)
	if len(p.Errors) > 0 || f.detail().Rounds[0].Complete {
		t.Fatalf("finish preview: %+v", p)
	}
	_, err = f.svc.ApplyMutation(f.ctx, op, p.PreviewHash, true)
	must(t, err)
	if !f.detail().Rounds[0].Complete {
		t.Fatal("confirmed finish lost")
	}
}
