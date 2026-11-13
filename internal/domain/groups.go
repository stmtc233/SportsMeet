package domain

// CompetitionGroup is a meet's independently ranked category, such as 初一.
// A heat remains a 组; a CompetitionGroup is always called 组别.
type CompetitionGroup struct {
	ID      string `json:"id"`
	MeetID  string `json:"meetId"`
	Name    string `json:"name"`
	Ordinal int    `json:"ordinal"`
}
