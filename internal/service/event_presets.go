package service

import "github.com/stmtc233/sportsmeet/internal/domain"

// EventPreset supplies initial input only; events retain their own configuration.
type EventPreset struct {
	ID       string     `json:"id"`
	Category string     `json:"category"`
	Label    string     `json:"label"`
	Input    EventInput `json:"input"`
}

func (s *Service) EventPresets() []EventPreset {
	out := []EventPreset{}
	add := func(category, name, kind string, start domain.StartMode, mode domain.DistanceMode, legs int) {
		out = append(out, EventPreset{ID: name, Category: category, Label: name, Input: EventInput{
			Gender: "男", Discipline: name, Kind: kind, LaneCount: 8, AttemptCount: 3,
			StartMode: string(start), DistanceMode: string(mode), RelayLegCount: legs,
		}})
	}
	for _, name := range []string{"50m", "60m", "100m", "200m", "400m"} {
		add("短跑", name, "track", domain.StartLanes, "", 0)
	}
	add("中长跑", "800m", "track", domain.StartBreak, "", 0)
	for _, name := range []string{"1000m", "1500m", "3000m", "5000m", "10000m"} {
		add("中长跑", name, "track", domain.StartGroup, "", 0)
	}
	for _, name := range []string{"60m栏", "100m栏", "110m栏", "400m栏"} {
		add("跨栏与障碍", name, "track", domain.StartLanes, "", 0)
	}
	add("跨栏与障碍", "3000m障碍", "track", domain.StartGroup, "", 0)
	add("接力", "4×100m", "relay", domain.StartLanes, "", 4)
	add("接力", "4×400m", "relay", domain.StartBreak, "", 4)
	add("接力", "混合4×400m", "relay", domain.StartBreak, "", 4)
	out[len(out)-1].Input.Gender, out[len(out)-1].Input.Discipline = "混合", "4×400m"
	for _, name := range []string{"跳远", "三级跳远", "立定跳远"} {
		add("跳跃", name, "distance", "", domain.DistanceJump, 0)
	}
	for _, name := range []string{"跳高", "撑竿跳高"} {
		add("跳跃", name, "height", "", "", 0)
	}
	for _, name := range []string{"铅球", "垒球", "实心球", "标枪", "铁饼", "链球"} {
		add("投掷", name, "distance", "", domain.DistanceThrow, 0)
	}
	return out
}

// EventRules previews configuration using exactly the same validation as SaveEvent.
func (s *Service) EventRules(in EventInput) (*domain.EventRules, error) {
	// An empty discipline is allowed while the creation form is being filled.
	if in.Discipline == "" {
		in.Discipline = "自定义"
	}
	e, err := in.event()
	if err != nil {
		return nil, err
	}
	r := e.Rules()
	return &r, nil
}

func (s *Service) TeamSizeWarning(event domain.Event, count int) string {
	return event.TeamSizeWarning(count)
}
