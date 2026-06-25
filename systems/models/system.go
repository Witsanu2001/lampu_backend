package models

type ClosedDays struct {
	Monday    bool `json:"monday" firestore:"monday"`
	Tuesday   bool `json:"tuesday" firestore:"tuesday"`
	Wednesday bool `json:"wednesday" firestore:"wednesday"`
	Thursday  bool `json:"thursday" firestore:"thursday"`
	Friday    bool `json:"friday" firestore:"friday"`
	Saturday  bool `json:"saturday" firestore:"saturday"`
	Sunday    bool `json:"sunday" firestore:"sunday"`
}

type SpecialHoliday struct {
	ID   string `json:"id" firestore:"id"`
	Date string `json:"date" firestore:"date"`
	Name string `json:"name" firestore:"name"`
}

type SystemSettingsPayload struct {
	Project          string           `json:"project"`
	IsManualOverride bool             `json:"isManualOverride"`
	StoreStatus      string           `json:"storeStatus"`
	OpenTime         string           `json:"openTime"`
	CloseTime        string           `json:"closeTime"`
	ClosedDays       ClosedDays       `json:"closedDays"`
	SpecialHolidays  []SpecialHoliday `json:"specialHolidays"`
	PIN              string           `json:"PIN"`
}
