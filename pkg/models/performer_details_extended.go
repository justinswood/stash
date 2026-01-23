package models

type PerformerDetailsExtendedStats struct {
	TopMalePartnerIDs     []int
	TopMalePartnerCount   int
	TopFemalePartnerIDs   []int
	TopFemalePartnerCount int
	TotalContentTime      float64
	ScenesTotal           int
	ScenesOrganized       int
	ScenesTimespan        PerformerDetailsExtendedTimespan
}

type PerformerDetailsExtendedResult struct {
	TopMalePartners       []*Performer                      `json:"top_male_partners"`
	TopMalePartnerCount   int                               `json:"top_male_partner_count"`
	TopFemalePartners     []*Performer                      `json:"top_female_partners"`
	TopFemalePartnerCount int                               `json:"top_female_partner_count"`
	TotalContentTime      float64                           `json:"total_content_time"`
	ScenesTotal           int                               `json:"scenes_total"`
	ScenesOrganized       int                               `json:"scenes_organized"`
	ScenesTimespan        *PerformerDetailsExtendedTimespan `json:"scenes_timespan"`
}

type PerformerDetailsExtendedTimespan struct {
	EarliestDate *string `json:"earliest_date"`
	LatestDate   *string `json:"latest_date"`
}
